// History modal: lists recently sent requests for the open collection, newest
// first. Reads the append-only log the backend writes on every send.
import { createSignal, For, onMount, Show } from "solid-js";
import { X } from "lucide-solid";
import { ICON } from "../lib/icons";
import { api } from "../lib/api";
import type { HistoryEntry } from "../lib/api";
import { collection } from "../lib/store";
import { statusClass } from "../lib/factory";
import { confirmDialog } from "../lib/dialog";

export default function HistoryPanel(props: { onClose: () => void }) {
  const [entries, setEntries] = createSignal<HistoryEntry[]>([]);
  const [loading, setLoading] = createSignal(true);

  const load = async () => {
    const coll = collection();
    if (!coll) {
      setLoading(false);
      return;
    }
    setEntries((await api.listHistory(coll.path, 200)) ?? []);
    setLoading(false);
  };

  onMount(load);

  const clear = async () => {
    const coll = collection();
    if (!coll || !(await confirmDialog("Clear request history?", { danger: true, okLabel: "Clear" }))) return;
    await api.clearHistory(coll.path);
    setEntries([]);
  };

  const when = (at: string) => {
    const d = new Date(at);
    return isNaN(d.getTime()) ? at : d.toLocaleString();
  };

  return (
    <div class="modal-backdrop" onClick={props.onClose}>
      <div class="modal modal-wide" onClick={(e) => e.stopPropagation()}>
        <div class="modal-head">
          <span class="modal-title">History</span>
          <button class="icon-btn" title="Close" onClick={props.onClose}>
            <X size={ICON.md} />
          </button>
        </div>
        <div class="modal-body">
          <Show when={loading()}>
            <div class="empty-hint">Loading…</div>
          </Show>
          <Show when={!loading() && entries().length === 0}>
            <div class="empty-hint">No history yet.</div>
          </Show>
          <div class="hist-list">
            <For each={entries()}>
              {(e) => (
                <div class="hist-row">
                  <span class="hist-method">{e.method}</span>
                  <Show
                    when={!e.error}
                    fallback={<span class="run-err" title={e.error}>err</span>}
                  >
                    <span class={`status-badge ${statusClass(e.status)}`}>{e.status}</span>
                  </Show>
                  <span class="hist-url" title={e.url}>{e.url}</span>
                  <span class="hist-when">{when(e.at)}</span>
                </div>
              )}
            </For>
          </div>
        </div>
        <div class="modal-foot">
          <button class="btn ghost" onClick={clear} disabled={entries().length === 0}>
            Clear
          </button>
          <button class="btn" onClick={props.onClose}>
            Close
          </button>
        </div>
      </div>
    </div>
  );
}
