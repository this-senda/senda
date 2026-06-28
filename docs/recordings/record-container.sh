#!/usr/bin/env bash
# Throwaway-container recording — maximum privacy.
#
# Runs the entire vhs recording inside the official charmbracelet/vhs image
# (ttyd + ffmpeg + chrome all bundled, so nothing is installed on your machine
# and chrome is NOT downloaded from a CDN). The container has a generic
# hostname, a root user, a fresh env, and a /vhs working dir — so NOTHING about
# your real machine (username, hostname, home path, public IP) can end up in the
# recording, even if every other guard failed. Only the repo is mounted in;
# output lands back in docs/screenshots/ through that mount.
#
#   needs: podman OR docker  (rootless podman → output owned by you, not root)
#   run:   docs/recordings/record-container.sh [tui|cli|all]
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
target="${1:-all}"

engine=""
if command -v podman >/dev/null; then engine=podman
elif command -v docker >/dev/null; then engine=docker
else echo "need podman or docker"; exit 1; fi

# Static Linux binary (CGO off) so it runs inside the image regardless of libc.
CGO_ENABLED=0 GOOS=linux go build -trimpath -o bin/senda ./cmd/senda
mkdir -p docs/screenshots/tui docs/screenshots/cli

# Script run INSIDE the container: start the mock, then render the tape(s).
# (Container teardown kills the mock — no explicit cleanup needed.)
incmd='set -e
./bin/senda mock -collection docs/recordings/senda-api -addr :8787 >/tmp/mock.log 2>&1 &
sleep 1
export PS1="$ "; unset PROMPT_COMMAND'
if [[ "$target" == tui || "$target" == all ]]; then incmd+='
vhs docs/recordings/tui.tape'; fi
if [[ "$target" == cli || "$target" == all ]]; then incmd+='
vhs docs/recordings/cli.tape'; fi

# By default let podman use its already-configured storage driver (forcing a
# different one than the existing store errors with "database configuration
# mismatch"). Only override when explicitly asked, e.g. SENDA_STORAGE_DRIVER=vfs
# on a FRESH store (after `podman system reset`).
driver_args=()
if [[ "$engine" == podman && -n "${SENDA_STORAGE_DRIVER:-}" ]]; then
  driver_args=(--storage-driver "$SENDA_STORAGE_DRIVER")
fi

# --network=none: the mock, ttyd and chrome all talk over loopback INSIDE the
# container, so no external networking is needed. This also (a) avoids rootless
# podman's pasta/slirp needing /dev/net/tun (which a stale post-kernel-update
# boot lacks) and (b) makes it physically impossible for the container to reach
# the internet — your IP cannot leak even in principle.
# If chrome fails to start in your container runtime, add: --security-opt seccomp=unconfined
"$engine" "${driver_args[@]}" run --rm \
  --network none \
  --hostname senda-demo \
  -v "$PWD:/vhs" -w /vhs \
  --entrypoint sh \
  ghcr.io/charmbracelet/vhs -c "$incmd"

# Strip any metadata the encoder embedded (ffmpeg creation_time + your timezone,
# encoder string) — privacy belt. No-op if exiftool is absent.
if command -v exiftool >/dev/null; then
  exiftool -q -all= -overwrite_original -ext png -ext gif \
    docs/screenshots/tui docs/screenshots/cli 2>/dev/null || true
fi

echo "done — wrote docs/screenshots/{tui,cli}/  (review them before committing)"
