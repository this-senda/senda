// Response pane: status line + body / headers tabs. Large bodies are flagged
// and gated behind an explicit "show" action (escape hatch).
import { createEffect, createMemo, createSignal, For, Index, Match, onCleanup, onMount, Show, Switch } from "solid-js";
import { AlertTriangle, Check, Copy, Download, FilePlus, MoreHorizontal, Search, X } from "lucide-solid";
import { ICON } from "../lib/icons";
import { api, BodyType } from "../lib/api";
import { collection, request, response, sending } from "../lib/store";
import { cancelSend } from "../lib/actions";
import { formatBytes, statusClass } from "../lib/factory";
import { toBase64, toHex } from "../lib/format";
import { alertDialog } from "../lib/dialog";
import CodeEditor from "./CodeEditor";
import JsonTree from "./JsonTree";
import Timeline from "./Timeline";
import ViewModeMenu, { type Option } from "./ViewModeMenu";

type Tab = "body" | "headers" | "timeline" | "tests";
type ViewMode = "pretty" | "preview" | "raw" | "hex" | "base64";

function HighlightedBody(props: { text: string; term: string }) {
  const parts = () => {
    const t = props.term.toLowerCase();
    if (!t) return [{ text: props.text, match: false }];
    const result: { text: string; match: boolean }[] = [];
    let remaining = props.text;
    let lower = remaining.toLowerCase();
    let idx: number;
    while ((idx = lower.indexOf(t)) !== -1) {
      if (idx > 0) result.push({ text: remaining.slice(0, idx), match: false });
      result.push({ text: remaining.slice(idx, idx + t.length), match: true });
      remaining = remaining.slice(idx + t.length);
      lower = remaining.toLowerCase();
    }
    if (remaining) result.push({ text: remaining, match: false });
    return result;
  };
  return (
    <pre class="resp-highlight-body">
      <Index each={parts()}>
        {(p) => p().match
          ? <mark class="resp-highlight-mark">{p().text}</mark>
          : <span>{p().text}</span>
        }
      </Index>
    </pre>
  );
}

const MODES: Option<ViewMode>[] = [
  { value: "pretty", label: "JSON" },
  { value: "preview", label: "Preview" },
  { value: "raw", label: "Raw" },
  { value: "hex", label: "Hex" },
  { value: "base64", label: "Base64" },
];

// Elapsed-seconds ticker shown while a request is in flight, with a cancel
// button that aborts the backend call.
function Sending() {
  const [elapsed, setElapsed] = createSignal(0);
  onMount(() => {
    const t0 = performance.now();
    const id = setInterval(() => setElapsed((performance.now() - t0) / 1000), 100);
    onCleanup(() => clearInterval(id));
  });
  return (
    <div class="resp-sending">
      <span class="resp-spinner" />
      <span class="resp-elapsed">{elapsed().toFixed(1)}s</span>
      <button class="btn resp-cancel" onClick={cancelSend}>
        Cancel
      </button>
    </div>
  );
}

// Kebab overflow menu for the response action buttons, so the status line
// doesn't wrap when the pane is narrow.
function OverflowMenu(props: { items: { icon: () => any; label: string; onClick: () => void }[] }) {
  const [open, setOpen] = createSignal(false);
  let root!: HTMLDivElement;
  const onDocDown = (e: MouseEvent) => {
    if (open() && root && !root.contains(e.target as HTMLElement)) setOpen(false);
  };
  onMount(() => document.addEventListener("mousedown", onDocDown));
  onCleanup(() => document.removeEventListener("mousedown", onDocDown));
  return (
    <div class="viewmode" ref={root}>
      <button class="icon-btn" title="More actions" onClick={() => setOpen(!open())}>
        <MoreHorizontal size={ICON.sm} />
      </button>
      <Show when={open()}>
        <div class="viewmode-menu">
          <For each={props.items}>
            {(it) => (
              <button class="viewmode-item" onClick={() => { it.onClick(); setOpen(false); }}>
                {it.label}
                {it.icon()}
              </button>
            )}
          </For>
        </div>
      </Show>
    </div>
  );
}

