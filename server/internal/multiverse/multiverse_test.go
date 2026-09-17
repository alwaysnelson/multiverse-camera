package multiverse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"log/slog"
	"math/rand/v2"
	"strings"
	"testing"

	"multiverse-camera/server/internal/openai"
)

// TestPickCandidatesDistinctCategories guards the diversity rule: a roll must
// never offer two universes from the same category.
func TestPickCandidatesDistinctCategories(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 200; i++ {
		picks := PickCandidates(r, 3)
		if len(picks) != 3 {
			t.Fatalf("expected 3 candidates, got %d", len(picks))
		}
		seen := map[Category]bool{}
		for _, u := range picks {
			if seen[u.Category] {
				t.Fatalf("duplicate category %q in %v", u.Category, picks)
			}
			seen[u.Category] = true
		}
	}
}

// TestPickCandidatesBounds covers the degenerate inputs.
func TestPickCandidatesBounds(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	if got := PickCandidates(r, 0); got != nil {
		t.Errorf("expected nil for n=0, got %v", got)
	}
	if got := PickCandidates(r, 100); len(got) != countCategories() {
		t.Errorf("expected %d picks when n exceeds categories, got %d", countCategories(), len(got))
	}
}

// TestLibraryIsWellFormed catches typos in the curated data.
func TestLibraryIsWellFormed(t *testing.T) {
	names := map[string]bool{}
	for _, u := range Library {
		if u.Name == "" || u.Hint == "" || u.Category == "" {
			t.Errorf("incomplete universe: %+v", u)
		}
		if names[u.Name] {
			t.Errorf("duplicate universe name %q", u.Name)
		}
		names[u.Name] = true
	}
	if countCategories() < 5 {
		t.Errorf("expected at least 5 categories for diversity, got %d", countCategories())
	}
}

