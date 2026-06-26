#!/bin/sh
# Local-only e2e runner for Arch (rolling libs break playwright's pinned WebKit).
# Runs tests in the official playwright image so ICU/libxml2/flite versions match.
# CI uses bun run test:e2e directly; this file is not referenced by CI.
# Image lacks bun, but dev:mock is just `vite --mode test` -> run via node.
set -e
IMG=mcr.microsoft.com/playwright:v1.61.1-noble   # keep in sync with @playwright/test version
REPO=$(git -C "$(dirname "$0")" rev-parse --show-toplevel)
exec podman run --rm --network host -v "$REPO":/work -w /work/frontend "$IMG" \
  sh -c 'echo "[run-podman] starting vite..."
         node node_modules/.bin/vite --mode test --port 5173 >/tmp/vite.log 2>&1 &
         for i in $(seq 1 60); do curl -sf http://localhost:5173 >/dev/null && { echo "[run-podman] vite up (${i}s), running playwright"; break; }; sleep 1; done
         node node_modules/.bin/playwright test "$@"' _ "$@"
