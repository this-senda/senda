// Flows modal: lists the *.flow.yaml graphs under .senda/flows/, shows a
// selected flow's structure (nodes + edges) in execution order so you can see
// what it does before running, then runs it and streams the steps. You can also
// create, edit (raw YAML, with live validation) and delete flows in place;
// edits write the file verbatim and file-watch reloads.
import { createSignal, For, onMount, Show } from "solid-js";
import { Events } from "@wailsio/runtime";
import { X, Play, Plus, Pencil, Trash2, CornerDownRight, Workflow, ChevronDown, ChevronRight } from "lucide-solid";
import { ICON } from "../lib/icons";
import { api } from "../lib/api";
import type { FlowInfo, FlowStep, Flow } from "../lib/api";
import { collection, activeEnv } from "../lib/store";
import { statusClass } from "../lib/factory";
import { confirmDialog, promptDialog } from "../lib/dialog";
import CodeEditor from "./CodeEditor";

// summary renders the one-line description of a node's payload.
function summary(node: any): string {
  switch (node.type) {
    case "request":
      return node.request ?? "";
    case "branch":
      return `if ${node.cond?.left ?? ""} ${node.cond?.op ?? ""} ${node.cond?.right ?? ""}`;
    case "setvar":
      return `${node.var ?? ""} = ${node.from ?? ""}`;
    case "delay":
      return `${node.ms ?? 0}ms`;
    case "loop":
      return `data ${node.data ?? ""} · body [${(node.body ?? []).join(", ")}]`;
    case "parallel":
      return `branches ${(node.branches ?? []).map((b: string[]) => `[${b.join(", ")}]`).join(" ")}`;
    default:
      return "";
  }
}

// edges lists a node's outgoing transitions for the graph view.
function edges(node: any): { label: string; to: string }[] {
  if (node.type === "branch") {
    const out: { label: string; to: string }[] = [];
    if (node.onTrue) out.push({ label: "true", to: node.onTrue });
    if (node.onFalse) out.push({ label: "false", to: node.onFalse });
    return out;
  }
  return node.next ? [{ label: "next", to: node.next }] : [];
}

// ordered lays the nodes out in execution order: a DFS from `start` following
// next/onTrue/onFalse, emitting a loop/parallel container's owned body ids right
// after it, then appending any unreachable nodes so nothing is hidden. Go map
// JSON order is otherwise arbitrary.
function ordered(def: Flow): [string, any][] {
  const nodes: Record<string, any> = (def.nodes as any) ?? {};
  const seen = new Set<string>();
  const out: [string, any][] = [];
  const emit = (id: string) => {
    if (!id || seen.has(id) || !nodes[id]) return false;
    seen.add(id);
    out.push([id, nodes[id]]);
    return true;
  };
  const visit = (id: string) => {
    if (!emit(id)) return;
    const node = nodes[id];
    if (node.type === "loop") (node.body ?? []).forEach((b: string) => emit(b));
    if (node.type === "parallel") (node.branches ?? []).forEach((br: string[]) => br.forEach((b) => emit(b)));
    if (node.type === "branch") {
      visit(node.onTrue);
      visit(node.onFalse);
    } else {
      visit(node.next);
    }
  };
  visit(def.start);
  for (const id of Object.keys(nodes)) if (!seen.has(id)) out.push([id, nodes[id]]);
  return out;
}

