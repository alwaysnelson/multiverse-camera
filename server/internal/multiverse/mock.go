package multiverse

import (
	"context"
	"time"
)

// Mock is a Transformer that returns the original photo with a canned plan.
// It lets the frontend be developed and demoed without an API key, and keeps
// CI free of network calls.
type Mock struct {
	// Delay simulates model latency so loading states can be exercised.
	Delay time.Duration
}

// Transform returns immediately (after Delay) with a fixed universe. The
// output image is simply the input, which is enough to drive the UI.
func (m Mock) Transform(ctx context.Context, in Input) (Result, error) {
	start := time.Now()
	if m.Delay > 0 {
		select {
		case <-time.After(m.Delay):
		case <-ctx.Done():
			return Result{}, ctx.Err()
		}
	}
	elapsed := time.Since(start)
	return Result{
		ID: newID(),
		Plan: Plan{
			Subject:             "the person in the photo",
			Action:              "holding the same pose as in the capture",
			HeldObject:          "nothing",
			Setting:             "wherever the photo was taken",
			UniverseName:        "Mock Universe",
			UniverseDescription: "A stand-in world used when no OpenAI API key is configured. Set OPENAI_API_KEY to travel for real.",
			NewIdentity:         "an exact copy of themselves",
			MirroredObject:      "nothing",
			NewSetting:          "the same place, unchanged",
			VisualStyle:         "photographic",
			Caption:             "In this universe nothing changed, because MOCK_MODE is on.",
			EditPrompt:          "(no prompt in mock mode)",
		},
		Image:    in.Image,
		MIMEType: in.MIMEType,
		Timings: Timings{
			Analyze: elapsed.Milliseconds() / 2,
			Render:  elapsed.Milliseconds() / 2,
			Total:   elapsed.Milliseconds(),
		},
		Mock:     true,
		Duration: elapsed,
	}, nil
}
