// Package httpapi exposes the pipeline over HTTP and serves the built
// frontend so a single Go binary runs the whole prototype.
package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"multiverse-camera/server/internal/multiverse"
	"multiverse-camera/server/internal/openai"
)

// Transformer is the single capability the HTTP layer needs from the domain.
// Both the real Pipeline and the Mock satisfy it.
type Transformer interface {
	Transform(ctx context.Context, in multiverse.Input) (multiverse.Result, error)
}

// Options configure the HTTP server.
type Options struct {
	Transformer    Transformer
	Configured     bool
	MockMode       bool
	VisionModel    string
	ImageModel     string
	ImageQuality   string
	MaxUploadBytes int64
	RequestTimeout time.Duration
	WebDist        string
	Logger         *slog.Logger
}

// Server holds the router and its dependencies.
type Server struct {
	opts    Options
	handler http.Handler
	log     *slog.Logger
}

// New builds the router. Method checks live in methodOnly rather than in
// the mux patterns so that the JSON 404 catch-all for /api/ never shadows a
// 405 for a known route.
func New(opts Options) *Server {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	s := &Server{opts: opts, log: opts.Logger}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", methodOnly(http.MethodGet, s.handleHealth))
	mux.HandleFunc("/api/transform", methodOnly(http.MethodPost, s.handleTransform))
	mux.HandleFunc("/api/", s.handleNotFound)

	if dir := opts.WebDist; dir != "" && dirExists(dir) {
		s.log.Info("serving frontend", "dir", dir)
		mux.Handle("/", spaHandler(dir))
	} else {
		mux.HandleFunc("/", s.handleNoFrontend)
	}

	s.handler = chain(mux, s.recoverPanics, s.logRequests, securityHeaders)
	return s
}

// ServeHTTP makes Server usable anywhere an http.Handler is expected.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// healthResponse tells the frontend whether it can expect real results.
type healthResponse struct {
	Status       string `json:"status"`
	Configured   bool   `json:"configured"`
	Mock         bool   `json:"mock"`
	VisionModel  string `json:"vision_model"`
	ImageModel   string `json:"image_model"`
	ImageQuality string `json:"image_quality"`
	MaxUploadMB  int64  `json:"max_upload_mb"`
}

// handleHealth reports readiness and the active configuration. It never
// reveals the API key, only whether one is present.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:       "ok",
		Configured:   s.opts.Configured,
		Mock:         s.opts.MockMode,
		VisionModel:  s.opts.VisionModel,
		ImageModel:   s.opts.ImageModel,
		ImageQuality: s.opts.ImageQuality,
		MaxUploadMB:  s.opts.MaxUploadBytes >> 20,
	})
}

// transformResponse is the JSON body returned for a successful transform.
type transformResponse struct {
	ID       string             `json:"id"`
	Plan     multiverse.Plan    `json:"plan"`
	Image    imagePayload       `json:"image"`
	Timings  multiverse.Timings `json:"timings_ms"`
	Mock     bool               `json:"mock"`
	Universe universeSummary    `json:"universe"`
}

// imagePayload carries the rendered image inline as a data URL so the
// browser can display and download it without a second request.
type imagePayload struct {
	DataURL  string `json:"data_url"`
	MIMEType string `json:"mime_type"`
}

// universeSummary is the human-facing subset of the plan, grouped for the UI.
type universeSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Identity    string `json:"identity"`
	Object      string `json:"object"`
	Setting     string `json:"setting"`
	Style       string `json:"style"`
}

