---
layout: default
title: Flows & chaining
---

# Flows & request chaining

Two features let one request build on another without copy-pasting values:

1. **Response references** (`{{res.<slug>...}}`) — pull a value from a request
   already sent in the same run into a later request, with no script.
2. **Flows** (`*.flow.yaml`) — a declarative graph that orchestrates requests
   with branching, loops, parallel fan-out, delays and variable extraction,
   beyond the top-to-bottom folder runner.

Both are **just files on disk** — git-trackable, diffable, and runnable headless
with the same `senda` binary, so a flow that works locally works in CI with no
extra runtime.

## Response references

Inside any request (URL, headers, body, params) you can reference an earlier
response in the current run:

```
{{res.<slug>.<target>}}
```

- `<slug>` — the referenced request's **file name without the `.yaml`
  extension**. `auth/login.yaml` → `login`. Latest send wins within a run.
- `<target>` — the same grammar as [assertions](testing.md):
  - `status` — status code
  - `body` — the raw response body
  - `json.<path>` — a value from the JSON body, e.g. `json.user.id`
  - `header.<Name>` — a response header, e.g. `header.Location`

Example — use the token a login returns as the bearer for the next request:

```yaml
# auth/login.yaml sends and returns { "token": "..." }
# users/me.yaml:
headers:
  - { key: Authorization, value: "Bearer {{res.login.json.token}}", enabled: true }
```

Notes and limits:

- References only resolve to requests sent **earlier in the same run** (a folder
  run or a flow). An unresolved reference is left verbatim and surfaces as an
  `unresolved variables` error — the same as an undefined `{{var}}`.
- To be referenceable, name the file with a **slug-safe name** (letters, digits,
  `.`, `-`, `_`) — spaces can't appear inside `{{...}}`.
- Array indexing (`json.items[0].id`) isn't available in references (the
  `{{...}}` grammar excludes brackets). Use a post-request script for that.
- This is the lightweight alternative to `senda.setVar()` in a post-script — both
  still work, and a flow's `setvar` node (below) is the declarative middle ground.

## Flows

A flow is one YAML file under `.senda/flows/`. Execution starts at `start` and
follows each node's outgoing edge (`next`, or `onTrue`/`onFalse` for a branch)
until a node has none.

```yaml
name: chain
start: getPost
nodes:
  getPost:
    type: request
    request: Chaining/get-post.yaml      # path relative to the collection root
    next: check
  check:
    type: branch
    cond: { left: "{{res.get-post.status}}", op: eq, right: "200" }
    onTrue: setUid
    onFalse: ""                           # empty edge ends the flow
  setUid:
    type: setvar
    var: uid
    from: "{{res.get-post.json.userId}}"
    next: getUser
  getUser:
    type: request
    request: Chaining/get-user.yaml       # its URL uses {{uid}}
```

### Node types

| Type | Fields | Edge | Does |
|------|--------|------|------|
| `request` | `request` (path) | `next` | Sends a request; assertions, scripts, vars and `{{res...}}` all apply. |
| `branch` | `cond: {left, op, right}` | `onTrue` / `onFalse` | Interpolates `left`/`right`, compares with `op` (the [assertion operators](testing.md): `eq`, `neq`, `contains`, `gt`, …). |
| `setvar` | `var`, `from` | `next` | Sets a runtime variable to the interpolated `from` (so later requests read `{{var}}`). |
| `delay` | `ms` | `next` | Sleeps for `ms` milliseconds. |
| `loop` | `data` (CSV/JSON file), `body` (node ids) | `next` | Runs the `body` nodes once per data row, injecting the row as variables. |
| `parallel` | `branches` (lists of node ids) | `next` | Runs each branch concurrently, then continues. |

Nodes listed in a `loop` body or a `parallel` branch are owned by that container
and run linearly (their own edges are ignored) — don't also target them from the
main graph. Loop/parallel bodies support `request`, `setvar` and `delay` nodes.

A global step cap (5000) bounds cycles so a flow can't hang.

### Running a flow

From the desktop app: **collection menu → Flows**, then run one and watch the
steps stream in.

Headless / CI — same binary, same pipeline:

```bash
senda run -collection ./my-api -flow chain            # by name (under .senda/flows)
senda run -collection ./my-api -flow ./path/to.flow.yaml
senda run -collection ./my-api -flow chain --report junit -o flow.xml
```

Exit code is `0` when every request step passes, `1` otherwise. Try the bundled
example:

```bash
senda run -collection examples-collection/public-api -flow chain
```
