#!/bin/sh
# Local-only e2e runner for Arch (rolling libs break playwright's pinned WebKit).
# Runs tests in the official playwright image so ICU/libxml2/flite versions match.
# CI uses bun run test:e2e directly; this file is not referenced by CI.
#
# The mounted node_modules is installed on the host (e.g. darwin-arm64), so the
# native rolldown (vite) binding for the container's linux platform is missing.
# We fetch just that one binding into a tmp prefix and expose it via NODE_PATH,
# then let playwright start vite via node (PW_WEB_CMD overrides the bun command
# in playwright.config.ts — the image has no bun).
set -e
IMG=mcr.microsoft.com/playwright:v1.61.1-noble   # keep in sync with @playwright/test version
REPO=$(git -C "$(dirname "$0")" rev-parse --show-toplevel)
exec podman run --rm --network host -v "$REPO":/work -w /work/frontend "$IMG" \
  sh -c '
    set -e
    case "$(uname -m)" in
      aarch64) RD=linux-arm64-gnu ;;
      x86_64)  RD=linux-x64-gnu ;;
      *) echo "[run-podman] unsupported arch $(uname -m)" >&2; exit 1 ;;
    esac
    VER=$(node -p "require(\"rolldown/package.json\").version")
    echo "[run-podman] fetching @rolldown/binding-$RD@$VER"
    npm install --silent --no-package-lock --prefix /tmp/rd "@rolldown/binding-$RD@$VER"
    export NODE_PATH=/tmp/rd/node_modules
    export PW_WEB_CMD="node node_modules/.bin/vite --mode test --port 5173"
    node node_modules/.bin/playwright test "$@"
  ' _ "$@"
