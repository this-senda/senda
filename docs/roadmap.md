---
layout: page
title: Roadmap
permalink: /roadmap/
---

# Senda — Roadmap

> Where Senda is headed. This is intent, not a promise — priorities shift as the
> project and the people using it do. For exactly what changed and when, see the
> [CHANGELOG](https://github.com/this-senda/senda/blob/main/CHANGELOG.md).

## Shipped in v0.1

The v0.1 line is feature-complete for everyday API work:

- **Core** — 3-pane shell, multi-request tabs, all HTTP methods, collections as plain YAML folders.
- **Requests** — params, headers, body (JSON, raw, form, multipart, GraphQL); auth (Bearer, Basic, API key, OAuth2).
- **Environments** — `{{var}}` interpolation with a precedence stack and gitignored `*.secret.yaml` overlays.
- **Testing** — per-request assertions, pre/post-request JS scripting (Goja sandbox), folder runner, load testing.
- **AI assist** — optional LLM-suggested assertions from a response (bring your own key; Anthropic or any OpenAI-compatible endpoint).
- **Mock server** — YAML-defined routes, scenarios, stateful CRUD resources, proxy passthrough, hot-reload.
- **Protocols** — HTTP/HTTPS, WebSocket, and Server-Sent Events.
- **And more** — security scanning, JSON Schema response validation, import (curl / Postman / OpenAPI), code generation, doc generation, 13 themes, command palette, request history, file watch, cookie jar, read-only source-control diff (working tree vs `HEAD`) — plus a headless CLI and a terminal UI.

See the [CHANGELOG](https://github.com/this-senda/senda/blob/main/CHANGELOG.md) for the full v0.1 detail.

## Next

Near-term, roughly in priority order:

- **Secrets editing UI** — manage `*.secret.yaml` from the app, with auto-gitignore on the first secret.
- **Rendered markdown** in the per-request docs tab (today it shows raw source).
- **gRPC** — first-class gRPC requests alongside HTTP and GraphQL.

## Later / exploring

Bigger bets, not yet committed:

- **Split view** — two requests side by side, for comparing environments or endpoints.
- **Visual test orchestration** — branching and data-driven runs beyond the sequential folder runner.
- **OpenAPI spec editor** — edit a linked spec and pull request-body schema hints into the editor.
- **GraphQL introspection** — fetch `__schema` to drive query autocomplete.
- **In-app change history** — the read-only source-control diff (working tree vs `HEAD`) shipped in v0.1; next is surfacing `git log --follow` history for a request.

## Non-goals

Things Senda deliberately won't do, to stay small and local-first:

- Cloud accounts, hosted sync, or telemetry.
- A proprietary collection format — it's plain YAML, and stays that way.
- Electron — the native Wails shell is the whole point.

Have an idea? [Open a feature request](https://github.com/this-senda/senda/issues/new/choose).
