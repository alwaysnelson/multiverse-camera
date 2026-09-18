// Package multiverse implements the capture-to-parallel-universe pipeline:
// a vision model analyses the photo and designs a universe, then an image
// model re-renders the photo inside that universe with the pose preserved.
package multiverse

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	"log/slog"
	mrand "math/rand/v2"
	"time"

	// Register decoders so image.DecodeConfig can read the formats the
	// browser sends. WebP is intentionally not supported to stay stdlib-only.
	_ "image/jpeg"
	_ "image/png"

	"multiverse-camera/server/internal/openai"
)

// Analyzer runs the vision model. It is an interface so the pipeline can be
// tested without network access.
type Analyzer interface {
	Analyze(ctx context.Context, req openai.AnalyzeRequest) (string, error)
}

// Editor runs the image model.
type Editor interface {
	Edit(ctx context.Context, req openai.EditRequest) (openai.EditResult, error)
}

// Options tune the pipeline. Zero values are replaced by sensible defaults.
type Options struct {
	VisionModel  string
	ImageModel   string
	ImageQuality string
	// Candidates is how many universes the vision model gets to choose from.
	Candidates int
	// Rand is the random source used to draw candidates. Tests inject a
	// seeded source; production uses the default.
	Rand *mrand.Rand
}

// Pipeline orchestrates the two model calls.
type Pipeline struct {
	analyzer Analyzer
	editor   Editor
	opts     Options
	log      *slog.Logger
}

// Input is a captured photo as uploaded by the browser.
type Input struct {
	Image    []byte
	MIMEType string
	Filename string
}

// Result is what the API returns to the browser.
type Result struct {
	ID       string        `json:"id"`
	Plan     Plan          `json:"plan"`
	Image    []byte        `json:"-"`
	MIMEType string        `json:"-"`
	Timings  Timings       `json:"timings_ms"`
	Mock     bool          `json:"mock"`
	Duration time.Duration `json:"-"`
}

// Timings breaks the total latency down per stage, in milliseconds.
type Timings struct {
	Analyze int64 `json:"analyze"`
	Render  int64 `json:"render"`
	Total   int64 `json:"total"`
}

// New wires a Pipeline with the given model clients.
func New(analyzer Analyzer, editor Editor, opts Options, log *slog.Logger) *Pipeline {
	if opts.Candidates <= 0 {
		opts.Candidates = 3
	}
	if opts.Rand == nil {
		opts.Rand = mrand.New(mrand.NewPCG(uint64(time.Now().UnixNano()), 0xC0FFEE))
	}
	if opts.ImageQuality == "" {
		opts.ImageQuality = "medium"
	}
	if log == nil {
		log = slog.Default()
	}
	return &Pipeline{analyzer: analyzer, editor: editor, opts: opts, log: log}
}

// Transform runs the full pipeline for one photo.
func (p *Pipeline) Transform(ctx context.Context, in Input) (Result, error) {
	start := time.Now()
	id := newID()
	log := p.log.With("request_id", id)

	// Stage 1: understand the photo and design the universe.
	candidates := PickCandidates(p.opts.Rand, p.opts.Candidates)
	log.Info("analyzing photo", "candidates", universeNames(candidates))

	analyzeStart := time.Now()
	raw, err := p.analyzer.Analyze(ctx, openai.AnalyzeRequest{
		Model:        p.opts.VisionModel,
		Instructions: Instructions,
		Prompt:       UserPrompt(candidates),
		Image:        in.Image,
		MIMEType:     in.MIMEType,
		SchemaName:   PlanSchemaName,
		Schema:       PlanSchema(),
		Temperature:  1.0,
	})
	if err != nil {
		return Result{}, fmt.Errorf("analyze: %w", err)
	}
	plan, err := ParsePlan(raw)
	if err != nil {
		return Result{}, fmt.Errorf("analyze: %w", err)
	}
	analyzeMS := time.Since(analyzeStart).Milliseconds()
	log.Info("universe chosen", "universe", plan.UniverseName, "identity", plan.NewIdentity, "ms", analyzeMS)

	// Stage 2: render the same moment inside the new universe.
	renderStart := time.Now()
	edited, err := p.editor.Edit(ctx, openai.EditRequest{
		Model:         p.opts.ImageModel,
		Prompt:        BuildEditPrompt(plan),
		Image:         in.Image,
		Filename:      in.Filename,
		MIMEType:      in.MIMEType,
		Size:          sizeFor(in.Image),
		Quality:       p.opts.ImageQuality,
		InputFidelity: "high",
		OutputFormat:  "jpeg",
	})
	if err != nil {
		return Result{}, fmt.Errorf("render: %w", err)
	}
	renderMS := time.Since(renderStart).Milliseconds()
	log.Info("render complete", "ms", renderMS, "bytes", len(edited.Image))

	return Result{
		ID:       id,
		Plan:     plan,
		Image:    edited.Image,
		MIMEType: edited.MIMEType,
		Timings: Timings{
			Analyze: analyzeMS,
			Render:  renderMS,
			Total:   time.Since(start).Milliseconds(),
		},
		Duration: time.Since(start),
	}, nil
}

// sizeFor maps the source aspect ratio onto the sizes the image model
// supports so a portrait phone shot stays portrait and a landscape webcam
// frame stays landscape.
func sizeFor(img []byte) string {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(img))
	if err != nil || cfg.Width == 0 || cfg.Height == 0 {
		return "1024x1024"
	}
	ratio := float64(cfg.Width) / float64(cfg.Height)
	switch {
	case ratio > 1.2:
		return "1536x1024"
	case ratio < 0.83:
		return "1024x1536"
	default:
		return "1024x1024"
	}
}

// newID returns a short random hex identifier for log correlation.
func newID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// universeNames flattens candidates for structured logging.
func universeNames(us []Universe) []string {
	names := make([]string, len(us))
	for i, u := range us {
		names[i] = u.Name
	}
	return names
}
