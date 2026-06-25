// Request editing pane: method + URL + send, and tabs for params / headers /
// body. Drives the shared request store.
import { createResource, createSignal, For, Index, Match, Show, Switch } from "solid-js";
import { buildClientSchema, getIntrospectionQuery, type GraphQLSchema } from "graphql";
import { Code2, Plus, Save as SaveIcon, X } from "lucide-solid";
import { ICON } from "../lib/icons";
import { api, BodyType, type KV, type Request, type WSSession, type SSEEvent } from "../lib/api";
import { blankKV } from "../lib/factory";
import { saveActive, sendActive } from "../lib/actions";
import {
  activePath,
  activeEnv,
  collection,
  dirty,
  request,
  response,
  sending,
  setDirty,
  setRequest,
} from "../lib/store";
import { Events } from "@wailsio/runtime";
import { docsSrcdoc } from "../lib/docsPreview";
import KVEditor from "./KVEditor";
import CodeEditor from "./CodeEditor";
import AuthEditor from "./AuthEditor";
import AssertEditor from "./AssertEditor";
import CodeGenDialog from "./CodeGenDialog";
import UrlField from "./UrlField";

const METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"];
type Tab = "params" | "headers" | "auth" | "body" | "tests" | "script" | "docs" | "ws" | "sse";

// Auth types that don't actually send credentials — used to dim the tab badge.
const PASSIVE_AUTH = new Set(["", "inherit", "none"]);