// TestPlanSchemaMatchesStruct ensures every schema property has a Plan field
// and vice versa, so strict mode never rejects or silently drops data.
func TestPlanSchemaMatchesStruct(t *testing.T) {
	var schema struct {
		Required   []string                   `json:"required"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(PlanSchema(), &schema); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	var fields map[string]any
	raw, _ := json.Marshal(Plan{})
	_ = json.Unmarshal(raw, &fields)

	for _, key := range schema.Required {
		if _, ok := fields[key]; !ok {
			t.Errorf("schema requires %q but Plan has no such field", key)
		}
		if _, ok := schema.Properties[key]; !ok {
			t.Errorf("schema requires %q but does not define it", key)
		}
	}
	for key := range fields {
		if _, ok := schema.Properties[key]; !ok {
			t.Errorf("Plan field %q is missing from the schema", key)
		}
	}
}

// TestParsePlan checks validation of the model output.
func TestParsePlan(t *testing.T) {
	valid := `{"subject":"a man","action":"sitting","held_object":"a book","setting":"park",
		"universe_name":"Infernal Court","universe_description":"d","new_identity":"a demon",
		"mirrored_object":"a contract","new_setting":"lava","visual_style":"photo",
		"caption":"c","edit_prompt":"p"}`
	p, err := ParsePlan(valid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.NewIdentity != "a demon" {
		t.Errorf("unexpected identity %q", p.NewIdentity)
	}

	if _, err := ParsePlan(`{not json`); err == nil {
		t.Error("expected error for malformed JSON")
	}
	if _, err := ParsePlan(`{"universe_name":"","new_identity":""}`); err == nil {
		t.Error("expected error for empty plan")
	}

	long := strings.Replace(valid, `"caption":"c"`, `"caption":"`+strings.Repeat("x", 500)+`"`, 1)
	p, err = ParsePlan(long)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len([]rune(p.Caption)); got != 120 {
		t.Errorf("expected caption clamped to 120 runes, got %d", got)
	}
}

// TestBuildEditPrompt verifies the hard constraints are always appended and
// the fallback kicks in when the model returns no prompt.
func TestBuildEditPrompt(t *testing.T) {
	withPrompt := BuildEditPrompt(Plan{EditPrompt: "Make it a demon."})
	if !strings.HasPrefix(withPrompt, "Make it a demon.") {
		t.Errorf("model prompt should lead: %q", withPrompt)
	}
	for _, must := range []string{"Preserve the exact body pose", "camera angle", "No text"} {
		if !strings.Contains(withPrompt, must) {
			t.Errorf("constraints missing %q", must)
		}
	}

	fallback := BuildEditPrompt(Plan{NewIdentity: "a demon", MirroredObject: "a contract", NewSetting: "lava", VisualStyle: "photo"})
	if !strings.Contains(fallback, "a demon") || !strings.Contains(fallback, "a contract") {
		t.Errorf("fallback prompt should use structured fields: %q", fallback)
	}
}

// TestUserPromptListsCandidates checks the numbering the instructions refer to.
func TestUserPromptListsCandidates(t *testing.T) {
	prompt := UserPrompt(Library[:2])
	if !strings.Contains(prompt, "1. "+Library[0].Name) || !strings.Contains(prompt, "2. "+Library[1].Name) {
		t.Errorf("candidates not listed: %q", prompt)
	}
}

// TestSizeFor maps aspect ratios to supported output sizes.
func TestSizeFor(t *testing.T) {
	cases := []struct {
		w, h int
		want string
	}{
		{1920, 1080, "1536x1024"},
		{1080, 1920, "1024x1536"},
		{1000, 1000, "1024x1024"},
		{1100, 1000, "1024x1024"},
	}
	for _, c := range cases {
		if got := sizeFor(testJPEG(t, c.w, c.h)); got != c.want {
			t.Errorf("sizeFor(%dx%d) = %s, want %s", c.w, c.h, got, c.want)
		}
	}
	if got := sizeFor([]byte("not an image")); got != "1024x1024" {
		t.Errorf("undecodable input should default to square, got %s", got)
	}
}

// fakeModels records what the pipeline sends and returns canned answers.
type fakeModels struct {
	analyzeReq openai.AnalyzeRequest
	editReq    openai.EditRequest
	analyzeErr error
	editErr    error
}

func (f *fakeModels) Analyze(_ context.Context, req openai.AnalyzeRequest) (string, error) {
	f.analyzeReq = req
	if f.analyzeErr != nil {
		return "", f.analyzeErr
	}
	return `{"subject":"a man","action":"sitting","held_object":"a book","setting":"park",
		"universe_name":"Infernal Court","universe_description":"d","new_identity":"a demon",
		"mirrored_object":"a contract","new_setting":"lava","visual_style":"photo",
		"caption":"c","edit_prompt":"Turn him into a demon."}`, nil
}

func (f *fakeModels) Edit(_ context.Context, req openai.EditRequest) (openai.EditResult, error) {
	f.editReq = req
	if f.editErr != nil {
		return openai.EditResult{}, f.editErr
	}
	return openai.EditResult{Image: []byte("rendered"), MIMEType: "image/jpeg"}, nil
}

// TestPipelineTransform exercises the happy path end to end with fakes.
func TestPipelineTransform(t *testing.T) {
	fake := &fakeModels{}
	p := New(fake, fake, Options{
		VisionModel:  "vision-x",
		ImageModel:   "image-x",
		ImageQuality: "low",
		Rand:         rand.New(rand.NewPCG(7, 7)),
	}, slog.New(slog.DiscardHandler))

	src := testJPEG(t, 1080, 1920)
	res, err := p.Transform(context.Background(), Input{Image: src, MIMEType: "image/jpeg", Filename: "shot.jpg"})
	if err != nil {
		t.Fatalf("transform failed: %v", err)
	}

	if fake.analyzeReq.Model != "vision-x" || !bytes.Equal(fake.analyzeReq.Image, src) {
		t.Error("vision model did not receive the source image and model name")
	}
	if !strings.Contains(fake.analyzeReq.Prompt, "Candidate universes") {
		t.Error("vision prompt should list candidates")
	}
	if fake.editReq.Model != "image-x" || fake.editReq.Size != "1024x1536" || fake.editReq.Quality != "low" {
		t.Errorf("unexpected edit request: %+v", fake.editReq)
	}
	if fake.editReq.InputFidelity != "high" {
		t.Error("edits must request high input fidelity to keep the pose")
	}
	if !strings.HasPrefix(fake.editReq.Prompt, "Turn him into a demon.") {
		t.Errorf("edit prompt should start with the model's prompt: %q", fake.editReq.Prompt)
	}
	if res.Plan.UniverseName != "Infernal Court" || string(res.Image) != "rendered" || res.Mock {
		t.Errorf("unexpected result: %+v", res)
	}
	if res.ID == "" || res.Timings.Total < 0 {
		t.Error("result should carry an id and timings")
	}
}

// TestPipelineErrorsAreWrapped keeps stage names in the error chain so logs
// say which model failed, and keeps the original error reachable.
func TestPipelineErrorsAreWrapped(t *testing.T) {
	boom := &openai.APIError{StatusCode: 401, Message: "bad key"}

	fake := &fakeModels{analyzeErr: boom}
	p := New(fake, fake, Options{}, nil)
	_, err := p.Transform(context.Background(), Input{Image: testJPEG(t, 10, 10), MIMEType: "image/jpeg"})
	var apiErr *openai.APIError
	if !errors.As(err, &apiErr) || !strings.HasPrefix(err.Error(), "analyze:") {
		t.Errorf("analyze error not wrapped correctly: %v", err)
	}

	fake = &fakeModels{editErr: boom}
	p = New(fake, fake, Options{}, nil)
	_, err = p.Transform(context.Background(), Input{Image: testJPEG(t, 10, 10), MIMEType: "image/jpeg"})
	if !errors.As(err, &apiErr) || !strings.HasPrefix(err.Error(), "render:") {
		t.Errorf("render error not wrapped correctly: %v", err)
	}
}

// TestMockTransform confirms the mock echoes the input and honours cancellation.
func TestMockTransform(t *testing.T) {
	in := Input{Image: []byte("photo"), MIMEType: "image/png"}
	res, err := Mock{}.Transform(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Mock || string(res.Image) != "photo" || res.MIMEType != "image/png" {
		t.Errorf("mock should echo input: %+v", res)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (Mock{Delay: 1e9}).Transform(ctx, in); !errors.Is(err, context.Canceled) {
		t.Errorf("expected cancellation error, got %v", err)
	}
}

// testJPEG encodes a solid-colour JPEG of the given dimensions.
func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 120, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 50}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// countCategories returns the number of distinct categories in the library.
func countCategories() int {
	seen := map[Category]bool{}
	for _, u := range Library {
		seen[u.Category] = true
	}
	return len(seen)
}
