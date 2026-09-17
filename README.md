<div align="center">

# Multiverse Camera

**Take a photo. See the same moment in a parallel universe.**

Same pose, same framing, different existence: an old man reading on a park bench becomes a horned noble reading a burning contract on a throne of cooling lava.

[![CI](https://github.com/alwaysnelson/multiverse-camera/actions/workflows/ci.yml/badge.svg)](https://github.com/alwaysnelson/multiverse-camera/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](server/go.mod)
[![TypeScript](https://img.shields.io/badge/TypeScript-strict-3178C6?logo=typescript&logoColor=white)](web/tsconfig.app.json)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)](web/package.json)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

</div>

---

## Table of contents

- [Features](#features)
- [How it works](#how-it-works)
- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
- [Configuration](#configuration)
- [API](#api)
- [Development](#development)
- [Troubleshooting](#troubleshooting)
- [Limitations and roadmap](#limitations-and-roadmap)
- [License](#license)

## Features

- **Live camera in the browser.** Rear camera by default on phones, front camera on laptops, one-tap flip. Falls back to file upload whenever the camera is unavailable.
- **One-shot transformation.** The photo is analysed, a universe is chosen, and the scene is re-rendered with the pose, framing and subject placement preserved.
- **45 curated universes in 10 categories.** Mythic, creature, era, art style, material, scale, object, future, absurd, elemental. Every request draws candidates from different categories so results stay varied.
- **Narrative mirror.** The new identity performs the same action with an object that plays the same role: a book becomes a contract, a phone becomes a scrying mirror.
- **Before/after comparison.** Draggable slider (mouse, touch, keyboard) or side by side. Re-roll the same photo with *Another universe*, download the result, or shoot again.
- **Runs locally, stores nothing.** One Go binary, one OpenAI API key. Photos are streamed to OpenAI and returned inline; nothing touches disk.
- **Mock mode.** Try the whole UI without an API key.

## How it works

```
Browser (React)              Go server                              OpenAI
───────────────              ─────────                              ──────
camera ─▶ JPEG ──POST──▶  validate (sniff bytes, size cap)
                            │
                            ├─ 1. analyse ───────────────────▶  Responses API (vision, strict JSON)
                            │    photo + 3 universe candidates
                            │    ◀── plan: subject, pose, chosen universe, edit prompt
                            │
                            ├─ 2. render ────────────────────▶  Images Edit API (gpt-image-1)
                            │    photo + edit prompt + fixed constraints, input_fidelity=high
                            │    ◀── rendered JPEG
                            │
compare slider ◀── JSON ────┘
```

Two decisions carry the concept. The **Images Edit** endpoint uses the original photo as its canvas, which is what keeps the pose; generating from text alone never does. And the **constraints are appended server-side** (preserve pose, limbs, gaze, camera angle, framing, subject scale; no text), so a creative but forgetful model cannot drop them.

Details, prompt strategy and the error model are in [`docs/architecture.md`](docs/architecture.md).

## Requirements

| | Minimum | Notes |
|---|---|---|
| Go | 1.24 | Standard library only, no modules to download |
| Node.js | 20 | npm 10 included |
| OpenAI API key | — | Needs access to `gpt-4.1` and `gpt-image-1` ([create one](https://platform.openai.com/api-keys)) |
| OS | macOS, Linux, Windows (WSL or Git Bash) | The Makefile and `scripts/dev.sh` are POSIX shell |

**Supported browsers**

| Environment | Camera | Notes |
|---|---|---|
| Chrome, Edge, Firefox, Safari (desktop) | Front camera, mirrored preview | Works on `http://localhost` |
| iOS Safari 15+ | Rear camera by default, flip available | Requires HTTPS (see [Usage](#on-a-phone)) |
| Android Chrome | Rear camera by default, flip available | Requires HTTPS |
| Any browser without camera access | Upload fallback | HEIC uploads are not decoded; use JPEG or PNG |

## Installation

```sh
git clone https://github.com/alwaysnelson/multiverse-camera.git
cd multiverse-camera
make setup            # installs web dependencies and creates .env from .env.example
```

Open `.env` and set your key:

```sh
OPENAI_API_KEY=sk-...
```

The key is read by the Go server only and never reaches the browser. `.env` is git-ignored.

## Usage

### On your computer

```sh
make dev
```

Open **http://localhost:5173**, allow the camera, take a photo. A transform takes 20–90 seconds; the screen shows which stage it is in and lets you cancel.

### On a phone

Browsers expose the camera only on HTTPS or `localhost`, so the plain LAN address will not work. Start the dev server with a self-signed certificate instead:

```sh
make dev-https
```

Open `https://<your-computer's-LAN-IP>:5173` on the phone (same Wi-Fi), accept the certificate warning once, and the rear camera opens.

### Single binary

```sh
make run              # builds web/dist, compiles server/bin/server, serves everything on :8080
```

### Without an API key

```sh
MOCK_MODE=1 make dev
```

The UI runs end to end and returns your own photo as the "parallel universe" so you can test the flow for free.

## Configuration

All settings are environment variables. `.env` in the project root is loaded automatically; real environment variables take precedence. See [`.env.example`](.env.example).

| Variable | Default | Description |
|---|---|---|
| `OPENAI_API_KEY` | — | Required unless `MOCK_MODE` is on |
| `MOCK_MODE` | `0` | Return placeholder results without calling OpenAI |
| `OPENAI_VISION_MODEL` | `gpt-4.1` | Analyses the photo and designs the universe |
| `OPENAI_IMAGE_MODEL` | `gpt-image-1` | Renders the result; `gpt-image-1.5` works if enabled on your account |
| `IMAGE_QUALITY` | `medium` | `low`, `medium` or `high`. Cost and latency scale with it |
| `PORT` | `8080` | API port |
| `MAX_UPLOAD_MB` | `12` | Upload cap; the browser already downscales to 1536 px |
| `REQUEST_TIMEOUT` | `4m` | Whole-pipeline timeout |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `WEB_DIST` | `web/dist` | Built frontend to serve from the Go binary |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | Point at a proxy or compatible API |

**Cost.** One shot is one vision call plus one image edit, roughly $0.05–0.20 depending on `IMAGE_QUALITY`.

## API

| Method | Route | Request | Response |
|---|---|---|---|
| `GET` | `/api/health` | — | `{ status, configured, mock, vision_model, image_model, image_quality, max_upload_mb }` |
| `POST` | `/api/transform` | multipart form, field `image` (JPEG or PNG, ≤ `MAX_UPLOAD_MB`) | `{ id, universe, plan, image: { data_url, mime_type }, timings_ms, mock }` |

Errors always use one envelope with a stable code:

```json
{ "error": { "code": "rate_limited", "message": "OpenAI rate limit or quota reached. Wait a moment and try again." } }
```

| Code | Status | Meaning |
|---|---|---|
| `not_configured` | 503 | No API key and mock mode is off |
| `missing_image` | 400 | No `image` field in the form |
| `unsupported_type` | 415 | Bytes are not JPEG or PNG |
| `too_large` | 413 | Over `MAX_UPLOAD_MB` |
| `invalid_api_key` | 502 | OpenAI rejected the key |
| `rate_limited` | 429 | OpenAI rate limit or quota |
| `moderated` | 422 | OpenAI's safety system declined the photo |
| `timeout` | 504 | Pipeline exceeded `REQUEST_TIMEOUT` |
| `upstream_error` | 502 | Any other OpenAI error |
| `internal_error` | 500 | Unexpected failure (details are logged, not returned) |

## Development

```sh
make help             # list all targets
make dev              # Go API :8080 + Vite :5173 with /api proxied
make check            # everything CI runs: gofmt, go vet, go test -race, tsc, oxlint, prettier, vitest
make test             # tests only
make lint             # lint only
make fmt              # format Go and TypeScript
make clean            # remove build artefacts
```

### Project structure

```
server/                       Go 1.24, standard library only
├── cmd/server/               entry point, graceful shutdown
└── internal/
    ├── config/               env + .env loading, validation
    ├── openai/               minimal client: Responses API + Images Edit API
    ├── multiverse/           universe library, prompts, pipeline, mock
    └── httpapi/              routes, error mapping, middleware, SPA serving
web/                          React 19 + TypeScript + Vite 8
└── src/
    ├── hooks/useCamera.ts    getUserMedia lifecycle, facing mode, fallbacks
    ├── components/           CameraStage, ProcessingView, CompareSlider, ResultView, ErrorView
    ├── lib/                  API client and image helpers (unit tested)
    └── styles/global.css     design tokens and layout
docs/architecture.md          pipeline, prompt strategy, state machine, error model
.github/workflows/ci.yml      CI for both halves
```

### Testing

- **Go:** config precedence, candidate diversity, schema/struct parity, prompt constraints, HTTP validation and error mapping, SPA fallback, and the OpenAI client against a local fake server.
- **Web:** API client and pure image helpers with Vitest; strict TypeScript; oxlint with React hooks rules.
- **Browser:** the full flow (capture, processing, result, slider, re-roll, upload fallback, server offline, permission denied) is exercised with Playwright and a fake camera on iPhone and desktop viewports before release.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Chip says **Server offline** | The Go API is not running | Run `make dev`, then press Retry |
| Chip says **Needs API key** | `OPENAI_API_KEY` is empty | Add it to `.env` and restart |
| No camera on the phone | Page opened over plain HTTP | Use `make dev-https` and accept the certificate |
| Camera permission denied | Blocked in browser settings | Allow it for the site, or use the upload button |
| "OpenAI rejected the API key" | Wrong or revoked key | Check the key and the account's model access |
| "This photo was declined" | OpenAI safety filter | Try a different scene |
| Result drifts from the pose | Model variance | Press *Another universe*; about one in four shots drifts |
| `go: command not found` | Go not on PATH | Install from [go.dev/dl](https://go.dev/dl/); the Makefile also checks `~/sdk/go/bin` |

## Limitations and roadmap

- Pose preservation is strong but not pixel-locked. A ControlNet/OpenPose pipeline would lock the skeleton at the cost of a second provider and weaker world-building.
- Single-subject scenes work best; in a crowd the model picks one main character.
- Rendering is a single long request. Streaming stage progress (SSE) would make the wait feel shorter.
- A local gallery of past universes (IndexedDB) is a natural next step.

## License

[MIT](LICENSE)
