// Package config loads runtime configuration from environment variables.
//
// The server is configured exclusively through the environment so that it
// behaves the same on a laptop, in CI, and inside a container. A `.env` file
// in the repository root is honoured for local development, but values that
// are already present in the environment always win.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds every tunable the server understands.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port int

	// OpenAIAPIKey authenticates requests to the OpenAI API. When empty the
	// server still starts, but /api/transform reports that it is unconfigured
	// unless MockMode is enabled.
	OpenAIAPIKey string

	// OpenAIBaseURL allows pointing the client at a proxy or compatible API.
	OpenAIBaseURL string

	// VisionModel analyses the captured photo and plans the parallel universe.
	VisionModel string

	// ImageModel renders the parallel-universe image from the original photo.
	ImageModel string

	// ImageQuality is passed straight to the image model: low, medium or high.
	ImageQuality string

	// MockMode short-circuits the OpenAI pipeline and returns a canned result.
	// Useful for UI work and CI where no API key is available.
	MockMode bool

	// MaxUploadBytes caps the size of an uploaded photo.
	MaxUploadBytes int64

	// RequestTimeout bounds the whole capture-to-result pipeline.
	RequestTimeout time.Duration

	// WebDist is the directory holding the built frontend. When it exists the
	// server serves it with an SPA fallback so one process runs everything.
	WebDist string

	// LogLevel is one of debug, info, warn, error.
	LogLevel string
}

// Defaults that are safe to run with zero configuration.
const (
	defaultPort           = 8080
	defaultBaseURL        = "https://api.openai.com/v1"
	defaultVisionModel    = "gpt-4.1"
	defaultImageModel     = "gpt-image-1"
	defaultImageQuality   = "medium"
	defaultMaxUploadMB    = 12
	defaultRequestTimeout = 4 * time.Minute
	defaultWebDist        = "web/dist"
	defaultLogLevel       = "info"
)

// Load reads the environment (after optionally sourcing a .env file) and
// returns a validated Config.
func Load() (Config, error) {
	loadDotEnv()

	cfg := Config{
		Port:           envInt("PORT", defaultPort),
		OpenAIAPIKey:   strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		OpenAIBaseURL:  envString("OPENAI_BASE_URL", defaultBaseURL),
		VisionModel:    envString("OPENAI_VISION_MODEL", defaultVisionModel),
		ImageModel:     envString("OPENAI_IMAGE_MODEL", defaultImageModel),
		ImageQuality:   envString("IMAGE_QUALITY", defaultImageQuality),
		MockMode:       envBool("MOCK_MODE", false),
		MaxUploadBytes: int64(envInt("MAX_UPLOAD_MB", defaultMaxUploadMB)) << 20,
		RequestTimeout: envDuration("REQUEST_TIMEOUT", defaultRequestTimeout),
		WebDist:        envString("WEB_DIST", defaultWebDist),
		LogLevel:       envString("LOG_LEVEL", defaultLogLevel),
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Configured reports whether the real OpenAI pipeline can run.
func (c Config) Configured() bool {
	return c.MockMode || c.OpenAIAPIKey != ""
}

// validate rejects values that would make the server misbehave at runtime.
func (c Config) validate() error {
	var errs []error
	if c.Port <= 0 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("PORT must be between 1 and 65535, got %d", c.Port))
	}
	switch c.ImageQuality {
	case "low", "medium", "high":
	default:
		errs = append(errs, fmt.Errorf("IMAGE_QUALITY must be low, medium or high, got %q", c.ImageQuality))
	}
	if c.MaxUploadBytes <= 0 {
		errs = append(errs, errors.New("MAX_UPLOAD_MB must be positive"))
	}
	if c.RequestTimeout <= 0 {
		errs = append(errs, errors.New("REQUEST_TIMEOUT must be positive"))
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("LOG_LEVEL must be debug, info, warn or error, got %q", c.LogLevel))
	}
	return errors.Join(errs...)
}

// loadDotEnv sources the first .env file found in the working directory or
// any of its parents. It never overrides variables already in the environment.
func loadDotEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for {
		path := filepath.Join(dir, ".env")
		if f, err := os.Open(path); err == nil {
			parseDotEnv(f)
			_ = f.Close()
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

// parseDotEnv understands the common KEY=VALUE format with optional quotes
// and `#` comments. Anything more exotic is intentionally unsupported.
func parseDotEnv(f *os.File) {
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
}

// envString returns the trimmed variable or fallback when unset or blank.
func envString(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// envInt parses an integer variable, returning fallback when unset or invalid.
func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// envBool accepts 1/true/yes/on and 0/false/no/off, case-insensitively.
func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

// envDuration parses a Go duration such as "90s" or "4m".
func envDuration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
