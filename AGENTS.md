# Multiverse Camera — agent instructions

Local-only prototype: a browser camera app (React 19, TypeScript, Vite 8) backed by a Go 1.24+ API that sends each photo through OpenAI (Responses API for analysis, Images Edit API for rendering) and returns the same moment in a parallel universe. Read `README.md` for the product story and `docs/architecture.md` for the pipeline.

## Commands

```sh
make setup        # npm install + create .env
make dev          # Go API :8080 + Vite :5173 (proxy /api)
make dev-https    # same, self-signed HTTPS so phones can use the camera
make check        # lint + tests for both halves (what CI runs)
make run          # build frontend, then serve everything from the Go binary
```

Go lives at `$HOME/sdk/go/bin/go` on this machine if it is not on PATH; the Makefile finds it.

## Boundaries

Always:
- Keep every function commented: godoc style in Go (comment starts with the name), JSDoc in TypeScript.
- Run `make check` before declaring work done.
- Keep all user-facing text and code comments in English.

Ask first:
- Adding a dependency to `web/package.json` or `server/go.mod` (the server is intentionally stdlib-only).
- Changing the OpenAI request shapes in `server/internal/openai`.

Never:
- Commit `.env` or any API key. `.env.example` holds names only.
- Store uploaded photos or rendered images on disk; the app is stateless by design.
- Add hosting, auth or a database. This is a local prototype.

## Conventions that differ from defaults

- Frontend lint is oxlint, not ESLint; formatting is Prettier (no semicolons, single quotes, width 100).
- `web/` installs with `legacy-peer-deps` (see `web/.npmrc`) because vitest 5 and Vite 8 peer ranges lag; do not remove it without checking `npm ci` still passes.
- API errors are always `{ "error": { "code", "message" } }` with a stable `code`; the frontend switches on `code`, never on message text.
- Universes live in `server/internal/multiverse/universes.go`. Keep categories balanced; `PickCandidates` draws one per category.

## Gotchas

- Browsers only expose the camera on HTTPS or `localhost`. A phone opening `http://<lan-ip>:5173` gets no camera; use `make dev-https`.
- `images/edits` needs `input_fidelity=high` to keep faces and pose; dropping it makes results drift.
- Reasoning-model names (gpt-5 family) reject the `temperature` field; the client only sends it when non-zero, so set `Temperature: 0` in `AnalyzeRequest` if switching to one.
- A transform takes 20–90 s. The Go server has no `WriteTimeout` on purpose; the Vite proxy timeout is raised to 5 minutes for the same reason.
- `http.ServeFile` rejects paths containing `..`; the SPA fallback uses `ServeContent` so those still land on `index.html`.
