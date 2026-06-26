// Flows modal: lists the *.flow.yaml graphs under .senda/flows/, shows a
// selected flow's structure (nodes + edges) so you can see what it does before
// running, then runs it and streams the steps. Authoring/editing is done in YAML
// (file-watch reloads); this panel only inspects and executes.
import { createSignal, For, onMount, Show } from "solid-js";
import { Events } from "@wailsio/runtime";
import { X, Play, CornerDownRight, Workflow } from "lucide-solid";
import { ICON } from "../lib/icons";
import { api } from "../lib/api";
import type { FlowInfo, FlowStep, Flow } from "../lib/api";
import { collection, activeEnv } from "../lib/store";
import { statusClass } from "../lib/factory";

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

export default function FlowPanel(props: { onClose: () => void }) {
  const [flows, setFlows] = createSignal<FlowInfo[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [selected, setSelected] = createSignal<FlowInfo | null>(null);
  const [def, setDef] = createSignal<Flow | null>(null);
  const [running, setRunning] = createSignal(false);
  const [steps, setSteps] = createSignal<FlowStep[]>([]);
  const [error, setError] = createSignal("");

  onMount(async () => {
    const coll = collection();
    const fs = coll ? (await api.listFlows(coll.path)) ?? [] : [];
    setFlows(fs);
    setLoading(false);
    if (fs.length) void select(fs[0]); // show the first flow's graph immediately
  });

  const select = async (f: FlowInfo) => {
    if (running()) return;
    setSelected(f);
    setSteps([]);
    setError("");
    setDef(null);
    setDef(await api.readFlow(f.path));
  };

  const run = async () => {
    const coll = collection();
    const f = selected();
    if (!coll || !f || running()) return;
    setSteps([]);
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
                No flows yet. Add a <code>*.flow.yaml</code> under <code>.senda/flows/</code>.
              </div>
            </Show>

            <Show when={flows().length > 0}>
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
                </div>
              </div>
            </Show>

            <Show when={def()}>
              <div class="flow-detail">
                <div class="flow-detail-head">
                  <span class="flow-detail-name">{def()!.name}</span>
                  <button class="btn flow-run" disabled={running()} onClick={run}>
                    <Play size={ICON.sm} /> {running() ? "Running…" : "Run flow"}
                  </button>
                </div>
                <div class="flow-graph">
                  <For each={Object.entries(def()!.nodes ?? {})}>
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
              </div>
            </Show>

            <Show when={error()}>
              <div class="run-err flow-error">{error()}</div>
            </Show>

            <Show when={steps().length > 0}>
              <div class="flow-steps">
                <div class="flow-steps-label">Run</div>
                <For each={steps()}>
                  {(s) => (
                    <div class="flow-step">
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
                  )}
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