export default function RequestEditor() {
  const [tab, setTab] = createSignal<Tab>("params");
  const [showCode, setShowCode] = createSignal(false);
  const [docsView, setDocsView] = createSignal<"edit" | "preview">("edit");
  // Render docs markdown to HTML via the Go docgen renderer (same one used for
  // exported docs) only while the preview tab is showing. Empty docs → skip.
  const [docsHtml] = createResource(
    () => (docsView() === "preview" ? (request.docs ?? "") : null),
    (md) => api.renderMarkdown(md),
  );

  const send = () => void sendActive();
  const save = () => void saveActive();

  return (
    <div class="request-editor">
      <div class="url-bar">
        <div class="url-group">
          <select
            class={`method method-inline method-${request.method.toLowerCase()}`}
            value={request.method}
            onChange={(e) => {
              setRequest("method", e.currentTarget.value);
              setDirty(true);
            }}
          >
            {METHODS.map((m) => (
              <option value={m}>{m}</option>
            ))}
          </select>
          <span class="url-divider" />
          <UrlField />
        </div>
        <button class="send-btn" onClick={send} disabled={sending()}>
          {sending() ? "…" : "Send"}
        </button>
        <button class="url-icon-btn" title="Generate code" onClick={() => setShowCode(true)}>
          <Code2 size={ICON.lg} />
        </button>
        <Show when={dirty()}>
          <button
            class="url-icon-btn dirty"
            title={activePath() ? "Save changes (⌘S)" : "Save request (⌘S)"}
            onClick={save}
          >
            <SaveIcon size={ICON.lg} />
          </button>
        </Show>
      </div>
      <Show when={showCode()}>
        <CodeGenDialog onClose={() => setShowCode(false)} />
      </Show>

      <div class="tabs">
        <button classList={{ active: tab() === "params" }} onClick={() => setTab("params")}>
          Params<Show when={request.params.length}> ({request.params.length})</Show>
        </button>
        <button classList={{ active: tab() === "headers" }} onClick={() => setTab("headers")}>
          Headers<Show when={request.headers.length}> ({request.headers.length})</Show>
        </button>
        <button classList={{ active: tab() === "auth" }} onClick={() => setTab("auth")}>
          Auth<Show when={!PASSIVE_AUTH.has(request.auth?.type ?? "")}> •</Show>
        </button>
        <button classList={{ active: tab() === "body" }} onClick={() => setTab("body")}>
          Body
        </button>
        <button classList={{ active: tab() === "tests" }} onClick={() => setTab("tests")}>
          Tests<Show when={request.asserts?.length}> ({request.asserts!.length})</Show>
        </button>
        <button classList={{ active: tab() === "script" }} onClick={() => setTab("script")}>
          Script<Show when={request.preScript || request.postScript}> •</Show>
        </button>
        <button classList={{ active: tab() === "docs" }} onClick={() => setTab("docs")}>
          Docs<Show when={request.docs}> •</Show>
        </button>
        <Show when={request.body?.type === "websocket"}>
          <button classList={{ active: tab() === "ws" }} onClick={() => setTab("ws")}>
            WebSocket
          </button>
        </Show>
        <Show when={request.body?.type === "sse"}>
          <button classList={{ active: tab() === "sse" }} onClick={() => setTab("sse")}>
            SSE
          </button>
        </Show>
      </div>

      <div class="tab-body">
        <Switch>
          <Match when={tab() === "params"}>
            <BulkKVSection
              rows={request.params}
              onChange={(r) => {
                setRequest("params", r);
                setDirty(true);
              }}
            />
          </Match>
          <Match when={tab() === "headers"}>
            <BulkKVSection
              rows={request.headers}
              keyPlaceholder="Header"
              separator=": "
              onChange={(r) => {
                setRequest("headers", r);
                setDirty(true);
              }}
            />
          </Match>
          <Match when={tab() === "auth"}>
            <AuthEditor
              auth={request.auth ?? ({ type: "inherit" } as any)}
              onChange={(a) => {
                setRequest("auth", a);
                setDirty(true);
              }}
            />
          </Match>
          <Match when={tab() === "body"}>
            <BodyEditor />
          </Match>
          <Match when={tab() === "tests"}>
            <AssertEditor
              rows={request.asserts ?? []}
              onChange={(r) => {
                setRequest("asserts", r);
                setDirty(true);
              }}
            />
          </Match>
          <Match when={tab() === "script"}>
            <ScriptEditor />
          </Match>
          <Match when={tab() === "docs"}>
            <div class="docs-editor">
              <div class="docs-toolbar">
                <button
                  type="button"
                  classList={{ active: docsView() === "edit" }}
                  onClick={() => setDocsView("edit")}
                >
                  Edit
                </button>
                <button
                  type="button"
                  classList={{ active: docsView() === "preview" }}
                  onClick={() => setDocsView("preview")}
                >
                  Preview
                </button>
              </div>
              <Show
                when={docsView() === "edit"}
                fallback={
                  <Show
                    when={(request.docs ?? "").trim()}
                    fallback={<div class="empty-hint docs-hint">Nothing to preview yet.</div>}
                  >
                    <Switch>
                      <Match when={docsHtml.error}>
                        <div class="empty-hint docs-hint">Preview failed: {String(docsHtml.error)}</div>
                      </Match>
                      <Match when={docsHtml.loading}>
                        <div class="empty-hint docs-hint">Rendering…</div>
                      </Match>
                      <Match when={true}>
                        <iframe
                          class="docs-preview"
                          sandbox=""
                          srcdoc={docsSrcdoc(docsHtml() ?? "")}
                          title="Docs preview"
                        />
                      </Match>
                    </Switch>
                  </Show>
                }
              >
                <div class="empty-hint docs-hint">Markdown notes for this request — stored in its YAML.</div>
                <CodeEditor
                  value={request.docs ?? ""}
                  language="text"
                  onChange={(v) => {
                    setRequest("docs", v);
                    setDirty(true);
                  }}
                />
              </Show>
            </div>
          </Match>
          <Match when={tab() === "ws"}>
            <WebSocketPanel />
          </Match>
          <Match when={tab() === "sse"}>
            <SSEPanel />
          </Match>
        </Switch>
      </div>
    </div>
  );
}

