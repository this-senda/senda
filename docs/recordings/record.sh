#!/usr/bin/env bash
# Record the senda TUI + CLI documentation (stills + walkthrough GIFs) with vhs.
# Drives the REAL binary against docs/recordings/senda-api, backed by the
# built-in mock server so responses are deterministic and offline.
#
#   deps: vhs, ttyd, ffmpeg   (pacman -S vhs ttyd ffmpeg)
#   run:  task shots:tui:vhs | task shots:cli:vhs | docs/recordings/record.sh [tui|cli|all]
#
# PRIVACY: these recordings must never reveal your local machine. Guards below,
# defense-in-depth so no single failure leaks anything:
#   1. Tapes reset PS1 (hidden) before any frame is captured — no user@host/path.
#   2. We also scrub the prompt in the env vhs inherits (belt for the tape).
#   3. The binary is built with -trimpath, so even a panic shows no /home/<you>
#      source paths.
#   4. The mock server's stdout (which prints the real mocks/ path) is sent to a
#      log file, never the terminal vhs records.
#   5. Every request targets the localhost mock (127.0.0.1:8787) — nothing dials
#      a real host, so your IP is never exposed.
# Still: eyeball the generated PNGs/GIF before committing (see README).
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

target="${1:-all}"

command -v vhs  >/dev/null || { echo "vhs not found — pacman -S vhs"; exit 1; }
command -v ttyd >/dev/null || { echo "ttyd not found — pacman -S ttyd"; exit 1; }

# Guard 3: -trimpath strips local source paths from the binary (panic safety).
go build -trimpath -o bin/senda ./cmd/senda
mkdir -p docs/screenshots/tui docs/screenshots/cli

# Guard 2: hand vhs a clean prompt env so the interactive shell can't print a
# real user@host or set an identifying window title, even if the tape's reset
# were to race the first prompt.
export PS1='$ '
unset PROMPT_COMMAND

# Guard 4: start the mock server with its (path-revealing) output redirected to a
# log file, not the recorded terminal. Kill it on exit no matter what.
mock_log="$(mktemp)"
./bin/senda mock -collection docs/recordings/senda-api -addr :8787 >"$mock_log" 2>&1 &
mock_pid=$!
trap 'kill "$mock_pid" 2>/dev/null || true; rm -f "$mock_log"' EXIT
sleep 1

if [[ "$target" == tui || "$target" == all ]]; then vhs docs/recordings/tui.tape; fi
if [[ "$target" == cli || "$target" == all ]]; then vhs docs/recordings/cli.tape; fi

# Strip any metadata the encoder embedded (ffmpeg creation_time + your timezone,
# encoder string) — privacy belt. No-op if exiftool is absent.
if command -v exiftool >/dev/null; then
  exiftool -q -all= -overwrite_original -ext png -ext gif \
    docs/screenshots/tui docs/screenshots/cli 2>/dev/null || true
fi

echo "done — wrote docs/screenshots/{tui,cli}/  (review them before committing)"
