package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAnalyze checks the request shape sent to the Responses API and the
// extraction of the structured text output.
func TestAnalyze(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(w, `{"output":[
			{"type":"reasoning","summary":[]},
			{"type":"message","content":[{"type":"output_text","text":"{\"ok\":true}"}]}
		]}`)
	}))
	defer srv.Close()

	c := New("sk-test", srv.URL+"/")
	text, err := c.Analyze(context.Background(), AnalyzeRequest{
		Model: "vision-x", Instructions: "sys", Prompt: "hi",
		Image: []byte{0xFF, 0xD8}, MIMEType: "image/jpeg",
		SchemaName: "s", Schema: json.RawMessage(`{"type":"object"}`), Temperature: 0.9,
	})
	if err != nil {
		t.Fatal(err)
	}
	if text != `{"ok":true}` {
		t.Errorf("text %q", text)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("auth header %q", gotAuth)
	}
	if gotBody["model"] != "vision-x" || gotBody["instructions"] != "sys" || gotBody["temperature"] != 0.9 {
		t.Errorf("body %v", gotBody)
	}
	format := gotBody["text"].(map[string]any)["format"].(map[string]any)
	if format["type"] != "json_schema" || format["strict"] != true || format["name"] != "s" {
		t.Errorf("format %v", format)
	}
	content := gotBody["input"].([]any)[0].(map[string]any)["content"].([]any)
	img := content[1].(map[string]any)
	if !strings.HasPrefix(img["image_url"].(string), "data:image/jpeg;base64,") {
		t.Errorf("image not sent as data URL: %v", img)
	}
}

// TestAnalyzeRefusalAndEmpty covers the two non-error but unusable outputs.
func TestAnalyzeRefusalAndEmpty(t *testing.T) {
	responses := []string{
		`{"output":[{"type":"message","content":[{"type":"refusal","refusal":"no"}]}]}`,
		`{"output":[]}`,
	}
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, responses[i])
		i++
	}))
	defer srv.Close()

	c := New("k", srv.URL)
	_, err := c.Analyze(context.Background(), AnalyzeRequest{Schema: json.RawMessage(`{}`)})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "refusal" {
		t.Errorf("expected refusal error, got %v", err)
	}
	if _, err := c.Analyze(context.Background(), AnalyzeRequest{Schema: json.RawMessage(`{}`)}); err == nil {
		t.Error("expected error for empty output")
	}
}

// TestEdit checks the multipart upload and base64 decoding of the result.
func TestEdit(t *testing.T) {
	rendered := []byte("fake-jpeg-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/edits" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		for key, want := range map[string]string{
			"model": "image-x", "prompt": "do it", "size": "1024x1536",
			"quality": "high", "input_fidelity": "high", "output_format": "jpeg",
		} {
			if got := r.FormValue(key); got != want {
				t.Errorf("field %s = %q, want %q", key, got, want)
			}
		}
		file, header, err := r.FormFile("image")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if string(data) != "source" || header.Filename != "shot.jpg" {
			t.Errorf("image part %q %q", data, header.Filename)
		}
		if ct := header.Header.Get("Content-Type"); ct != "image/png" {
			t.Errorf("image part Content-Type = %q, want image/png (octet-stream is rejected upstream)", ct)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(rendered)}},
		})
	}))
	defer srv.Close()

	c := New("k", srv.URL)
	res, err := c.Edit(context.Background(), EditRequest{
		Model: "image-x", Prompt: "do it", Image: []byte("source"), Filename: "shot.jpg", MIMEType: "image/png",
		Size: "1024x1536", Quality: "high", InputFidelity: "high", OutputFormat: "jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Image) != string(rendered) || res.MIMEType != "image/jpeg" {
		t.Errorf("result %+v", res)
	}
}

// TestAPIErrorParsing verifies the error envelope and its classifiers.
func TestAPIErrorParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"message":"slow down","type":"rate_limit","code":"rate_limit_exceeded"}}`)
	}))
	defer srv.Close()

	c := New("k", srv.URL)
	_, err := c.Edit(context.Background(), EditRequest{Image: []byte("x")})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %v", err)
	}
	if !apiErr.IsRateLimited() || apiErr.Message != "slow down" || apiErr.Code != "rate_limit_exceeded" {
		t.Errorf("unexpected APIError %+v", apiErr)
	}

	plain := parseAPIError(502, []byte("<html>bad gateway</html>"))
	if !strings.Contains(plain.Error(), "bad gateway") {
		t.Errorf("plain-text error should be preserved: %v", plain)
	}

	moderated := &APIError{StatusCode: 400, Code: "moderation_blocked"}
	if !moderated.IsModerated() {
		t.Error("moderation_blocked should classify as moderated")
	}
	if (&APIError{StatusCode: 401}).IsModerated() {
		t.Error("401 must not classify as moderated")
	}
}