// handleTransform accepts a multipart upload with an `image` part, runs the
// pipeline and returns the rendered parallel-universe image.
func (s *Server) handleTransform(w http.ResponseWriter, r *http.Request) {
	if !s.opts.Configured {
		writeError(w, http.StatusServiceUnavailable, "not_configured",
			"The server has no OpenAI API key. Add OPENAI_API_KEY to your .env file and restart, or set MOCK_MODE=1 to demo without one.")
		return
	}

	// MaxBytesReader stops oversized uploads before they are buffered.
	r.Body = http.MaxBytesReader(w, r.Body, s.opts.MaxUploadBytes+1<<16)

	file, header, err := r.FormFile("image")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "too_large",
				fmt.Sprintf("The photo is larger than %d MB.", s.opts.MaxUploadBytes>>20))
			return
		}
		writeError(w, http.StatusBadRequest, "missing_image", "Send the photo as a multipart field named \"image\".")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, s.opts.MaxUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "unreadable_image", "The uploaded photo could not be read.")
		return
	}
	if int64(len(data)) > s.opts.MaxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large",
			fmt.Sprintf("The photo is larger than %d MB.", s.opts.MaxUploadBytes>>20))
		return
	}

	// Trust the bytes, not the declared header: browsers and users both lie.
	mime := http.DetectContentType(data)
	if mime != "image/jpeg" && mime != "image/png" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_type", "Only JPEG and PNG photos are supported.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.opts.RequestTimeout)
	defer cancel()

	result, err := s.opts.Transformer.Transform(ctx, multiverse.Input{
		Image:    data,
		MIMEType: mime,
		Filename: safeFilename(header.Filename, mime),
	})
	if err != nil {
		s.writeTransformError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, transformResponse{
		ID:   result.ID,
		Plan: result.Plan,
		Image: imagePayload{
			DataURL:  "data:" + result.MIMEType + ";base64," + base64.StdEncoding.EncodeToString(result.Image),
			MIMEType: result.MIMEType,
		},
		Timings: result.Timings,
		Mock:    result.Mock,
		Universe: universeSummary{
			Name:        result.Plan.UniverseName,
			Description: result.Plan.UniverseDescription,
			Identity:    result.Plan.NewIdentity,
			Object:      result.Plan.MirroredObject,
			Setting:     result.Plan.NewSetting,
			Style:       result.Plan.VisualStyle,
		},
	})
}

// writeTransformError maps pipeline failures onto HTTP statuses and messages
// a person can act on. Internal details go to the log, not the client.
func (s *Server) writeTransformError(w http.ResponseWriter, err error) {
	s.log.Error("transform failed", "error", err)

	var apiErr *openai.APIError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusGatewayTimeout, "timeout", "The models took too long. Please try again.")
	case errors.Is(err, context.Canceled):
		writeError(w, 499, "cancelled", "The request was cancelled.")
	case errors.As(err, &apiErr) && apiErr.IsUnauthorized():
		writeError(w, http.StatusBadGateway, "invalid_api_key", "OpenAI rejected the API key. Check OPENAI_API_KEY in your .env file.")
	case errors.As(err, &apiErr) && apiErr.IsRateLimited():
		writeError(w, http.StatusTooManyRequests, "rate_limited", "OpenAI rate limit or quota reached. Wait a moment and try again.")
	case errors.As(err, &apiErr) && apiErr.IsModerated():
		writeError(w, http.StatusUnprocessableEntity, "moderated", "OpenAI's safety system declined this photo. Try a different scene.")
	case errors.As(err, &apiErr):
		writeError(w, http.StatusBadGateway, "upstream_error", "OpenAI returned an error: "+apiErr.Message)
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong while creating your parallel universe.")
	}
}

// methodOnly rejects every method except the allowed one with a JSON 405
// and an Allow header, as HTTP clients expect.
func methodOnly(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use "+method+" for this route.")
			return
		}
		next(w, r)
	}
}

// handleNotFound gives API callers JSON instead of the default HTML 404.
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "No such API route.")
}

// handleNoFrontend explains how to get the UI when web/dist is absent.
func (s *Server) handleNoFrontend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "Multiverse Camera API is running.")
	fmt.Fprintln(w, "The frontend build was not found. Run `make build` to serve it from here, or `make dev` for the Vite dev server.")
}

// errorResponse is the JSON envelope for every error.
type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// writeError writes a JSON error with a stable machine-readable code.
func writeError(w http.ResponseWriter, status int, code, message string) {
	var body errorResponse
	body.Error.Code = code
	body.Error.Message = message
	writeJSON(w, status, body)
}

// writeJSON encodes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write response", "error", err)
	}
}

// safeFilename returns a filename with an extension matching the detected
// MIME type. The upstream API uses the extension to sniff the format.
func safeFilename(name, mime string) string {
	ext := ".jpg"
	if mime == "image/png" {
		ext = ".png"
	}
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if base == "" || base == "." {
		base = "capture"
	}
	return base + ext
}

// dirExists reports whether path is an existing directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// spaHandler serves static files and falls back to index.html for any path
// without a file so client-side routing keeps working after a refresh.
func spaHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			// Vite fingerprints assets, so they can be cached for a year.
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fs.ServeHTTP(w, r)
			return
		}
		serveIndex(w, r, index)
	})
}

// serveIndex streams index.html with ServeContent rather than ServeFile,
// because ServeFile refuses any request path containing ".." and we want
// those to land on the app too (the path was already confined to dir).
func serveIndex(w http.ResponseWriter, r *http.Request, index string) {
	f, err := os.Open(index)
	if err != nil {
		http.Error(w, "frontend build is missing index.html", http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, "frontend build is unreadable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "index.html", info.ModTime(), f)
}