export function BulkKVSection(props: {
  rows: KV[];
  onChange: (rows: KV[]) => void;
  keyPlaceholder?: string;
  separator?: string;
}) {
  const sep = () => props.separator ?? "=";
  const [bulk, setBulk] = createSignal(false);
  const [bulkText, setBulkText] = createSignal("");

  const toBulk = (rows: KV[]) =>
    rows.map((r) => `${r.enabled ? "" : "# "}${r.key}${sep()}${r.value}`).join("\n");

  const fromBulk = (text: string): KV[] =>
    text
      .split("\n")
      .map((l) => l.trim())
      .filter(Boolean)
      .map((line) => {
        const disabled = line.startsWith("#");
        if (disabled) line = line.slice(1).trim();
        const eq = line.indexOf("=");
        const colon = line.indexOf(":");
        let key: string, value: string;
        if (eq === -1 && colon === -1) {
          key = line; value = "";
        } else {
          const idx = colon !== -1 && (eq === -1 || colon < eq) ? colon : eq;
          key = line.slice(0, idx).trim();
          value = line.slice(idx + 1).trim();
        }
        return { key, value, enabled: !disabled };
      });

  const enterBulk = () => { setBulkText(toBulk(props.rows)); setBulk(true); };
  const exitBulk  = () => { props.onChange(fromBulk(bulkText())); setBulk(false); };

  return (
    <div>
      <div class="kv-toolbar">
        <button class="mini-btn" onClick={() => (bulk() ? exitBulk() : enterBulk())}>
          {bulk() ? "← Table" : "Bulk Edit"}
        </button>
      </div>
      <Show when={bulk()} fallback={
        <KVEditor rows={props.rows} onChange={props.onChange} keyPlaceholder={props.keyPlaceholder} />
      }>
        <textarea
          class="bulk-edit-area"
          value={bulkText()}
          onInput={(e) => setBulkText(e.currentTarget.value)}
          placeholder={`key${sep()}value\n# disabled${sep()}row`}
          spellcheck={false}
        />
      </Show>
    </div>
  );
}

function MultipartEditor() {
  const rows = () => request.body.form ?? [];
  const update = (i: number, patch: Partial<KV>) => {
    setRequest(
      "body",
      "form",
      rows().map((r, idx) => (idx === i ? { ...r, ...patch } : r)) as KV[]
    );
    setDirty(true);
  };
  const remove = (i: number) => {
    setRequest("body", "form", rows().filter((_, idx) => idx !== i) as KV[]);
    setDirty(true);
  };
  const add = () => {
    setRequest("body", "form", [...rows(), blankKV()]);
    setDirty(true);
  };
  const browse = async (i: number) => {
    const path = await api.pickFile("Choose file to upload");
    if (path) update(i, { value: path, file: true });
  };

  return (
    <div class="kv-editor">
      <Index each={rows()}>
        {(row, i) => (
          <div class="kv-row" classList={{ disabled: !row().enabled }}>
            <input
              type="checkbox"
              checked={row().enabled}
              onChange={(e) => update(i, { enabled: e.currentTarget.checked })}
              title="Enable / disable"
            />
            <input
              class="kv-key"
              placeholder="field"
              value={row().key}
              onInput={(e) => update(i, { key: e.currentTarget.value })}
            />
            <select
              class="assert-op"
              value={row().file ? "file" : "text"}
              onChange={(e) => update(i, { file: e.currentTarget.value === "file" })}
              title="Field kind"
            >
              <option value="text">text</option>
              <option value="file">file</option>
            </select>
            <input
              class="kv-val"
              placeholder={row().file ? "/path/to/file" : "value"}
              value={row().value}
              onInput={(e) => update(i, { value: e.currentTarget.value })}
            />
            <Show when={row().file}>
              <button class="mini-btn" onClick={() => browse(i)} title="Browse for file">
                …
              </button>
            </Show>
            <button class="icon-btn" onClick={() => remove(i)} title="Remove">
              <X size={ICON.sm} />
            </button>
          </div>
        )}
      </Index>
      <button class="add-row" onClick={add}>
        <Plus size={ICON.xs} /> Add field
      </button>
    </div>
  );
}

// Full introspection query: buildClientSchema needs the complete shape to
// drive editor validation + autocomplete (the old reduced query wasn't enough).
// Collapsed to one line — same single-line shape as the original working query.
const GQL_INTROSPECT = JSON.stringify({
  query: getIntrospectionQuery({
    inputValueDeprecation: false,
    specifiedByUrl: false,
    directiveIsRepeatable: false,
    schemaDescription: false,
  })
    .replace(/\s+/g, " ")
    .trim(),
});

