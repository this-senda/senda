// Collapsible, lazily-rendered JSON tree for the response viewer. Children only
// mount when a node is expanded (<Show>), and large arrays are split into
// nested count buckets, so even a huge response keeps the DOM tiny → smooth.
import { createContext, createEffect, createMemo, createSignal, For, on, Show, useContext } from "solid-js";
import { ChevronRight, ChevronsDownUp, ChevronsUpDown, WrapText } from "lucide-solid";
// eslint note: ChevronsUpDown = unfold, ChevronsDownUp = fold.
import { ICON } from "../lib/icons";

// FoldCmd broadcasts fold-all / unfold-all to every mounted node: bump v and
// set open. Nodes that mount later keep their own default state.
export type FoldCmd = { v: number; open: boolean };

type Props = {
  text: string;
  onParseError?: () => void;
  // Hide the built-in fold/unfold buttons (e.g. when a parent provides its own).
  controls?: boolean;
};

const FoldContext = createContext<() => FoldCmd>(() => ({ v: 0, open: true }));

const CHUNK = 100; // max elements rendered directly before bucketing

export default function JsonTree(props: Props) {
  // The fold/unfold-all control lives inside the tree so every consumer — the
  // response pane, the folder-run detail modal, etc. — gets it for free.
  const [fold, setFold] = createSignal<FoldCmd>({ v: 0, open: true });
  // Single toggle: tracks the last broadcast state so one button flips between
  // fold-all / unfold-all.
  const [allOpen, setAllOpen] = createSignal(true);
  const toggleAll = () => {
    const next = !allOpen();
    setAllOpen(next);
    setFold((c) => ({ v: c.v + 1, open: next }));
  };
  // Wrap long string values vs. single-line + horizontal scroll. Persisted so
  // it survives response/request switches (default on).
  const [wrap, setWrap] = createSignal(localStorage.getItem("senda.jsonWrap") !== "0");
  createEffect(() => localStorage.setItem("senda.jsonWrap", wrap() ? "1" : "0"));
  const parsed = createMemo(() => {
    try {
      return { ok: true as const, value: JSON.parse(props.text) };
    } catch {
      props.onParseError?.();
      return { ok: false as const, value: undefined };
    }
  });

  return (
    <div class="json-tree" classList={{ wrap: wrap() }}>
      <Show when={parsed().ok}>
        <Show when={props.controls !== false}>
          <div class="jt-controls">
            <button
              class="icon-btn"
              classList={{ active: wrap() }}
              title={wrap() ? "Disable wrap" : "Wrap long values"}
              onClick={() => setWrap((w) => !w)}
            >
              <WrapText size={ICON.sm} />
            </button>
            <button
              class="icon-btn"
              title={allOpen() ? "Fold all" : "Unfold all"}
              onClick={toggleAll}
            >
              {allOpen() ? <ChevronsDownUp size={ICON.sm} /> : <ChevronsUpDown size={ICON.sm} />}
            </button>
          </div>
        </Show>
        <FoldContext.Provider value={fold}>
          <Node value={parsed().value} keyName={null} depth={0} rootOpen />
        </FoldContext.Provider>
      </Show>
    </div>
  );
}

function typeOf(v: unknown): "object" | "array" | "string" | "number" | "boolean" | "null" {
  if (v === null) return "null";
  if (Array.isArray(v)) return "array";
  return typeof v as any;
}

function countLabel(n: number): string {
  return `${n} item${n === 1 ? "" : "s"}`;
}

