#!/usr/bin/env bash
# Runs the Go API and the Vite dev server together and stops both on exit.
#
# Usage:
#   scripts/dev.sh          # http://localhost:5173  (camera works on localhost)
#   scripts/dev.sh --https  # https://<your-lan-ip>:5173 (camera works on phones)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_BIN="${GO:-$(command -v go || echo "$HOME/sdk/go/bin/go")}"

if [[ "${1:-}" == "--https" ]]; then
  export VITE_HTTPS=1
fi

cleanup() {
  # Kill the whole process group so child processes of npm die too.
  trap - EXIT INT TERM
  kill 0 2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo "▶ Go API      → http://localhost:${PORT:-8080}"
(cd "$ROOT/server" && "$GO_BIN" run ./cmd/server) &

echo "▶ Vite dev    → ${VITE_HTTPS:+https}${VITE_HTTPS:-http}://localhost:5173"
(cd "$ROOT/web" && npm run dev -- --clearScreen false) &

wait
