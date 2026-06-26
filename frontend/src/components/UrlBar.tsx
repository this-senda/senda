// Method + URL + save/codegen/send. Lives in the titlebar (wide free space)
// and drives the active request via the shared store.
import { createSignal, For, onCleanup, onMount, Show } from "solid-js";
import { Check, Code2, Save as SaveIcon } from "lucide-solid";
import { ICON } from "../lib/icons";
import { BodyType } from "../lib/api";
import { saveActive, sendActive } from "../lib/actions";
import { activePath, dirty, request, sending, setDirty, setRequest, setReqTab } from "../lib/store";
import CodeGenDialog from "./CodeGenDialog";
import UrlField from "./UrlField";

const METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"];

export default function UrlBar() {
  const [showCode, setShowCode] = createSignal(false);

  // ws/sse requests aren't HTTP — route Send to their tab (Connect lives there)
  // instead of firing a doomed http.Client call ("unsupported protocol scheme").
  const streamTab = () =>
    request.body?.type === BodyType.BodyWebSocket ? "ws"
    : request.body?.type === BodyType.BodySSE ? "sse"
    : null;
  const send = () => {
    const t = streamTab();
    if (t) return void setReqTab(t);
    void sendActive();
  };
  const save = () => void saveActive();

  return (
    <div class="url-bar">
      <div class={`url-group method-${request.method.toLowerCase()}`}>
        <Show when={streamTab()} fallback={<MethodSelect />}>
          {(t) => <span class="method-inline method-stream">{t() === "ws" ? "WS" : "SSE"}</span>}
        </Show>
        <span class="url-divider" />
        <UrlField />
        {/* Actions ride inside the pill, right edge. */}
        <div class="url-actions">
          <Show when={dirty()}>
            <button
              class="url-icon-btn dirty"
              title={activePath() ? "Save changes (⌘S)" : "Save request (⌘S)"}
              onClick={save}
            >
              <SaveIcon size={ICON.lg} />
            </button>
          </Show>
          <button class="url-icon-btn" title="Generate code" onClick={() => setShowCode(true)}>
            <Code2 size={ICON.lg} />
          </button>
          <button class="send-btn" onClick={send} disabled={sending()} title="Send (⏎)" aria-label="Send">
            {sending() ? "…" : (
              <svg width={ICON.lg} height={ICON.lg} viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                <path d="M3.4 20.4 21 12 3.4 3.6 3 10l12 2-12 2z" />
              </svg>
            )}
          </button>
        </div>
      </div>
      <Show when={showCode()}>
        <CodeGenDialog onClose={() => setShowCode(false)} />
      </Show>
    </div>
  );
}

// Custom verb dropdown — native <select> can't render per-option color rails +
// check. Trigger shows the colored verb; menu lists all methods.
function MethodSelect() {
  const [open, setOpen] = createSignal(false);
  let ref: HTMLDivElement | undefined;
  const onDoc = (e: MouseEvent) => {
    if (ref && !ref.contains(e.target as Node)) setOpen(false);
  };
  onMount(() => document.addEventListener("mousedown", onDoc));
  onCleanup(() => document.removeEventListener("mousedown", onDoc));
  const pick = (m: string) => {
    setRequest("method", m);
    setDirty(true);
    setOpen(false);
  };
  return (
    <div class="method-select" ref={ref}>
      <button
        class={`method-inline method-${request.method.toLowerCase()}`}
        onClick={() => setOpen(!open())}
        onKeyDown={(e) => e.key === "Escape" && setOpen(false)}
      >
        {request.method}
      </button>
      <Show when={open()}>
        <div class="method-menu">
          <div class="method-menu-head">Method</div>
          <For each={METHODS}>
            {(m) => (
              <button
                class={`method-opt method-${m.toLowerCase()}`}
                classList={{ selected: request.method === m }}
                onClick={() => pick(m)}
              >
                <span class="method-rail" />
                <span class="method-opt-label">{m}</span>
                <Show when={request.method === m}>
                  <Check size={14} class="method-check" />
                </Show>
              </button>
            )}
          </For>
        </div>
      </Show>
    </div>
  );
}