function GraphQLEditor() {
  const [schema, setSchema] = createSignal<{ name: string; fields: string[] }[]>([]);
  const [gqlSchema, setGqlSchema] = createSignal<GraphQLSchema | undefined>();
  const [introspecting, setIntrospecting] = createSignal(false);
  const [schemaError, setSchemaError] = createSignal("");
  const [showSchema, setShowSchema] = createSignal(false);

  const introspect = async () => {
    if (!request.url) return;
    setIntrospecting(true);
    setSchemaError("");
    try {
      const introspectReq = {
        ...request,
        method: "POST",
        body: { type: BodyType.BodyJSON, raw: GQL_INTROSPECT, form: [], variables: "" },
        headers: [
          ...request.headers.filter((h) => h.key.toLowerCase() !== "content-type"),
          { key: "Content-Type", value: "application/json", enabled: true },
        ],
      };
      const resp = await api.send(introspectReq, collection()?.path ?? "", activePath(), activeEnv() ?? "");
      if (resp.error) { setSchemaError(resp.error); return; }
      const json = JSON.parse(resp.body);
      if (json?.errors?.length) {
        setSchemaError(json.errors.map((e: any) => e.message).join("; "));
        return;
      }
      if (!json?.data?.__schema) {
        setSchemaError(`No schema (HTTP ${resp.status}): ${resp.body.slice(0, 160)}`);
        return;
      }
      const types = (json?.data?.__schema?.types ?? []) as any[];
      const filtered = types
        .filter((t: any) => t.kind === "OBJECT" && !t.name.startsWith("__"))
        .map((t: any) => ({
          name: t.name as string,
          fields: (t.fields ?? []).map((f: any) => f.name as string),
        }));
      setSchema(filtered);
      // Build a typed schema for in-editor validation + autocomplete.
      try {
        setGqlSchema(buildClientSchema(json.data));
      } catch {
        setGqlSchema(undefined);
      }
      setShowSchema(true);
    } catch (e) {
      setSchemaError(String(e));
    } finally {
      setIntrospecting(false);
    }
  };

  return (
    <div class="graphql-editor">
      <div class="gql-toolbar">
        <button class="mini-btn" onClick={introspect} disabled={introspecting() || !request.url}>
          {introspecting() ? "…" : "Introspect schema"}
        </button>
        <Show when={schema().length > 0}>
          <button class="mini-btn" onClick={() => setShowSchema(!showSchema())}>
            {showSchema() ? "Hide types" : `${schema().length} types`}
          </button>
        </Show>
        <Show when={schemaError()}>
          <span class="gql-error">{schemaError()}</span>
        </Show>
      </div>
      <Show when={showSchema() && schema().length > 0}>
        <div class="gql-schema">
          <For each={schema()}>
            {(t) => (
              <div class="gql-type">
                <span class="gql-type-name">{t.name}</span>
                <span class="gql-type-fields">{t.fields.join(", ")}</span>
              </div>
            )}
          </For>
        </div>
      </Show>
      <div class="gql-panes">
        <div class="gql-pane">
          <div class="gql-label">Query</div>
          <CodeEditor
            value={request.body.raw ?? ""}
            language="graphql"
            schema={gqlSchema()}
            onChange={(v) => {
              setRequest("body", "raw", v);
              setDirty(true);
            }}
          />
        </div>
        <div class="gql-pane gql-vars">
          <div class="gql-label">Variables (JSON)</div>
          <CodeEditor
            value={request.body.variables ?? ""}
            language="json"
            varComplete
            onChange={(v) => {
              setRequest("body", "variables", v);
              setDirty(true);
            }}
          />
        </div>
      </div>
    </div>
  );
}

