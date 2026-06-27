// Flows modal: lists the *.flow.yaml graphs under .senda/flows/ and runs them.
// Authoring/editing is done in YAML (file-watch reloads); this panel only lists
// and executes. Steps stream in live via "flow:step" Wails events; request
// steps show a status badge, branch/setvar/etc. show a node marker.
import { createSignal, For, onMount, Show } from "solid-js";
import { Events } from "@wailsio/runtime";
import { X, Play } from "lucide-solid";
import { ICON } from "../lib/icons";
import { api } from "../lib/api";
import type { FlowInfo, FlowStep } from "../lib/api";
import { collection, activeEnv } from "../lib/store";
import { statusClass } from "../lib/factory";

export default function FlowPanel(props: { onClose: () => void }) {
  const [flows, setFlows] = createSignal<FlowInfo[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [running, setRunning] = createSignal<string | null>(null);
  const [steps, setSteps] = createSignal<FlowStep[]>([]);
  const [error, setError] = createSignal("");

  onMount(async () => {
    const coll = collection();
    if (coll) setFlows((await api.listFlows(coll.path)) ?? []);
    setLoading(false);
  });

  const run = async (f: FlowInfo) => {
    const coll = collection();
    if (!coll || running()) return;
    setSteps([]);
    setError("");
    setRunning(f.path);
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
      setRunning(null);
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
          <Show when={loading()}>
            <div class="empty-hint">Loading…</div>
          </Show>
          <Show when={!loading() && flows().length === 0}>
            <div class="empty-hint">
              No flows yet. Add a <code>*.flow.yaml</code> under <code>.senda/flows/</code>.
            </div>
          </Show>
          <div class="flow-list">
            <For each={flows()}>
              {(f) => (
                <div class="flow-row">
                  <button
                    class="btn ghost flow-run"
                    title="Run flow"
                    disabled={!!running()}
                    onClick={() => run(f)}
                  >
                    <Play size={ICON.sm} /> {f.name}
                  </button>
                </div>
              )}
            </For>
          </div>
          <Show when={error()}>
            <div class="run-err">{error()}</div>
          </Show>
          <Show when={steps().length > 0}>
            <div class="flow-steps">
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
        <div class="modal-foot">
          <button class="btn" onClick={props.onClose}>Close</button>
        </div>
      </div>
    </div>
  );
}