export default function ResponseViewer() {
  const [tab, setTab] = createSignal<Tab>("body");
  const [forceShow, setForceShow] = createSignal(false);
  const [mode, setMode] = createSignal<ViewMode>("pretty");
  const [searchOpen, setSearchOpen] = createSignal(false);
  const [searchTerm, setSearchTerm] = createSignal("");

  onMount(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "f" && tab() === "body" && response()) {
        e.preventDefault();
        setSearchOpen(true);
      }
      if (e.key === "Escape" && searchOpen()) {
        setSearchOpen(false);
        setSearchTerm("");
      }
    };
    window.addEventListener("keydown", handler);
    onCleanup(() => window.removeEventListener("keydown", handler));
  });

  const looksJSON = createMemo(() => {
    const b = response()?.body?.trimStart() ?? "";
    return b.startsWith("{") || b.startsWith("[");
  });

  const contentType = createMemo(() => {
    const headers = response()?.headers ?? {};
    for (const [k, vals] of Object.entries(headers)) {
      if (k.toLowerCase() === "content-type") return (vals ?? []).join(";").toLowerCase();
    }
    return "";
  });

  const looksHTML = createMemo(() => {
    if (contentType().includes("text/html")) return true;
    const b = (response()?.body ?? "").trimStart().slice(0, 200).toLowerCase();
    return b.startsWith("<!doctype html") || b.startsWith("<html");
  });

  // Schema assert results extracted from response asserts for status bar badge.
  const schemaResults = createMemo(() =>
    (response()?.asserts ?? []).filter((a) => (a.target ?? "").startsWith("schema"))
  );
  const schemaPass = createMemo(() => schemaResults().every((a) => a.pass));

  // Pick a sensible default mode whenever a new response arrives: tree for
  // JSON, rendered preview for HTML, raw otherwise.
  createEffect(() => {
    const r = response();
    setMode(looksJSON() ? "pretty" : looksHTML() ? "preview" : "raw");
    setForceShow(false);
    if (tab() === "tests" && !(r?.asserts?.length)) setTab("body");
  });

  const copyBody = () => {
    void navigator.clipboard.writeText(response()?.body ?? "");
  };

  const saveBody = () => {
    const ext = looksJSON() ? "json" : looksHTML() ? "html" : "txt";
    void api.exportFile(`response.${ext}`, response()?.body ?? "").catch(() => {});
  };

  const [savedMock, setSavedMock] = createSignal(false);
  const saveAsMock = async () => {
    const coll = collection();
    const r = response();
    if (!coll || !r) return;
    const headers: Record<string, string> = {};
    for (const [k, vals] of Object.entries(r.headers ?? {})) {
      if (vals && vals[0]) headers[k] = vals[0];
    }
    // Strip scheme+host from the request URL to get the mock path.
    const path = (request.url ?? "").replace(/^[a-z]+:\/\/[^/]+/i, "") || "/";
    try {
      await api.saveResponseAsMock(
        coll.path,
        request.name || "mock",
        request.method,
        path,
        r.status,
        headers,
        r.body ?? "",
      );
      setSavedMock(true);
      setTimeout(() => setSavedMock(false), 1500);
    } catch (e) {
      await alertDialog("Could not save mock: " + e);
    }
  };


  return (
    <div class="response-viewer">
      <Switch>
        <Match when={sending()}>
          <Sending />
        </Match>
        <Match when={request.body?.type === BodyType.BodyWebSocket}>
          <div class="resp-empty">WebSocket session — connect and send messages from the WebSocket tab.</div>
        </Match>
        <Match when={request.body?.type === BodyType.BodySSE}>
          <div class="resp-empty">SSE stream — connect and watch events from the SSE tab.</div>
        </Match>
        <Match when={!response()}>
          <div class="resp-empty">Send a request to see the response.</div>
        </Match>
        <Match when={response()!.error}>
          <div class="resp-error"><AlertTriangle size={ICON.md} /> {response()!.error}</div>
        </Match>
        <Match when={response()}>
          {(() => {
            const r = response()!;
            const asserts = () => r.asserts ?? [];
            const failed = () => asserts().filter((a) => !a.pass).length;
            return (
              <>
                <div class="status-line">
                  <span class={`status-badge ${statusClass(r.status)}`}>
                    {r.status} {r.statusText}
                  </span>
                  <span class="meta">{r.durationMs} ms</span>
                  <span class="meta">{formatBytes(r.sizeBytes)}</span>
                  <Show when={r.truncated}>
                    <span class="meta warn">truncated</span>
                  </Show>
                  <Show when={schemaResults().length > 0}>
                    <span
                      class={`schema-badge ${schemaPass() ? "schema-pass" : "schema-fail"}`}
                      title={schemaPass() ? "Schema valid" : "Schema validation failed"}
                    >
                      {schemaPass() ? "✓ schema" : "✗ schema"}
                    </span>
                  </Show>
                  <span class="status-spacer" />
                  <Show when={tab() === "body"}>
                    <button
                      class="icon-btn"
                      title="Search in body (Ctrl+F)"
                      onClick={() => setSearchOpen(!searchOpen())}
                      classList={{ active: searchOpen() }}
                    >
                      <Search size={ICON.sm} />
                    </button>
                    <ViewModeMenu value={mode()} options={MODES} onChange={setMode} />
                    <OverflowMenu
                      items={[
                        { label: "Copy body", icon: () => <Copy size={ICON.sm} />, onClick: copyBody },
                        { label: "Save body to file", icon: () => <Download size={ICON.sm} />, onClick: saveBody },
                        {
                          label: savedMock() ? "Saved as mock" : "Save as mock",
                          icon: () => (savedMock() ? <Check size={ICON.sm} /> : <FilePlus size={ICON.sm} />),
                          onClick: () => void saveAsMock(),
                        },
                      ]}
                    />
                  </Show>
                </div>
                <Show when={searchOpen() && tab() === "body"}>
                  <div class="resp-search-bar">
                    <Search size={ICON.xs} />
                    <input
                      class="resp-search-input"
                      placeholder="Search in body…"
                      value={searchTerm()}
                      onInput={(e) => setSearchTerm(e.currentTarget.value)}
                      autofocus
                    />
                    <Show when={searchTerm()}>
                      <span class="resp-search-count">
                        {(() => {
                          const body = response()?.body ?? "";
                          const term = searchTerm().toLowerCase();
                          if (!term) return "";
                          const matches = body.toLowerCase().split(term).length - 1;
                          return `${matches} match${matches !== 1 ? "es" : ""}`;
                        })()}
                      </span>
                    </Show>
                    <button class="icon-btn" onClick={() => { setSearchOpen(false); setSearchTerm(""); }}>
                      <X size={ICON.xs} />
                    </button>
                  </div>
                </Show>

                <div class="tabs">
                  <button classList={{ active: tab() === "body" }} onClick={() => setTab("body")}>
                    Body
                  </button>
                  <button classList={{ active: tab() === "headers" }} onClick={() => setTab("headers")}>
                    Headers
                  </button>
                  <Show when={r.timing}>
                    <button classList={{ active: tab() === "timeline" }} onClick={() => setTab("timeline")}>
                      Timeline
                    </button>
                  </Show>
                  <Show when={asserts().length}>
                    <button classList={{ active: tab() === "tests" }} onClick={() => setTab("tests")}>
                      Tests{" "}
                      <span class="assert-badge" classList={{ ok: failed() === 0, err: failed() > 0 }}>
                        {asserts().length - failed()}/{asserts().length}
                      </span>
                    </button>
                  </Show>
                </div>

                <div class="tab-body">
                  <Switch>
                    <Match when={tab() === "body"}>
                      <Show
                        when={!r.truncated || forceShow()}
                        fallback={
                          <div class="too-large">
                            <p>
                              Response is large ({formatBytes(r.sizeBytes)}).
                              Showing it inline may be slow.
                            </p>
                            <button onClick={() => setForceShow(true)}>
                              Show anyway
                            </button>
                          </div>
                        }
                      >
                        <Show when={searchOpen() && searchTerm()} fallback={
                          <Switch>
                            <Match when={mode() === "pretty"}>
                              <JsonTree text={r.body} onParseError={() => setMode("raw")} />
                            </Match>
                            <Match when={mode() === "preview"}>
                              <iframe class="resp-preview" sandbox="" srcdoc={r.body} title="Response preview" />
                            </Match>
                            <Match when={mode() === "hex"}>
                              <CodeEditor value={toHex(r.body)} language="text" readOnly />
                            </Match>
                            <Match when={mode() === "base64"}>
                              <CodeEditor value={toBase64(r.body)} language="text" readOnly />
                            </Match>
                            <Match when={mode() === "raw"}>
                              <CodeEditor value={r.body} language={looksJSON() ? "json" : "text"} readOnly />
                            </Match>
                          </Switch>
                        }>
                          <HighlightedBody text={r.body} term={searchTerm()} />
                        </Show>
                      </Show>
                    </Match>
                    <Match when={tab() === "timeline"}>
                      <Timeline response={r} />
                    </Match>
                    <Match when={tab() === "tests"}>
                      <div class="assert-results">
                        <For each={asserts()}>
                          {(a) => (
                            <div class="assert-row" classList={{ pass: a.pass, fail: !a.pass }}>
                              <span class="assert-mark">{a.pass ? <Check size={ICON.sm} /> : <X size={ICON.sm} />}</span>
                              <span class="assert-expr">
                                {a.target} {a.op}
                                {a.value ? ` ${a.value}` : ""}
                              </span>
                              <Show when={!a.pass}>
                                <span class="assert-detail">
                                  {a.error ? a.error : `actual: ${a.actual ?? ""}`}
                                </span>
                              </Show>
                            </div>
                          )}
                        </For>
                      </div>
                    </Match>
                    <Match when={tab() === "headers"}>
                      <div class="resp-headers">
                        <For each={Object.entries(r.headers)}>
                          {([k, vals]) => (
                            <div class="hdr-row">
                              <span class="hdr-key">{k}</span>
                              <span class="hdr-val">{(vals ?? []).join(", ")}</span>
                            </div>
                          )}
                        </For>
                      </div>
                    </Match>
                  </Switch>
                </div>
              </>
            );
          })()}
        </Match>
      </Switch>
    </div>
  );
}
