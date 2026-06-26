// URL input with {{var}} pills + autocomplete. Two display modes share one
// editable string:
//   - rest (unfocused): a read view renders {{var}} tokens as compact name pills
//     (color = resolution status); hovering a pill shows a popover with its
//     resolved value and source scope. Plain text otherwise.
//   - edit (focused): a native <input> with a colored mirror behind it (the
//     input can't color its own text) plus a "{{" autocomplete popup.
// Clicking the read view (or pressing Enter on it) swaps to the raw editable
// text, caret placed at the clicked token.
import { createMemo, createResource, createSignal, For, Show } from "solid-js";
import { request, setRequest, setDirty, activeEnv, activePath, collection } from "../lib/store";
import { api, BodyType } from "../lib/api";
import { sendActive } from "../lib/actions";
import {
  buildScope,
  buildSources,
  sourceOf,
  splitVars,
  triggerAt,
  type Segment,
  type VarSource,
  type VarStatus,
} from "../lib/vars";

export default function UrlField() {
  let inputRef: HTMLInputElement | undefined;
  let highlightRef: HTMLDivElement | undefined;

  const [focused, setFocused] = createSignal(false);
  const [acOpen, setAcOpen] = createSignal(false);
  const [acStart, setAcStart] = createSignal(0); // index of the prefix start
  const [acItems, setAcItems] = createSignal<string[]>([]);
  const [acIdx, setAcIdx] = createSignal(0);

  // Resolve the scope server-side so it includes folder-chain variables and
  // secrets (the client can't read disk). Keyed on collection/request/env; the
  // resource keeps its previous value across refetches, so no flicker on switch.
  const scopeKey = createMemo(() => ({
    coll: collection()?.path ?? "",
    req: activePath(),
    env: activeEnv(),
  }));
  const [serverScope] = createResource(scopeKey, async (k) => {
    if (!k.coll) return [];
    return (await api.resolveScope(k.coll, k.req, k.env)) ?? [];
  });

  // name -> value, secrets excluded (their values never reach the client, so
  // they're masked in the popover and not offered as autocomplete values).
  const scope = createMemo(() => {
    const sv = serverScope();
    if (sv === undefined) return buildScope(); // still loading: client fallback
    const m = new Map<string, string>();
    for (const v of sv) if (v.source !== "secret") m.set(v.key, v.value);
    return m;
  });
  const sources = createMemo(() => {
    const sv = serverScope();
    if (sv === undefined) return buildSources();
    const m = new Map<string, VarSource>();
    for (const v of sv) m.set(v.key, v.source as VarSource);
    return m;
  });
  // splitVars marks status from `scope` (secrets are absent there). Re-tag any
  // token the server knows as a secret so it shows masked, not "missing".
  const segments = createMemo(() => {
    const src = sources();
    return splitVars(request.url, scope()).map((s) =>
      s.name && src.get(s.name) === "secret"
        ? { ...s, status: "secret" as VarStatus }
        : s,
    );
  });

  // Hover popover anchored to a pill (viewport coords, fixed-positioned).
  const [pop, setPop] = createSignal<{ seg: Segment; x: number; y: number } | null>(null);
  const showPop = (seg: Segment, el: HTMLElement) => {
    const r = el.getBoundingClientRect();
    setPop({ seg, x: r.left, y: r.bottom + 4 });
  };
  const hidePop = () => setPop(null);

  const srcLabel = (s: VarSource): string =>
    s === "collection"
      ? "collection variable"
      : s === "env"
        ? `environment · ${activeEnv() || "?"}`
        : s === "secret"
          ? "secret file (server-side)"
          : "not defined in this scope";

  // segs adds char offsets so a click on a pill can place the caret there.
  const segs = createMemo(() => {
    let pos = 0;
    return segments().map((s) => {
      const start = pos;
      pos += s.text.length;
      return { ...s, start, end: pos };
    });
  });

  // Show the raw input while focused, or when empty so the placeholder shows.
  const editing = () => focused() || request.url === "";

  // pillValue is the resolved value text shown in the popover.
  const pillValue = (seg: Segment) =>
    seg.status === "found" ? seg.value : seg.status === "secret" ? "•••• (server-side secret)" : "unresolved";

  // activate switches to edit mode and drops the caret at pos (default: end).
  const activate = (pos?: number) => {
    setFocused(true);
    queueMicrotask(() => {
      if (!inputRef) return;
      inputRef.focus();
      const p = pos ?? request.url.length;
      inputRef.setSelectionRange(p, p);
    });
  };

  const syncScroll = () => {
    if (highlightRef && inputRef) highlightRef.scrollLeft = inputRef.scrollLeft;
  };

  const closeAc = () => setAcOpen(false);

  // refreshAc recomputes the autocomplete popup from the caret position.
  const refreshAc = () => {
    if (!inputRef) return;
    const trig = triggerAt(request.url, caret());
    if (!trig) {
      closeAc();
      return;
    }
    const items = [...scope().keys()].filter((k) => k.startsWith(trig.prefix)).sort();
    if (items.length === 0) {
      closeAc();
      return;
    }
    setAcStart(trig.start);
    setAcItems(items);
    setAcIdx(0);
    setAcOpen(true);
  };

  // accept replaces the {{ token being completed with the chosen name, always
  // closing the braces. It consumes any leftover token text after the caret
  // (e.g. when re-picking inside an already-complete {{baseUrl}}) so the old
  // name + closing braces don't survive and duplicate.
  const caret = () => inputRef?.selectionStart ?? request.url.length;
  const accept = (name: string) => {
    if (!inputRef) return;
    const start = acStart();
    // Drop any remaining name chars + optional closing braces right of caret.
    const rest = request.url.slice(caret()).replace(/^[\w.-]*\}{0,2}/, "");
    const next = request.url.slice(0, start) + name + "}}" + rest;
    setRequest("url", next);
    setDirty(true);
    closeAc();
    const pos = start + name.length + 2;
    queueMicrotask(() => {
      inputRef!.focus();
      inputRef!.setSelectionRange(pos, pos);
    });
  };

  const onKeyDown = (e: KeyboardEvent) => {
    if (acOpen()) {
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setAcIdx((i) => (i + 1) % acItems().length);
        return;
      }
      if (e.key === "ArrowUp") {
        e.preventDefault();
        setAcIdx((i) => (i - 1 + acItems().length) % acItems().length);
        return;
      }
      if (e.key === "Enter" || e.key === "Tab") {
        e.preventDefault();
        accept(acItems()[acIdx()]);
        return;
      }
      if (e.key === "Escape") {
        e.preventDefault();
        closeAc();
        return;
      }
    }
    if (e.key === "Enter") void sendActive();
  };

  return (
    <div class="url-field">
      <Show
        when={editing()}
        fallback={
          <div
            class="url-read"
            tabindex="0"
            role="textbox"
            aria-label="Request URL"
            onMouseDown={(e) => {
              e.preventDefault();
              activate();
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                activate();
              }
            }}
          >
            <For each={segs()}>
              {(seg) =>
                seg.status ? (
                  <span
                    class={`url-pill pill-${seg.status}`}
                    aria-label={`{{${seg.name}}} → ${pillValue(seg)}`}
                    onMouseDown={(e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      activate(seg.end);
                    }}
                    onMouseEnter={(e) => showPop(seg, e.currentTarget)}
                    onMouseLeave={hidePop}
                  >
                    {seg.name}
                  </span>
                ) : (
                  <span>{seg.text}</span>
                )
              }
            </For>
          </div>
        }
      >
        <div class="url-highlight" aria-hidden="true" ref={highlightRef}>
          <For each={segments()}>
            {(seg) => (
              <span classList={{ [`var-${seg.status}`]: !!seg.status }}>{seg.text}</span>
            )}
          </For>
        </div>
        <input
          class="url-input"
          ref={inputRef}
          placeholder="https://api.example.com/{{path}}"
          value={request.url}
          spellcheck={false}
          autocomplete="off"
          onFocus={() => setFocused(true)}
          onInput={(e) => {
            const v = e.currentTarget.value;
            setRequest("url", v);
            // ws://wss:// scheme is unambiguous → flip body to WebSocket so the tab/Connect
            // appear. Only when body is None, so we never clobber an explicit body choice.
            if (/^wss?:\/\//i.test(v) && request.body?.type === BodyType.BodyNone) {
              setRequest("body", "type", BodyType.BodyWebSocket);
            }
            setDirty(true);
            syncScroll();
            refreshAc();
          }}
          onScroll={syncScroll}
          onKeyUp={(e) => {
            // Caret moves (arrows, home/end) without changing text.
            if (["ArrowLeft", "ArrowRight", "Home", "End"].includes(e.key)) refreshAc();
          }}
          onClick={refreshAc}
          onBlur={() => {
            setTimeout(closeAc, 120);
            setFocused(false);
          }}
          onKeyDown={onKeyDown}
        />
      </Show>
      <Show when={acOpen()}>
        <ul class="url-ac">
          <For each={acItems()}>
            {(name, i) => (
              <li
                classList={{ active: i() === acIdx() }}
                onMouseDown={(e) => {
                  e.preventDefault();
                  accept(name);
                }}
                onMouseEnter={() => setAcIdx(i())}
              >
                <span class="url-ac-name">{name}</span>
                <span class="url-ac-val">{scope().get(name)}</span>
              </li>
            )}
          </For>
        </ul>
      </Show>
      <Show when={pop()}>
        {(p) => (
          <div class="pill-pop" style={{ left: `${p().x}px`, top: `${p().y}px` }}>
            <code class="pop-name">{`{{${p().seg.name}}}`}</code>
            <span class={`pop-val pv-${p().seg.status}`}>{pillValue(p().seg)}</span>
            <span class="pop-src">{srcLabel(sourceOf(p().seg.name!, sources()))}</span>
          </div>
        )}
      </Show>
    </div>
  );
}