function ScriptEditor() {
  const [section, setSection] = createSignal<"pre" | "post">("pre");
  const consoleLogs = () => response()?.scriptLogs ?? [];
  return (
    <div class="script-editor">
      <div class="script-switch">
        <button classList={{ active: section() === "pre" }} onClick={() => setSection("pre")}>
          Pre-request{request.preScript ? " •" : ""}
        </button>
        <button classList={{ active: section() === "post" }} onClick={() => setSection("post")}>
          Post-response{request.postScript ? " •" : ""}
        </button>
        <span class="script-hint">
          {section() === "pre"
            ? "mutate req (url, headers, body…); senda.setVar / senda.getVar / pm.*"
            : "read res (status, body, json, headers); senda.setVar / pm.test / pm.expect"}
        </span>
      </div>
      <Show when={section() === "pre"} fallback={
        <CodeEditor
          value={request.postScript ?? ""}
          language="text"
          onChange={(v) => {
            setRequest("postScript", v);
            setDirty(true);
          }}
        />
      }>
        <CodeEditor
          value={request.preScript ?? ""}
          language="text"
          onChange={(v) => {
            setRequest("preScript", v);
            setDirty(true);
          }}
        />
      </Show>
      <Show when={consoleLogs().length > 0}>
        <div class="script-console">
          <div class="script-console-header">Console</div>
          <div class="script-console-body">
            <For each={consoleLogs()}>
              {(line) => <div class="script-console-line">{line}</div>}
            </For>
          </div>
        </div>
      </Show>
    </div>
  );
}

function BodyEditor() {
  const TYPES: { value: Request["body"]["type"]; label: string }[] = [
    { value: BodyType.BodyNone, label: "None" },
    { value: BodyType.BodyJSON, label: "JSON" },
    { value: BodyType.BodyRaw, label: "Raw" },
    { value: BodyType.BodyForm, label: "Form" },
    { value: BodyType.BodyMultipart, label: "Multipart" },
    { value: BodyType.BodyGraphQL, label: "GraphQL" },
    { value: BodyType.BodyWebSocket, label: "WebSocket" },
    { value: BodyType.BodySSE, label: "SSE" },
  ];
  const [fmtError, setFmtError] = createSignal("");

  const format = () => {
    try {
      const pretty = JSON.stringify(JSON.parse(request.body.raw ?? ""), null, 2);
      setRequest("body", "raw", pretty);
      setDirty(true);
      setFmtError("");
    } catch (e) {
      setFmtError(e instanceof SyntaxError ? e.message : String(e));
    }
  };

  return (
    <div class="body-editor">
      <div class="body-types">
        <label class="body-type-field">
          Body
          <select
            class="body-type-select"
            value={request.body.type}
            onChange={(e) => {
              setRequest("body", "type", e.currentTarget.value as Request["body"]["type"]);
              setDirty(true);
              setFmtError("");
            }}
          >
            {TYPES.map((t) => (
              <option value={t.value}>{t.label}</option>
            ))}
          </select>
        </label>
        <Show when={request.body.type === "json"}>
          <span class="body-toolbar">
            <Show when={fmtError()}>
              <span class="fmt-error" title={fmtError()}>
                invalid JSON
              </span>
            </Show>
            <button class="mini-btn" onClick={format} title="Pretty-print JSON body">
              Format
            </button>
          </span>
        </Show>
      </div>
      <Switch>
        <Match when={request.body.type === "json" || request.body.type === "raw"}>
          <CodeEditor
            value={request.body.raw ?? ""}
            language={request.body.type === "json" ? "json" : "text"}
            varComplete
            onChange={(v) => {
              setRequest("body", "raw", v);
              setDirty(true);
            }}
          />
        </Match>
        <Match when={request.body.type === "form"}>
          <KVEditor
            rows={request.body.form ?? []}
            onChange={(r) => {
              setRequest("body", "form", r);
              setDirty(true);
            }}
          />
        </Match>
        <Match when={request.body.type === "multipart"}>
          <MultipartEditor />
        </Match>
        <Match when={request.body.type === "graphql"}>
          <GraphQLEditor />
        </Match>
        <Match when={request.body.type === "none"}>
          <div class="empty-hint">No body for this request.</div>
        </Match>
        <Match when={request.body.type === "websocket"}>
          <div class="empty-hint">Switch to the WebSocket tab to connect and send messages.</div>
        </Match>
        <Match when={request.body.type === "sse"}>
          <div class="empty-hint">Switch to the SSE tab to connect and stream events.</div>
        </Match>
      </Switch>
    </div>
  );
}