export default function FlowPanel(props: { onClose: () => void }) {
  const [flows, setFlows] = createSignal<FlowInfo[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [selected, setSelected] = createSignal<FlowInfo | null>(null);
  const [def, setDef] = createSignal<Flow | null>(null);
  const [running, setRunning] = createSignal(false);
  const [steps, setSteps] = createSignal<FlowStep[]>([]);
  const [expanded, setExpanded] = createSignal<number | null>(null);
  const [error, setError] = createSignal("");
  const [editing, setEditing] = createSignal(false);
  const [raw, setRaw] = createSignal("");
  const [valMsgs, setValMsgs] = createSignal<string[]>([]);
  const [saving, setSaving] = createSignal(false);

  onMount(async () => {
    const coll = collection();
    const fs = coll ? (await api.listFlows(coll.path)) ?? [] : [];
    setFlows(fs);
    setLoading(false);
    if (fs.length) void select(fs[0]); // show the first flow's graph immediately
  });

  const refreshFlows = async () => {
    const coll = collection();
    const fs = coll ? (await api.listFlows(coll.path)) ?? [] : [];
    setFlows(fs);
    return fs;
  };

  const select = async (f: FlowInfo) => {
    if (running()) return;
    setSelected(f);
    setSteps([]);
    setExpanded(null);
    setError("");
    setEditing(false);
    setValMsgs([]);
    setDef(null);
    setDef(await api.readFlow(f.path));
  };

  const run = async () => {
    const coll = collection();
    const f = selected();
    if (!coll || !f || running()) return;
    setSteps([]);
    setExpanded(null);
    setError("");
    setRunning(true);
    const off = Events.On("flow:step", (e: any) =>
      setSteps((prev) => [...prev, e.data as FlowStep]),
    );
    try {
      const out = await api.runFlow(f.path, coll.path, activeEnv());
      if (out?.length && steps().length === 0) setSteps(out);
    } catch (e) {
      setError(String(e));
    } finally {
      off();
      setRunning(false);
    }
  };

  const startEdit = async () => {
    const f = selected();
    if (!f) return;
    setError("");
    setValMsgs([]);
    setRaw(await api.readFlowRaw(f.path));
    setEditing(true);
  };

  let valTimer: ReturnType<typeof setTimeout> | undefined;
  const onRawChange = (v: string) => {
    setRaw(v);
    clearTimeout(valTimer);
    valTimer = setTimeout(async () => setValMsgs((await api.validateFlow(v)) ?? []), 300);
  };

  const save = async () => {
    const f = selected();
    if (!f) return;
    setSaving(true);
    setError("");
    try {
      await api.saveFlowRaw(f.path, raw());
      setDef(await api.readFlow(f.path)); // refresh the graph from the saved file
      setEditing(false);
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  const cancelEdit = () => {
    setEditing(false);
    setValMsgs([]);
  };

  const del = async () => {
    const f = selected();
    if (!f) return;
    if (!(await confirmDialog(`Delete flow "${f.name}"?`, { danger: true, okLabel: "Delete" }))) return;
    setError("");
    try {
      await api.deleteFlow(f.path);
    } catch (e) {
      setError(String(e));
      return;
    }
    setSelected(null);
    setDef(null);
    setEditing(false);
    const fs = await refreshFlows();
    if (fs.length) void select(fs[0]);
  };

  const create = async () => {
    const coll = collection();
    if (!coll || running()) return;
    const name = await promptDialog("New flow name");
    if (!name) return;
    setError("");
    try {
      const path = await api.createFlow(coll.path, name);
      const fs = await refreshFlows();
      const f = fs.find((x) => x.path === path);
      if (f) {
        await select(f);
        await startEdit();
      }
    } catch (e) {
      setError(String(e));
    }
  };

  return (
    <div class="modal-backdrop" onClick={props.onClose}>
      <div class="modal modal-wide flow-panel" onClick={(e) => e.stopPropagation()}>
        <div class="modal-head">
          <span class="modal-title">Flows</span>
          <button class="icon-btn" title="Close" onClick={props.onClose}>
            <X size={ICON.md} />
          </button>
        </div>
        <div class="modal-body">
          <div class="flow-body">
            <Show when={loading()}>
              <div class="empty-hint">Loading…</div>
            </Show>
            <Show when={!loading() && flows().length === 0}>
              <div class="empty-hint">
                No flows yet. Click <strong>New flow</strong> below, or add a{" "}
                <code>*.flow.yaml</code> under <code>.senda/flows/</code>.
              </div>
            </Show>

            <Show when={!loading()}>
              <div class="flow-picker">
                <div class="flow-picker-label">Flows in this collection — pick one to preview &amp; run</div>
                <div class="flow-chips">
                  <For each={flows()}>
                    {(f) => (
                      <button
                        class={`flow-chip ${selected()?.path === f.path ? "active" : ""}`}
                        disabled={running()}
                        onClick={() => select(f)}
                      >
                        <Workflow size={ICON.sm} /> {f.name}
                      </button>
                    )}
                  </For>
                  <button class="flow-chip flow-chip-new" disabled={running()} onClick={create}>
                    <Plus size={ICON.sm} /> New flow
                  </button>
                </div>
              </div>
            </Show>

            <Show when={def()}>
              <div class="flow-detail">
                <div class="flow-detail-head">
                  <span class="flow-detail-name">{def()!.name}</span>
                  <Show when={!editing()}>
                    <button class="btn flow-edit" disabled={running()} onClick={startEdit} title="Edit YAML">
                      <Pencil size={ICON.sm} /> Edit
                    </button>
                    <button class="btn flow-delete" disabled={running()} onClick={del} title="Delete flow">
                      <Trash2 size={ICON.sm} /> Delete
                    </button>
                    <button class="btn flow-run" disabled={running()} onClick={run}>
                      <Play size={ICON.sm} /> {running() ? "Running…" : "Run flow"}
                    </button>
                  </Show>
                  <Show when={editing()}>
                    <button class="btn" onClick={cancelEdit}>Cancel</button>
                    <button class="btn flow-run" disabled={saving()} onClick={save}>
                      {saving() ? "Saving…" : "Save"}
                    </button>
                  </Show>
                </div>

                <Show
                  when={editing()}
                  fallback={
                    <div class="flow-graph">
                      <For each={ordered(def()!)}>
                        {([id, node]) => (
                          <div class="flow-node">
                            <div class="flow-node-head">
                              <span class="flow-node-id">{id}</span>
                              <Show when={def()!.start === id}>
                                <span class="flow-node-start">start</span>
                              </Show>
                              <span class="flow-node-type">{(node as any).type}</span>
                            </div>
                            <Show when={summary(node)}>
                              <div class="flow-node-summary">{summary(node)}</div>
                            </Show>
                            <For each={edges(node)}>
                              {(e) => (
                                <div class="flow-node-edge">
                                  <CornerDownRight size={ICON.xs} />
                                  <span class="flow-edge-label">{e.label}</span>
                                  {e.to}
                                </div>
                              )}
                            </For>
                          </div>
                        )}
                      </For>
                    </div>
                  }
                >
                  <div class="flow-editor">
                    <CodeEditor value={raw()} language="text" onChange={onRawChange} />
                    <Show when={valMsgs().length > 0}>
                      <div class="flow-val">
                        <For each={valMsgs()}>{(m) => <div class="flow-val-msg">{m}</div>}</For>
                      </div>
                    </Show>
                  </div>
                </Show>
              </div>
            </Show>

            <Show when={error()}>
              <div class="run-err flow-error">{error()}</div>
            </Show>

            <Show when={steps().length > 0}>
              <div class="flow-steps">
                <div class="flow-steps-label">Run — click a request to open its response</div>
                <For each={steps()}>
                  {(s, i) => {
                    const body = () => s.result?.response?.body;
                    const open = () => expanded() === i();
                    return (
                      <div class="flow-step-wrap">
                        <div
                          class={`flow-step ${body() ? "clickable" : ""}`}
                          onClick={() => body() && setExpanded(open() ? null : i())}
                        >
                          <Show when={body()} fallback={<span class="flow-step-caret-spacer" />}>
                            <Show when={open()} fallback={<ChevronRight size={ICON.xs} />}>
                              <ChevronDown size={ICON.xs} />
                            </Show>
                          </Show>
                          <span class="flow-step-id">{s.nodeId}</span>
                          <Show
                            when={s.result}
                            fallback={
                              <span class="flow-step-type">
                                {s.type}
                                <Show when={s.branch}> → {s.branch}</Show>
                              </span>
                            }
                          >
                            <span class={`status-badge ${statusClass(s.result!.status)}`}>
                              {s.result!.status}
                            </span>
                            <span class="flow-step-url" title={s.result!.url}>{s.result!.url}</span>
                          </Show>
                          <Show when={s.err}>
                            <span class="run-err" title={s.err}>err</span>
                          </Show>
                        </div>
                        <Show when={open() && body()}>
                          <pre class="flow-step-body">{body()}</pre>
                        </Show>
                      </div>
                    );
                  }}
                </For>
              </div>
            </Show>
          </div>
        </div>
        <div class="modal-foot">
          <button class="btn" onClick={props.onClose}>Close</button>
        </div>
      </div>
    </div>
  );
}
