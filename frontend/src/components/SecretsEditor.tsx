// Secrets editor dialog: manage *.secret.yaml overlays (gitignored) for the
// collection and each environment. Values are masked by default with a per-row
// reveal toggle. Writes through api.save{Collection,Environment}Secrets, which
// ensure the files are gitignored before writing.
import { createEffect, createSignal, For, Index, onMount, Show } from "solid-js";
import { X, Eye, EyeOff, Trash2, Lock, LockOpen } from "lucide-solid";
import { ICON } from "../lib/icons";
import { api } from "../lib/api";
import type { KV } from "../lib/api";
import { collection, environments } from "../lib/store";

type EncStatus = { enabled: boolean; keyAvailable: boolean; source: string };

// scope "" = collection-level overlay; otherwise the environment name.
export default function SecretsEditor(props: { onClose: () => void }) {
  const [scope, setScope] = createSignal("");
  const [rows, setRows] = createSignal<KV[]>([]);
  const [revealed, setRevealed] = createSignal<Set<number>>(new Set<number>());
  const [loading, setLoading] = createSignal(false);
  const [saving, setSaving] = createSignal(false);
  const [error, setError] = createSignal("");
  const [tick, setTick] = createSignal(0); // bump to force a scope reload

  // Collection-wide at-rest encryption (AES-256-GCM, key in OS keychain).
  const [enc, setEnc] = createSignal<EncStatus>({ enabled: false, keyAvailable: false, source: "" });
  const [encBusy, setEncBusy] = createSignal(false);
  const [keyInput, setKeyInput] = createSignal("");
  const [exported, setExported] = createSignal("");
  const loadEnc = async () => {
    const coll = collection();
    if (coll) setEnc((await api.encryptionStatus(coll.path)) as EncStatus);
  };
  onMount(loadEnc);

  const toggleEnc = async (on: boolean) => {
    const coll = collection();
    if (!coll) return;
    setEncBusy(true);
    setError("");
    setExported("");
    try {
      if (on) await api.enableEncryption(coll.path);
      else await api.disableEncryption(coll.path);
      // Only the on-disk FORMAT changed — the editor's values are unchanged, so
      // we don't reload rows (that would discard unsaved edits). Saves read the
      // meta flag from disk, which the backend already persisted. Just refresh
      // the lock status. Unlock-via-import is the one case that reloads rows.
      await loadEnc();
    } catch (e) {
      setError(String(e));
    } finally {
      setEncBusy(false);
    }
  };

  const doExport = async () => {
    const coll = collection();
    if (!coll) return;
    setError("");
    try {
      setExported(await api.exportKey(coll.path));
    } catch (e) {
      setError(String(e));
    }
  };

  const doImport = async () => {
    const coll = collection();
    if (!coll) return;
    setEncBusy(true);
    setError("");
    try {
      await api.importKey(coll.path, keyInput().trim());
      setKeyInput("");
      await loadEnc();
      setTick(tick() + 1);
    } catch (e) {
      setError(String(e));
    } finally {
      setEncBusy(false);
    }
  };

  // Load the active scope's secrets whenever it changes (runs on mount too).
  createEffect(() => {
    const sc = scope();
    tick(); // reload trigger
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
  const remove = (i: number) => {
    setRows(rows().filter((_, idx) => idx !== i));
    // reindex reveal flags: drop i, shift everything above it down one.
    setRevealed((prev) => new Set([...prev].filter((x) => x !== i).map((x) => (x > i ? x - 1 : x))));
  };
  const addRow = () =>
    setRows([...rows(), { key: "", value: "", enabled: true } as KV]);

  const toggleReveal = (i: number) => {
    const next = new Set(revealed());
    next.has(i) ? next.delete(i) : next.add(i);
    setRevealed(next);
  };

  // Locked = encryption on but the key isn't reachable here. Reads come back
  // empty (swallowed) — saving in that state would WIPE the still-encrypted file
  // (empty vars → file removed), so the editor is read-blocked until unlocked.
  const locked = () => enc().enabled && !enc().keyAvailable;

  const save = async () => {
    const coll = collection();
    if (!coll || locked()) return;
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

        <div class="secrets-enc">
          <label class="secrets-enc-toggle" title="AES-256-GCM at rest, key in your OS keychain">
            <input
              type="checkbox"
              checked={enc().enabled}
              disabled={encBusy()}
              onChange={(e) => toggleEnc(e.currentTarget.checked)}
            />
            {enc().enabled ? <Lock size={ICON.sm} /> : <LockOpen size={ICON.sm} />}
            Encrypt secret files at rest
          </label>
          <Show when={enc().enabled && enc().keyAvailable}>
            <span class="secrets-enc-status">unlocked via {enc().source}</span>
            <button class="btn ghost" onClick={doExport}>
              Export key…
            </button>
          </Show>
          <Show when={enc().enabled && !enc().keyAvailable}>
            <span class="secrets-enc-locked">
              <Lock size={ICON.sm} /> locked — key not on this machine
            </span>
            <input
              class="secret-val"
              type="password"
              placeholder="Paste exported key…"
              value={keyInput()}
              onInput={(e) => setKeyInput(e.currentTarget.value)}
            />
            <button class="btn ghost" disabled={encBusy() || !keyInput().trim()} onClick={doImport}>
              Import key
            </button>
          </Show>
        </div>
        <Show when={exported()}>
          <div class="secrets-enc-export">
            <code class="secrets-enc-key">{exported()}</code>
            <span class="secrets-hint">
              Set as <code>SENDA_SECRET_KEY</code> to decrypt headless, or import on another machine. Treat like a password.
            </span>
          </div>
        </Show>

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
            <Show when={locked()}>
              <div class="empty-hint secrets-locked-hint">
                <Lock size={ICON.sm} /> Secrets are encrypted and locked. Import the
                key above to view or edit them.
              </div>
            </Show>
            <Show
              when={!loading() && !locked()}
              fallback={<Show when={!locked()}><div class="empty-hint">Loading…</div></Show>}
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
          <button class="btn primary" disabled={saving() || loading() || locked()} onClick={save}>
            {saving() ? "Saving…" : "Save"}
          </button>
        </div>
      </div>
    </div>
  );
}