function WebSocketPanel() {
  const [session, setSession] = createSignal<WSSession | null>(null);
  const [connecting, setConnecting] = createSignal(false);

  const connect = async () => {
    setConnecting(true);
    try {
      const s = await api.connectWebSocket(request, collection()?.path ?? "", activeEnv() ?? "");
      setSession(s);
    } finally {
      setConnecting(false);
    }
  };

  return (
    <div class="ws-panel">
      <div class="ws-toolbar">
        <div class="ws-hint">Initial message is set in the body Raw field above. URL must be ws:// or wss://.</div>
        <button class="send-btn" onClick={connect} disabled={connecting()}>
          {connecting() ? "Connecting…" : session() ? "Reconnect" : "Connect"}
        </button>
      </div>
      <Show when={session()}>
        {(s) => (
          <div class="ws-session">
            <div class="ws-meta">
              <span classList={{ "ws-status-ok": !s().error, "ws-status-err": !!s().error }}>
                {s().error ? `Error: ${s().error}` : `Done — ${s().messages?.length ?? 0} messages`}
              </span>
              <Show when={s().closeCode > 0}>
                <span class="ws-duration">close {s().closeCode}</span>
              </Show>
            </div>
            <div class="ws-messages">
              <For each={s().messages ?? []}>
                {(msg) => (
                  <div class={`ws-msg ws-msg-${msg.direction}`}>
                    <span class="ws-msg-dir">{msg.direction === "sent" ? "▲" : "▼"}</span>
                    <pre class="ws-msg-data">{msg.data}</pre>
                    <span class="ws-msg-time">{new Date(msg.at).toLocaleTimeString()}</span>
                  </div>
                )}
              </For>
            </div>
          </div>
        )}
      </Show>
      <Show when={!session() && !connecting()}>
        <div class="empty-hint">Click Connect to open WebSocket connection.</div>
      </Show>
    </div>
  );
}

function SSEPanel() {
  const [events, setEvents] = createSignal<SSEEvent[]>([]);
  const [connecting, setConnecting] = createSignal(false);
  const [error, setError] = createSignal("");

  const connect = async () => {
    setEvents([]);
    setError("");
    setConnecting(true);
    const unsub = Events.On("sse:event", (data: unknown) => {
      setEvents((prev) => [...prev, data as SSEEvent]);
    });
    try {
      const sess = await api.connectSSE(request, collection()?.path ?? "", activeEnv() ?? "");
      if (sess.error) setError(sess.error);
      else setEvents(sess.events ?? []);
    } catch (e) {
      setError(String(e));
    } finally {
      setConnecting(false);
      unsub();
    }
  };

  return (
    <div class="sse-panel">
      <div class="sse-toolbar">
        <button class="send-btn" onClick={connect} disabled={connecting()}>
          {connecting() ? "Listening…" : "Connect"}
        </button>
        <Show when={connecting()}>
          <span class="sse-live">● Streaming</span>
        </Show>
      </div>
      <Show when={error()}>
        <div class="ws-error">{error()}</div>
      </Show>
      <Show when={events().length > 0}>
        <div class="sse-events">
          <For each={events()}>
            {(evt) => (
              <div class="sse-event">
                <Show when={evt.event}>
                  <span class="sse-event-type">{evt.event}</span>
                </Show>
                <pre class="sse-event-data">{evt.data}</pre>
                <Show when={evt.id}>
                  <span class="sse-event-id">id:{evt.id}</span>
                </Show>
              </div>
            )}
          </For>
        </div>
      </Show>
      <Show when={!connecting() && events().length === 0 && !error()}>
        <div class="empty-hint">Click Connect to open SSE stream.</div>
      </Show>
    </div>
  );
}
