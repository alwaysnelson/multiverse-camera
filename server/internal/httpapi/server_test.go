package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"multiverse-camera/server/internal/multiverse"
	"multiverse-camera/server/internal/openai"
)

// stubTransformer returns a fixed result or error.
type stubTransformer struct {
	err error
	got multiverse.Input
}

func (s *stubTransformer) Transform(_ context.Context, in multiverse.Input) (multiverse.Result, error) {
	s.got = in
	if s.err != nil {
		return multiverse.Result{}, s.err
	}
	return multiverse.Result{
		ID:       "abc123",
		Plan:     multiverse.Plan{UniverseName: "Infernal Court", NewIdentity: "a demon", Caption: "cap"},
		Image:    []byte("rendered"),
		MIMEType: "image/jpeg",
	}, nil
}

// newTestServer builds a server with test-friendly defaults.
func newTestServer(t *testing.T, tr Transformer, configured bool) *Server {
	t.Helper()
	return New(Options{
		Transformer:    tr,
		Configured:     configured,
		VisionModel:    "v",
		ImageModel:     "i",
		ImageQuality:   "medium",
		MaxUploadBytes: 1 << 20,
		RequestTimeout: 5 * time.Second,
		Logger:         slog.New(slog.DiscardHandler),
	})
}

// multipartBody builds a request body with a single `image` part.
func multipartBody(t *testing.T, field, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, w.FormDataContentType()
}

// testPNG encodes a tiny PNG so the MIME sniffer sees a real image.
func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.White)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// decodeError reads the JSON error envelope.
func decodeError(t *testing.T, body io.Reader) (code, message string) {
	t.Helper()
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(body).Decode(&env); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return env.Error.Code, env.Error.Message
}

func TestHealth(t *testing.T) {
	srv := newTestServer(t, &stubTransformer{}, true)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" || !body.Configured || body.MaxUploadMB != 1 {
		t.Errorf("unexpected health body: %+v", body)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("security headers missing")
	}
}

func TestTransformSuccess(t *testing.T) {
	stub := &stubTransformer{}
	srv := newTestServer(t, stub, true)

	body, ctype := multipartBody(t, "image", "../weird name.PNG", testPNG(t))
	req := httptest.NewRequest(http.MethodPost, "/api/transform", body)
	req.Header.Set("Content-Type", ctype)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var res transformResponse
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if res.ID != "abc123" || res.Universe.Name != "Infernal Court" || res.Universe.Identity != "a demon" {
		t.Errorf("unexpected response: %+v", res)
	}
	if !strings.HasPrefix(res.Image.DataURL, "data:image/jpeg;base64,") {
		t.Errorf("image should be a data URL, got %q", res.Image.DataURL[:40])
	}
	if stub.got.MIMEType != "image/png" || stub.got.Filename != "weird name.png" {
		t.Errorf("transformer received %+v", stub.got)
	}
}

func TestTransformNotConfigured(t *testing.T) {
	srv := newTestServer(t, &stubTransformer{}, false)
	body, ctype := multipartBody(t, "image", "a.png", testPNG(t))
	req := httptest.NewRequest(http.MethodPost, "/api/transform", body)
	req.Header.Set("Content-Type", ctype)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
	if code, _ := decodeError(t, rec.Body); code != "not_configured" {
		t.Errorf("code %q", code)
	}
}

func TestTransformRejectsBadUploads(t *testing.T) {
	srv := newTestServer(t, &stubTransformer{}, true)

	cases := []struct {
		name     string
		field    string
		data     []byte
		wantCode int
		wantErr  string
	}{
		{"wrong field", "photo", testPNG(t), http.StatusBadRequest, "missing_image"},
		{"not an image", "image", []byte("hello world, definitely text"), http.StatusUnsupportedMediaType, "unsupported_type"},
		{"too large", "image", append(testPNG(t), make([]byte, 2<<20)...), http.StatusRequestEntityTooLarge, "too_large"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body, ctype := multipartBody(t, c.field, "a.png", c.data)
			req := httptest.NewRequest(http.MethodPost, "/api/transform", body)
			req.Header.Set("Content-Type", ctype)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != c.wantCode {
				t.Fatalf("status %d, want %d: %s", rec.Code, c.wantCode, rec.Body.String())
			}
			if code, _ := decodeError(t, rec.Body); code != c.wantErr {
				t.Errorf("code %q, want %q", code, c.wantErr)
			}
		})
	}
}

