# Architecture

This document explains how a photo becomes a parallel-universe rendering, why the pipeline is shaped the way it is, and how failures are handled. Read `README.md` first for the product framing.

## Components

| Layer | Location | Responsibility |
|---|---|---|
| Web app | `web/` | Camera access, capture, upload, comparison UI. Never talks to OpenAI. |
| HTTP API | `server/internal/httpapi` | Validation, timeouts, error mapping, serving the built web app. |
| Pipeline | `server/internal/multiverse` | Candidate selection, prompts, the two model calls. |
| OpenAI client | `server/internal/openai` | Thin HTTP client for the Responses API and the Images Edit API. |
| Config | `server/internal/config` | Environment variables with `.env` fallback and validation. |

In development Vite proxies `/api` to the Go server, so the browser sees one origin and no CORS is needed. In `make run` mode the Go binary serves `web/dist` itself with an SPA fallback.

## Request lifecycle

1. **Capture.** `useCamera` owns a `getUserMedia` stream. The shutter draws the current video frame onto a canvas (mirrored for the front camera so the capture matches the preview), downscales to a 1536 px longest edge and encodes JPEG at 0.92. Uploads go through `createImageBitmap` with `imageOrientation: 'from-image'` so EXIF rotation is applied before the same downscale.
2. **Upload.** `POST /api/transform` as multipart. The handler caps the body with `MaxBytesReader`, sniffs the bytes with `http.DetectContentType` (declared types are ignored) and accepts only JPEG and PNG.
3. **Analyse.** The pipeline draws three universes from distinct categories (`PickCandidates`) and sends the photo, the system instructions and the candidate list to the Responses API with a strict JSON schema. The model returns a `Plan`: what it saw, which universe it chose, the mirrored object, and an `edit_prompt`.
4. **Render.** The original photo and `BuildEditPrompt(plan)` go to the Images Edit API with `input_fidelity=high`, `quality` from config and a `size` matched to the photo's orientation (`1536x1024`, `1024x1536` or `1024x1024`).
5. **Respond.** The rendered JPEG is returned inline as a data URL alongside the plan and per-stage timings. Nothing is written to disk on either side.

The whole request runs under one `context.WithTimeout` (`REQUEST_TIMEOUT`, default four minutes). If the browser cancels, the request context is cancelled and both upstream calls abort.

## Prompt strategy

The creative quality of the result is decided in the analysis step, not the rendering step. Three mechanisms keep it from collapsing into generic fantasy:

- **Curated universes.** `universes.go` holds 45 hand-written seeds in 10 categories. Each hint names what the subject becomes *and* what objects become, e.g. "a fox spirit with lantern eyes in a rainy Edo alley; objects become paper talismans, masks and floating spirit lights".
- **Diversity by construction.** A roll never contains two universes from the same category, so the model always compares genuinely different directions.
- **Narrative mirror rule.** The system prompt requires the new identity to perform the same action with an object in the same role, forbids adding or removing held objects, and forbids clichés unless a candidate demands them.

The model's `edit_prompt` is creative and scene-specific. The server appends a fixed constraint block (`renderConstraints` in `prompt.go`) covering pose, limbs, gaze, camera angle, framing, subject scale and "no text". Keeping this server-side means it is applied on every request regardless of what the model wrote.

The schema is strict (`additionalProperties: false`, all fields required); `TestPlanSchemaMatchesStruct` guarantees the Go struct and the schema never drift apart.

## Error model

Every failure becomes `{ "error": { "code", "message" } }` with a stable code. The frontend maps codes to titles and always offers "Try again" and "New photo".

| Situation | Status | Code |
|---|---|---|
| No API key and not mock | 503 | `not_configured` |
| No `image` field | 400 | `missing_image` |
| Not JPEG/PNG (sniffed) | 415 | `unsupported_type` |
| Over `MAX_UPLOAD_MB` | 413 | `too_large` |
| OpenAI 401 | 502 | `invalid_api_key` |
| OpenAI 429 | 429 | `rate_limited` |
| OpenAI safety refusal | 422 | `moderated` |
| Pipeline exceeded timeout | 504 | `timeout` |
| Other OpenAI error | 502 | `upstream_error` |
| Anything else | 500 | `internal_error` (details logged, never returned) |

Panics are recovered by middleware and logged with a stack trace.

## Frontend state machine

`App.tsx` holds a single discriminated union:

```
camera ──shutter/upload──▶ processing ──ok──▶ result
   ▲                          │  │              │
   │        cancel ◀──────────┘  └──err──▶ error│
   └──────── new photo ◀────────────────────────┘
                     another universe / try again ──▶ processing (same photo)
```

Each phase carries exactly the data it needs (a result cannot exist without a photo). Object URLs for captured photos are created once and revoked when the photo is discarded.

`useCamera` handles: insecure context (no HTTPS), permission denied, no device, device busy, `facingMode` rejected (falls back to any camera), and multiple-camera detection for the flip button. Every failure state renders an upload fallback so there is no dead end.

## Testing strategy

- **Go unit tests** use interfaces (`Analyzer`, `Editor`, `Transformer`) so the pipeline and handlers run with fakes. The OpenAI client is tested against `httptest.Server` to pin the exact request shapes.
- **Frontend unit tests** cover the API client (fetch mocked) and the pure image helpers. Camera and canvas code is exercised in a real browser instead of jsdom.
- **Mock mode** (`MOCK_MODE=1`) returns the input photo with a canned plan after a two-second delay. It powers UI development, CI-free demos and the Playwright smoke run described in the README.

## Deliberate omissions

- No GraphQL, database, auth or hosting: a two-route local prototype does not need them.
- No official OpenAI SDK: two endpoints, one multipart upload; a visible hand-written client is easier to audit and test.
- No service worker or PWA manifest: the app is meant to be launched from a terminal on the same machine.