// A single value: primitive (leaf) or object/array (expandable).
function Node(props: {
  value: unknown;
  keyName: string | number | null;
  depth: number;
  rootOpen?: boolean;
}) {
  const t = () => typeOf(props.value);
  // Default to expanded (unfold-all by default). Large arrays still collapse via
  // Bucket, so a huge response can't explode the DOM on first render.
  const [open, setOpen] = createSignal(true);
  const indent = () => ({ "padding-left": `${props.depth * 14}px` });

  const fold = useContext(FoldContext);
  createEffect(on(fold, (c) => c.v > 0 && setOpen(c.open), { defer: true }));

  const KeyLabel = () => (
    <Show when={props.keyName !== null}>
      <span class="jt-key">{String(props.keyName)}</span>
      <span class="jt-colon">:</span>{" "}
    </Show>
  );

  return (
    <Show
      when={t() === "object" || t() === "array"}
      fallback={
        <div class="jt-row" style={indent()}>
          <span class="jt-caret-spacer" />
          <KeyLabel />
          <Primitive value={props.value} kind={t()} />
        </div>
      }
    >
      {(() => {
        const isArray = () => t() === "array";
        const entries = () =>
          isArray()
            ? (props.value as unknown[])
            : Object.entries(props.value as Record<string, unknown>);
        const len = () => entries().length;
        return (
          <div class="jt-node">
            <div class="jt-row jt-clickable" style={indent()} onClick={() => setOpen(!open())}>
              <span class="jt-caret" classList={{ open: open() }}><ChevronRight size={ICON.md} /></span>
              <KeyLabel />
              <span class="jt-bracket">{isArray() ? "[" : "{"}</span>
              <Show when={!open()}>
                <span class="jt-ellipsis">…</span>
                <span class="jt-bracket">{isArray() ? "]" : "}"}</span>
              </Show>
              <span class="jt-count">{countLabel(len())}</span>
            </div>
            <Show when={open()}>
              <Show
                when={isArray()}
                fallback={
                  <For each={entries() as [string, unknown][]}>
                    {([k, v]) => <Node value={v} keyName={k} depth={props.depth + 1} />}
                  </For>
                }
              >
                <ArrayRange
                  arr={props.value as unknown[]}
                  lo={0}
                  hi={(props.value as unknown[]).length}
                  depth={props.depth + 1}
                />
              </Show>
              <div class="jt-row" style={indent()}>
                <span class="jt-caret-spacer" />
                <span class="jt-bracket">{isArray() ? "]" : "}"}</span>
              </div>
            </Show>
          </div>
        );
      })()}
    </Show>
  );
}

// Render array indices [lo, hi). Small ranges render elements directly; large
// ranges split into nested count buckets, each lazily expandable.
function ArrayRange(props: { arr: unknown[]; lo: number; hi: number; depth: number }) {
  const span = () => props.hi - props.lo;

  return (
    <Show
      when={span() > CHUNK}
      fallback={
        <For each={range(props.lo, props.hi)}>
          {(i) => <Node value={props.arr[i]} keyName={i} depth={props.depth} />}
        </For>
      }
    >
      {(() => {
        // bucket size = largest power-of-CHUNK step that keeps <= CHUNK buckets
        let step = CHUNK;
        while (span() / step > CHUNK) step *= CHUNK;
        const starts = range(props.lo, props.hi, step);
        return (
          <For each={starts}>
            {(start) => (
              <Bucket
                arr={props.arr}
                lo={start}
                hi={Math.min(start + step, props.hi)}
                depth={props.depth}
              />
            )}
          </For>
        );
      })()}
    </Show>
  );
}

function Bucket(props: { arr: unknown[]; lo: number; hi: number; depth: number }) {
  const [open, setOpen] = createSignal(false);
  const indent = () => ({ "padding-left": `${props.depth * 14}px` });

  // Buckets only obey fold-all; unfold-all skips them so a 100k-element array
  // can't explode the DOM in one click.
  const fold = useContext(FoldContext);
  createEffect(on(fold, (c) => c.v > 0 && !c.open && setOpen(false), { defer: true }));
  return (
    <div class="jt-node">
      <div class="jt-row jt-clickable" style={indent()} onClick={() => setOpen(!open())}>
        <span class="jt-caret" classList={{ open: open() }}><ChevronRight size={ICON.md} /></span>
        <span class="jt-bracket">[</span>
        <span class="jt-count">{countLabel(props.hi - props.lo)}</span>
        <span class="jt-range">
          {props.lo} – {props.hi - 1}
        </span>
      </div>
      <Show when={open()}>
        <ArrayRange arr={props.arr} lo={props.lo} hi={props.hi} depth={props.depth + 1} />
      </Show>
    </div>
  );
}

function Primitive(props: { value: unknown; kind: string }) {
  const text = () =>
    props.kind === "string" ? JSON.stringify(props.value) : String(props.value);
  return <span class="jt-val" classList={{ [`jt-${props.kind}`]: true }}>{text()}</span>;
}

function range(lo: number, hi: number, step = 1): number[] {
  const out: number[] = [];
  for (let i = lo; i < hi; i += step) out.push(i);
  return out;
}
