package multiverse

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Plan is the structured output of the vision model. It records what the
// model saw, which universe it chose, and the exact prompt for the renderer.
type Plan struct {
	// What the model observed in the original photo.
	Subject    string `json:"subject"`
	Action     string `json:"action"`
	HeldObject string `json:"held_object"`
	Setting    string `json:"setting"`

	// The parallel universe it designed.
	UniverseName        string `json:"universe_name"`
	UniverseDescription string `json:"universe_description"`
	NewIdentity         string `json:"new_identity"`
	MirroredObject      string `json:"mirrored_object"`
	NewSetting          string `json:"new_setting"`
	VisualStyle         string `json:"visual_style"`

	// Caption is one short, witty line shown above the comparison.
	Caption string `json:"caption"`

	// EditPrompt is the model's scene-specific instruction for the renderer.
	// The server appends hard constraints before sending it.
	EditPrompt string `json:"edit_prompt"`
}

// planSchema is a strict JSON schema: every field is required and no extras
// are allowed, which guarantees the model output unmarshals into Plan.
var planSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": [
    "subject", "action", "held_object", "setting",
    "universe_name", "universe_description", "new_identity",
    "mirrored_object", "new_setting", "visual_style",
    "caption", "edit_prompt"
  ],
  "properties": {
    "subject": {"type": "string", "description": "The single main subject of the photo, e.g. 'an elderly man with a grey beard'. If there is no person or animal, the most prominent object."},
    "action": {"type": "string", "description": "Precise body pose and action, including where hands, head and gaze are."},
    "held_object": {"type": "string", "description": "What the subject holds or interacts with. 'nothing' if none."},
    "setting": {"type": "string", "description": "Where the photo was taken, lighting and time of day."},
    "universe_name": {"type": "string", "description": "A short evocative title for the parallel universe, 2 to 4 words."},
    "universe_description": {"type": "string", "description": "One or two sentences describing this universe for a curious viewer."},
    "new_identity": {"type": "string", "description": "Who or what the subject is in the parallel universe."},
    "mirrored_object": {"type": "string", "description": "What the held object becomes. Must serve the same role in the pose. 'nothing' if there was no object."},
    "new_setting": {"type": "string", "description": "The replacement environment, matching the original camera angle and layout."},
    "visual_style": {"type": "string", "description": "Rendering style: photographic by default, or the art style demanded by the universe."},
    "caption": {"type": "string", "description": "One witty line, at most 90 characters, contrasting both worlds."},
    "edit_prompt": {"type": "string", "description": "A detailed instruction for an image-editing model describing the full transformation while preserving pose, framing and composition."}
  }
}`)

// PlanSchema returns the strict JSON schema the vision model must satisfy.
func PlanSchema() json.RawMessage { return planSchema }

// PlanSchemaName is the identifier sent with the schema.
const PlanSchemaName = "parallel_universe_plan"

// Instructions is the system prompt for the vision model. It encodes the
// creative rules that make results feel like a parallel universe rather than
// a random re-paint.
const Instructions = `You are the art director of Multiverse Camera, an app that shows what a photographed moment looks like in a parallel universe.

The core idea: the same moment, the same pose, a different existence. A man reading on a bench becomes a demon reading a burning contract on a throne of cooling lava. The pose, the framing and the composition stay; identity, objects, environment and meaning change.

Rules you must follow:
1. Identify ONE main subject. Describe their exact pose in detail: body orientation, limbs, hands, head, gaze.
2. Choose ONE of the candidate universes offered to you. Pick the one that creates the most striking, unexpected contrast with the real scene while remaining legible. You may adapt its details to fit the photo.
3. Design a "narrative mirror": the subject's new identity must be doing the SAME action with an object that plays the SAME role. A book becomes a contract, a phone becomes a scrying mirror, a coffee cup becomes a skull goblet. Never leave hands empty if they were holding something, and never add an object if they were not.
4. Replace the entire environment so it belongs to the new universe, but keep the same layout: same camera angle, same horizon, same placement and scale of the subject, same major shapes (a bench stays a seat, a wall stays a wall).
5. Be bold and specific. Avoid clichés such as generic cyberpunk, steampunk or "epic fantasy" unless the candidate demands it. Concrete details beat adjectives.
6. Write edit_prompt as a direct instruction to an image-editing model, 80 to 160 words, describing the new identity, mirrored object, environment, lighting and style. Do not mention the original photo or use phrases like "in this image".
7. Never include real people's names, brands, logos or text to be rendered in the image.
8. Respect dignity: transformations are playful and imaginative, never mocking the subject's body, age, ethnicity or disability.`

// UserPrompt builds the per-request message listing candidate universes.
func UserPrompt(candidates []Universe) string {
	var b strings.Builder
	b.WriteString("Analyse the attached photo and plan its parallel universe.\n\nCandidate universes (choose one):\n")
	for i, u := range candidates {
		fmt.Fprintf(&b, "%d. %s (%s): %s\n", i+1, u.Name, u.Category, u.Hint)
	}
	b.WriteString("\nReturn only the JSON object described by the schema.")
	return b.String()
}

// renderConstraints is appended to every edit prompt. These are the lines that
// keep the result recognisably "the same moment".
const renderConstraints = `

Hard constraints:
- Preserve the exact body pose, limb positions, head angle and gaze direction of the main subject.
- Preserve the camera angle, framing, crop, horizon line and the subject's position and scale in the frame.
- Keep the overall spatial layout of the scene; large shapes may change material and meaning but not position.
- Transform identity, clothing, held objects, environment, lighting and colour palette completely to match the new universe.
- No text, letters, logos, watermarks or borders anywhere in the image.`

// BuildEditPrompt combines the model's scene-specific prompt with the fixed
// constraints. Keeping the constraints server-side means a creative but
// forgetful model cannot drop them.
func BuildEditPrompt(p Plan) string {
	base := strings.TrimSpace(p.EditPrompt)
	if base == "" {
		// Fallback in case the model returned an empty prompt: assemble one
		// from the structured fields so the request still makes sense.
		base = fmt.Sprintf(
			"Transform the main subject into %s, holding %s, in %s. Style: %s.",
			p.NewIdentity, p.MirroredObject, p.NewSetting, p.VisualStyle,
		)
	}
	return base + renderConstraints
}

// ParsePlan decodes and sanity-checks the model's JSON.
func ParsePlan(raw string) (Plan, error) {
	var p Plan
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return Plan{}, fmt.Errorf("plan is not valid JSON: %w", err)
	}
	if strings.TrimSpace(p.UniverseName) == "" || strings.TrimSpace(p.NewIdentity) == "" {
		return Plan{}, fmt.Errorf("plan is missing a universe or identity")
	}
	// Captions render in a fixed-height header; clamp runaway output.
	p.Caption = clamp(strings.TrimSpace(p.Caption), 120)
	return p, nil
}

// clamp truncates s to max runes, appending an ellipsis when it was cut.
func clamp(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}