func TestTransformErrorMapping(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode int
		wantErr  string
	}{
		{"timeout", context.DeadlineExceeded, http.StatusGatewayTimeout, "timeout"},
		{"bad key", &openai.APIError{StatusCode: 401}, http.StatusBadGateway, "invalid_api_key"},
		{"rate limit", &openai.APIError{StatusCode: 429}, http.StatusTooManyRequests, "rate_limited"},
		{"moderated", &openai.APIError{StatusCode: 400, Code: "moderation_blocked"}, http.StatusUnprocessableEntity, "moderated"},
		{"other upstream", &openai.APIError{StatusCode: 500, Message: "boom"}, http.StatusBadGateway, "upstream_error"},
		{"unknown", errors.New("disk on fire"), http.StatusInternalServerError, "internal_error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := newTestServer(t, &stubTransformer{err: c.err}, true)
			body, ctype := multipartBody(t, "image", "a.png", testPNG(t))
			req := httptest.NewRequest(http.MethodPost, "/api/transform", body)
			req.Header.Set("Content-Type", ctype)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != c.wantCode {
				t.Fatalf("status %d, want %d", rec.Code, c.wantCode)
			}
			code, msg := decodeError(t, rec.Body)
			if code != c.wantErr {
				t.Errorf("code %q, want %q", code, c.wantErr)
			}
			if strings.Contains(msg, "disk on fire") {
				t.Error("internal error details must not leak to the client")
			}
		})
	}
}

func TestMethodNotAllowedAndUnknownRoute(t *testing.T) {
	srv := newTestServer(t, &stubTransformer{}, true)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/transform", nil))
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != http.MethodPost {
		t.Errorf("GET /api/transform status %d allow %q", rec.Code, rec.Header().Get("Allow"))
	}

	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown route status %d", rec.Code)
	}
	if code, _ := decodeError(t, rec.Body); code != "not_found" {
		t.Errorf("unknown route should return JSON error, got %q", code)
	}
}

func TestRecoverPanics(t *testing.T) {
	srv := newTestServer(t, &stubTransformer{}, true)
	srv.handler = chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("kaboom")
	}), srv.recoverPanics)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status %d", rec.Code)
	}
}

func TestSPAHandler(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>app</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := New(Options{
		Transformer: &stubTransformer{}, WebDist: dir,
		MaxUploadBytes: 1, RequestTimeout: time.Second, Logger: slog.New(slog.DiscardHandler),
	})

	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}

	if rec := get("/assets/app.js"); rec.Code != 200 || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("asset not served with long cache: %d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	if rec := get("/some/client/route"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "<h1>app</h1>") {
		t.Errorf("SPA fallback failed: %d %q", rec.Code, rec.Body.String())
	}
	// The mux normalises dotted paths with a redirect; hit the handler
	// directly to prove traversal never escapes the dist directory.
	rec := httptest.NewRecorder()
	spaHandler(dir).ServeHTTP(rec, &http.Request{Method: http.MethodGet, URL: &url.URL{Path: "/../../../etc/passwd"}})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<h1>app</h1>") {
		t.Errorf("path traversal should fall back to index: %d %q", rec.Code, rec.Body.String())
	}
}

func TestSafeFilename(t *testing.T) {
	cases := map[[2]string]string{
		{"photo.PNG", "image/png"}:       "photo.png",
		{"../x/shot.jpeg", "image/jpeg"}: "shot.jpg",
		{"", "image/jpeg"}:               "capture.jpg",
		{"noext", "image/png"}:           "noext.png",
	}
	for in, want := range cases {
		if got := safeFilename(in[0], in[1]); got != want {
			t.Errorf("safeFilename(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}
