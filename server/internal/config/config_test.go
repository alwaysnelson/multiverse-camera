package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadDefaults confirms the server starts with zero configuration.
func TestLoadDefaults(t *testing.T) {
	t.Chdir(t.TempDir()) // no .env in scope
	for _, key := range []string{"PORT", "OPENAI_API_KEY", "IMAGE_QUALITY", "MOCK_MODE", "LOG_LEVEL"} {
		t.Setenv(key, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != defaultPort || cfg.ImageModel != defaultImageModel || cfg.MaxUploadBytes != defaultMaxUploadMB<<20 {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if cfg.Configured() {
		t.Error("should not be configured without a key")
	}
}

// TestLoadFromEnvAndDotEnv checks precedence: real environment beats .env.
func TestLoadFromEnvAndDotEnv(t *testing.T) {
	dir := t.TempDir()
	dotenv := "# comment\nOPENAI_API_KEY=\"sk-from-file\"\nPORT=9999\nexport IMAGE_QUALITY='high'\nMOCK_MODE=yes\nREQUEST_TIMEOUT=30s\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(dotenv), 0o600); err != nil {
		t.Fatal(err)
	}
	// Start in a subdirectory to prove parent directories are searched.
	sub := filepath.Join(dir, "server")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	t.Setenv("PORT", "1234") // environment wins over file
	for _, key := range []string{"OPENAI_API_KEY", "IMAGE_QUALITY", "MOCK_MODE", "REQUEST_TIMEOUT"} {
		os.Unsetenv(key)
		t.Cleanup(func() { os.Unsetenv(key) })
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 1234 {
		t.Errorf("environment should win, got port %d", cfg.Port)
	}
	if cfg.OpenAIAPIKey != "sk-from-file" || cfg.ImageQuality != "high" || !cfg.MockMode || cfg.RequestTimeout != 30*time.Second {
		t.Errorf("dotenv not applied: %+v", cfg)
	}
	if !cfg.Configured() {
		t.Error("should be configured")
	}
}

// TestValidation rejects nonsense values loudly instead of limping along.
func TestValidation(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("IMAGE_QUALITY", "ultra")
	t.Setenv("LOG_LEVEL", "loud")
	if _, err := Load(); err == nil {
		t.Fatal("expected validation error")
	}
}
