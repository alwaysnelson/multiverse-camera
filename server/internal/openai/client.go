// Package openai is a minimal, dependency-free client for the two OpenAI
// endpoints this project needs: the Responses API (vision analysis with
// structured output) and the Images Edit API (photo-to-photo rendering).
//
// The official SDK is deliberately not used: the surface area we touch is
// tiny, and keeping the HTTP shapes visible makes the pipeline easy to audit.
package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

// Client talks to the OpenAI REST API.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// New returns a Client. baseURL should not have a trailing slash.
func New(apiKey, baseURL string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		// Individual calls are bounded by their context; this is a safety net.
		http: &http.Client{Timeout: 5 * time.Minute},
	}
}

// APIError is returned when OpenAI responds with a non-2xx status.
type APIError struct {
	StatusCode int
	Type       string
	Code       string
	Message    string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("openai: %d %s: %s", e.StatusCode, e.Code, e.Message)
}

// IsUnauthorized reports whether the API key was rejected.
func (e *APIError) IsUnauthorized() bool { return e.StatusCode == http.StatusUnauthorized }

// IsRateLimited reports whether the request hit a rate or quota limit.
func (e *APIError) IsRateLimited() bool { return e.StatusCode == http.StatusTooManyRequests }

// IsModerated reports whether the request was blocked by the safety system.
func (e *APIError) IsModerated() bool {
	return e.StatusCode == http.StatusBadRequest &&
		(strings.Contains(e.Code, "moderation") || strings.Contains(e.Code, "safety") ||
			strings.Contains(strings.ToLower(e.Message), "safety system"))
}

// AnalyzeRequest describes a vision call that must return JSON matching Schema.
type AnalyzeRequest struct {
	Model        string
	Instructions string
	Prompt       string
	Image        []byte // encoded image bytes (JPEG or PNG)
	MIMEType     string
	SchemaName   string
	Schema       json.RawMessage
	Temperature  float64
}

// Analyze sends the image and prompt to the Responses API and returns the raw
// JSON text produced by the model. The caller unmarshals it into its own type.
func (c *Client) Analyze(ctx context.Context, req AnalyzeRequest) (string, error) {
	dataURL := "data:" + req.MIMEType + ";base64," + base64.StdEncoding.EncodeToString(req.Image)

	body := map[string]any{
		"model":        req.Model,
		"instructions": req.Instructions,
		"input": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": req.Prompt},
					map[string]any{"type": "input_image", "image_url": dataURL, "detail": "auto"},
				},
			},
		},
		"text": map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"name":   req.SchemaName,
				"schema": req.Schema,
				"strict": true,
			},
		},
	}
	// Reasoning models reject the temperature parameter, so only send it when
	// the caller asked for a non-default value.
	if req.Temperature > 0 {
		body["temperature"] = req.Temperature
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("encode analyze request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/responses", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	var parsed responsesPayload
	if err := c.do(httpReq, &parsed); err != nil {
		return "", err
	}
	return parsed.text()
}

// responsesPayload is the subset of the Responses API output we care about.
type responsesPayload struct {
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			Refusal string `json:"refusal"`
		} `json:"content"`
	} `json:"output"`
}

// text extracts the first output_text item, or a descriptive error if the
// model refused or returned nothing usable.
func (p responsesPayload) text() (string, error) {
	for _, item := range p.Output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			switch part.Type {
			case "output_text":
				if strings.TrimSpace(part.Text) != "" {
					return part.Text, nil
				}
			case "refusal":
				return "", &APIError{StatusCode: http.StatusBadRequest, Code: "refusal", Message: part.Refusal}
			}
		}
	}
	return "", errors.New("openai: response contained no text output")
}

// EditRequest describes an image-to-image edit.
type EditRequest struct {
	Model         string
	Prompt        string
	Image         []byte
	Filename      string
	MIMEType      string // image/jpeg or image/png; OpenAI rejects untyped parts
	Size          string // e.g. 1024x1024, 1536x1024, 1024x1536
	Quality       string // low, medium, high
	InputFidelity string // low or high; high keeps faces and details closer to the source
	OutputFormat  string // png, jpeg or webp
}

// EditResult is the decoded image returned by the Images Edit API.
type EditResult struct {
	Image    []byte
	MIMEType string
}

// Edit uploads the source image with the prompt and returns the rendered image.
func (c *Client) Edit(ctx context.Context, req EditRequest) (EditResult, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	fields := map[string]string{
		"model":          req.Model,
		"prompt":         req.Prompt,
		"size":           req.Size,
		"quality":        req.Quality,
		"input_fidelity": req.InputFidelity,
		"output_format":  req.OutputFormat,
	}
	for key, value := range fields {
		if value == "" {
			continue
		}
		if err := w.WriteField(key, value); err != nil {
			return EditResult{}, fmt.Errorf("encode field %s: %w", key, err)
		}
	}

	filename := req.Filename
	if filename == "" {
		filename = "source.jpg"
	}
	mimeType := req.MIMEType
	if mimeType == "" {
		mimeType = "image/jpeg"
	}
	// CreateFormFile would label the part application/octet-stream, which the
	// API rejects, so build the part header by hand with the real image type.
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="%s"`, quoteEscape(filename)))
	header.Set("Content-Type", mimeType)
	part, err := w.CreatePart(header)
	if err != nil {
		return EditResult{}, fmt.Errorf("encode image part: %w", err)
	}
	if _, err := part.Write(req.Image); err != nil {
		return EditResult{}, fmt.Errorf("write image part: %w", err)
	}
	if err := w.Close(); err != nil {
		return EditResult{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/images/edits", &buf)
	if err != nil {
		return EditResult{}, err
	}
	httpReq.Header.Set("Content-Type", w.FormDataContentType())

	var parsed struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := c.do(httpReq, &parsed); err != nil {
		return EditResult{}, err
	}
	if len(parsed.Data) == 0 || parsed.Data[0].B64JSON == "" {
		return EditResult{}, errors.New("openai: edit response contained no image data")
	}

	img, err := base64.StdEncoding.DecodeString(parsed.Data[0].B64JSON)
	if err != nil {
		return EditResult{}, fmt.Errorf("decode image data: %w", err)
	}

	mime := "image/png"
	switch req.OutputFormat {
	case "jpeg":
		mime = "image/jpeg"
	case "webp":
		mime = "image/webp"
	}
	return EditResult{Image: img, MIMEType: mime}, nil
}

// quoteEscape makes a filename safe inside a quoted multipart header value.
func quoteEscape(s string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, `\"`).Replace(s)
}

// do executes the request, handles auth and error envelopes, and decodes the
// JSON body into out.
func (c *Client) do(req *http.Request, out any) error {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("openai: request failed: %w", err)
	}
	defer resp.Body.Close()

	// Responses are at most a few megabytes (a base64 image); cap defensively.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return fmt.Errorf("openai: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return parseAPIError(resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("openai: decode response: %w", err)
	}
	return nil
}

// parseAPIError turns OpenAI's error envelope into an APIError, falling back
// to the raw body when the envelope is missing.
func parseAPIError(status int, body []byte) error {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Error.Message != "" {
		return &APIError{
			StatusCode: status,
			Type:       envelope.Error.Type,
			Code:       envelope.Error.Code,
			Message:    envelope.Error.Message,
		}
	}
	msg := strings.TrimSpace(string(body))
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	return &APIError{StatusCode: status, Message: msg}
}
