// Command server runs the Multiverse Camera API and, when a frontend build is
// present, serves the web app from the same port.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"multiverse-camera/server/internal/config"
	"multiverse-camera/server/internal/httpapi"
	"multiverse-camera/server/internal/multiverse"
	"multiverse-camera/server/internal/openai"
)

// main delegates to run so exit codes and deferred cleanup stay correct.
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run wires configuration, dependencies and the HTTP server, then blocks
// until a shutdown signal arrives. Returning an error instead of calling
// os.Exit keeps deferred cleanup working.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	log := newLogger(cfg.LogLevel)
	slog.SetDefault(log)

	transformer, err := buildTransformer(cfg, log)
	if err != nil {
		return err
	}

	handler := httpapi.New(httpapi.Options{
		Transformer:    transformer,
		Configured:     cfg.Configured(),
		MockMode:       cfg.MockMode,
		VisionModel:    cfg.VisionModel,
		ImageModel:     cfg.ImageModel,
		ImageQuality:   cfg.ImageQuality,
		MaxUploadBytes: cfg.MaxUploadBytes,
		RequestTimeout: cfg.RequestTimeout,
		WebDist:        cfg.WebDist,
		Logger:         log,
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		// WriteTimeout stays unset: a transform legitimately takes minutes and
		// is bounded by RequestTimeout inside the handler instead.
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", "http://localhost"+srv.Addr, "mock", cfg.MockMode, "configured", cfg.Configured())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// buildTransformer picks the mock or the real OpenAI-backed pipeline.
func buildTransformer(cfg config.Config, log *slog.Logger) (httpapi.Transformer, error) {
	if cfg.MockMode {
		log.Warn("MOCK_MODE is on: results are placeholders and no OpenAI calls are made")
		return multiverse.Mock{Delay: 2 * time.Second}, nil
	}
	if cfg.OpenAIAPIKey == "" {
		log.Warn("OPENAI_API_KEY is not set: /api/transform will return 503 until it is")
	}
	client := openai.New(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL)
	return multiverse.New(client, client, multiverse.Options{
		VisionModel:  cfg.VisionModel,
		ImageModel:   cfg.ImageModel,
		ImageQuality: cfg.ImageQuality,
	}, log), nil
}

// newLogger builds a human-readable structured logger at the requested level.
func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
