// Per-folder (and collection-root) settings modal. Edits the folder's
// senda.meta.yaml metadata: an organisational color + tags + description, plus
// folder-level default vars and auth that requests inside the folder inherit
// (request -> folder(s) -> collection root). Loads via ReadFolderMeta and
// saves via SaveCollection (the same senda.meta.yaml schema is used for the root
// and every sub-folder).
import { createSignal, For, onMount, Show } from "solid-js";
import { X } from "lucide-solid";
import { ICON } from "../lib/icons";
import { api } from "../lib/api";
import type { Auth, Collection, KV } from "../lib/api";
import { blankAuth } from "../lib/factory";
import AuthEditor from "./AuthEditor";
import KVEditor from "./KVEditor";

// FOLDER_COLORS is the preset palette shown in the color picker. "" means no
// color (inherit the default folder tint).
export const FOLDER_COLORS = [
  "#e5484d", // red
  "#f5a623", // amber
  "#f2d600", // yellow
  "#46a758", // green
  "#3e9fdf", // blue
  "#8e6fe0", // purple
  "#e055a0", // pink
  "#8a8f98", // gray
];

export default function FolderSettings(props: {
  path: string;
  name: string;
  onClose: () => void;
  onSaved?: () => void;
}) {
  const [color, setColor] = createSignal("");
  const [tags, setTags] = createSignal<string[]>([]);
  const [tagDraft, setTagDraft] = createSignal("");
  const [description, setDescription] = createSignal("");
  const [vars, setVars] = createSignal<KV[]>([]);
  const [auth, setAuth] = createSignal<Auth>(blankAuth());
  const [name, setName] = createSignal(props.name);
  const [loading, setLoading] = createSignal(true);
  const [saving, setSaving] = createSignal(false);
  const [error, setError] = createSignal("");

  onMount(async () => {
    try {
      const meta = await api.readFolderMeta(props.path);
      setName(meta.name || props.name);
      setColor(meta.color ?? "");
      setTags(meta.tags ?? []);
      setDescription(meta.description ?? "");
      setVars(meta.vars ?? []);
      setAuth((meta.auth && meta.auth.type ? meta.auth : blankAuth()) as Auth);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  });

  const addTag = (raw: string) => {
    const t = raw.trim();
    if (!t || tags().includes(t)) return;
    setTags([...tags(), t]);
    setTagDraft("");
  };
  const removeTag = (t: string) => setTags(tags().filter((x) => x !== t));

  const save = async () => {
    setSaving(true);
    setError("");
    try {
      const meta: Collection = {
        name: name(),
        path: props.path,
        color: color(),
        tags: tags(),
        description: description(),
        vars: vars(),
        auth: auth(),
        proxy: "", // root-only; folders never carry proxy/tls
        tls: { certFile: "", keyFile: "", caFile: "", insecure: false },
        tree: null as any,
      };
      await api.saveCollection(meta);
      props.onSaved?.();
      props.onClose();
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div class="modal-backdrop" onClick={props.onClose}>
      <div class="modal" onClick={(e) => e.stopPropagation()}>
        <div class="modal-head">
          <span class="modal-title">{props.name} — Folder settings</span>
          <button class="icon-btn" title="Close" onClick={props.onClose}>
            <X size={ICON.md} />
          </button>
        </div>
        <Show when={!loading()} fallback={<div class="modal-body">Loading…</div>}>
          <div class="modal-body folder-settings-body">
            <div class="modal-section-label">Color</div>
            <div class="folder-color-picker">
              <button
                class="folder-swatch none"
                classList={{ selected: color() === "" }}
                title="No color"
                onClick={() => setColor("")}
              >
                <X size={ICON.sm} />
              </button>
              <For each={FOLDER_COLORS}>
                {(c) => (
                  <button
                    class="folder-swatch"
                    classList={{ selected: color() === c }}
                    style={{ background: c }}
                    title={c}
                    onClick={() => setColor(c)}
                  />
                )}
              </For>
            </div>

            <div class="modal-section-label">Tags</div>
            <div class="tag-input">
              <For each={tags()}>
                {(t) => (
                  <span class="tag-chip">
                    {t}
                    <button class="tag-remove" title="Remove tag" onClick={() => removeTag(t)}>
                      <X size={ICON.xs} />
                    </button>
                  </span>
                )}
              </For>
              <input
                type="text"
                placeholder="Add tag…"
                value={tagDraft()}
                onInput={(e) => setTagDraft(e.currentTarget.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === ",") {
                    e.preventDefault();
                    addTag(tagDraft());
                  } else if (e.key === "Backspace" && tagDraft() === "" && tags().length) {
                    removeTag(tags()[tags().length - 1]);
                  }
                }}
                onBlur={() => addTag(tagDraft())}
              />
            </div>

            <div class="modal-section-label">Description</div>
            <textarea
              class="folder-desc"
              rows={3}
              placeholder="What lives in this folder…"
              value={description()}
              onInput={(e) => setDescription(e.currentTarget.value)}
            />

            <div class="modal-section-label">Folder variables</div>
            <p class="modal-hint">Override collection vars for requests in this folder.</p>
            <KVEditor rows={vars()} onChange={setVars} keyPlaceholder="name" valuePlaceholder="value" />

            <div class="modal-section-label">Default authentication</div>
            <p class="modal-hint">Requests set to “Inherit” use this (falling back to the collection).</p>
            <AuthEditor auth={auth()} onChange={setAuth} allowInherit={true} />
          </div>
        </Show>
        <Show when={error()}>
          <div class="modal-error">{error()}</div>
        </Show>
        <div class="modal-foot">
          <button class="btn ghost" onClick={props.onClose}>
            Cancel
          </button>
          <button class="btn" onClick={save} disabled={saving() || loading()}>
            {saving() ? "Saving…" : "Save"}
          </button>
        </div>
      </div>
    </div>
  );
}
