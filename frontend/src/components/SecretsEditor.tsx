// Secrets editor dialog: manage *.secret.yaml overlays (gitignored) for the
// collection and each environment. Values are masked by default with a per-row
// reveal toggle. Writes through api.save{Collection,Environment}Secrets, which
// ensure the files are gitignored before writing.
import { createEffect, createSignal, For, Index, Show } from "solid-js";
import { X, Eye, EyeOff, Trash2 } from "lucide-solid";
import { ICON } from "../lib/icons";
import { api } from "../lib/api";
import type { KV } from "../lib/api";
import { collection, environments } from "../lib/store";

// scope "" = collection-level overlay; otherwise the environment name.
export default function SecretsEditor(props: { onClose: () => void }) {
  const [scope, setScope] = createSignal("");
  const [rows, setRows] = createSignal<KV[]>([]);
  const [revealed, setRevealed] = createSignal<Set<number>>(new Set<number>());
  const [loading, setLoading] = createSignal(false);
  const [saving, setSaving] = createSignal(false);
  const [error, setError] = createSignal("");

  // Load the active scope's secrets whenever it changes (runs on mount too).
  createEffect(() => {
    const sc = scope();
    const coll = collection();
    if (!coll) return;
    setLoading(true);
    setError("");
    setRevealed(new Set<number>());
    (sc ? api.readEnvironmentSecrets(coll.path, sc) : api.readCollectionSecrets(coll.path))
      .then((r) => setRows((r ?? []).map((kv: KV) => ({ ...kv }))))
      .catch((e) => setError(String(e)))
      .finally(() => setLoading(false));
  });

  const patch = (i: number, p: Partial<KV>) =>
    setRows(rows().map((r, idx) => (idx === i ? { ...r, ...p } : r)));
  const remove = (i: number) => setRows(rows().filter((_, idx) => idx !== i));
  const addRow = () =>
    setRows([...rows(), { key: "", value: "", enabled: true } as KV]);

  const toggleReveal = (i: number) => {
    const next = new Set(revealed());
    next.has(i) ? next.delete(i) : next.add(i);
    setRevealed(next);
  };

  const save = async () => {
    const coll = collection();
    if (!coll) return;
    setSaving(true);
    setError("");
    try {
      const clean = rows().filter((r) => r.key.trim() !== "");
      if (scope()) await api.saveEnvironmentSecrets(coll.path, scope(), clean);
      else await api.saveCollectionSecrets(coll.path, clean);
      props.onClose();
    } catch (e) {
      setError(String(e));
      setSaving(false);
    }
  };

  return (
    <div class="modal-backdrop" onClick={props.onClose}>
      <div class="modal modal-wide secrets-modal" onClick={(e) => e.stopPropagation()}>
        <div class="modal-head">
          <span class="modal-title">Secrets</span>
          <button class="icon-btn" title="Close" onClick={props.onClose}>
            <X size={ICON.md} />
          </button>
        </div>
        <div class="modal-body env-editor-body">
          <div class="env-list">
            <button
              class="env-item"
              classList={{ active: scope() === "" }}
              onClick={() => setScope("")}
            >
              Collection
            </button>
            <For each={environments()}>
              {(env) => (
                <button
                  class="env-item"
                  classList={{ active: env.name === scope() }}
                  onClick={() => setScope(env.name)}
                >
                  {env.name}
                </button>
              )}
            </For>
          </div>
          <div class="env-vars">
            <div class="secrets-hint">
              Stored in a gitignored <code>*.secret.yaml</code> overlay. Values
              shadow plain variables at send time.
            </div>
            <Show
              when={!loading()}
              fallback={<div class="empty-hint">Loading…</div>}
            >
              <Index
                each={rows()}
                fallback={<div class="empty-hint">No secrets yet.</div>}
              >
                {(row, i) => (
                  <div class="secret-row">
                    <input
                      type="checkbox"
                      checked={row().enabled}
                      title="Enabled"
                      onChange={(e) => patch(i, { enabled: e.currentTarget.checked })}
                    />
                    <input
                      class="secret-key"
                      placeholder="name"
                      value={row().key}
                      onInput={(e) => patch(i, { key: e.currentTarget.value })}
                    />
                    <input
                      class="secret-val"
                      type={revealed().has(i) ? "text" : "password"}
                      placeholder="value"
                      value={row().value}
                      onInput={(e) => patch(i, { value: e.currentTarget.value })}
                    />
                    <button
                      class="icon-btn secret-reveal"
                      title={revealed().has(i) ? "Hide" : "Reveal"}
                      onClick={() => toggleReveal(i)}
                    >
                      {revealed().has(i) ? <EyeOff size={ICON.sm} /> : <Eye size={ICON.sm} />}
                    </button>
                    <button class="icon-btn" title="Remove" onClick={() => remove(i)}>
                      <Trash2 size={ICON.sm} />
                    </button>
                  </div>
                )}
              </Index>
              <button class="add-row" onClick={addRow}>
                + Add secret
              </button>
            </Show>
          </div>
        </div>
        <Show when={error()}>
          <div class="modal-error">{error()}</div>
        </Show>
        <div class="modal-foot">
          <button class="btn" onClick={props.onClose}>
            Close
          </button>
          <button class="btn primary" disabled={saving() || loading()} onClick={save}>
            {saving() ? "Saving…" : "Save"}
          </button>
        </div>
      </div>
    </div>
  );
}
