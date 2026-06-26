// Workspace switcher in the titlebar (Slack-style): a single pill showing the
// active collection's avatar + name + caret. Click opens a dropdown to switch
// between open workspaces or open another; right-click a row (or the pill) to
// set its emoji icon or close it. Single active root — switching swaps it.
import { createSignal, For, Show } from "solid-js";
import { Check, ChevronDown, FileArchive, FolderPlus, Plus } from "lucide-solid";
import { ICON } from "../lib/icons";
import { collection, collectionIcon, pinned, recents, setCollectionIcon, unpin } from "../lib/store";
import { openCollectionDialog, openZipCollectionDialog, switchCollection } from "../lib/actions";

// monogram derives up to two uppercase letters from a collection name:
// "train-travel-api" -> "TT", "petstore" -> "PE".
export function monogram(name: string): string {
  const words = (name || "").split(/[\s\-_./]+/).filter(Boolean);
  if (words.length >= 2) return (words[0][0] + words[1][0]).toUpperCase();
  return (name || "?").slice(0, 2).toUpperCase();
}

// boxHue maps a path to a stable hue so each workspace keeps its color.
export function boxHue(path: string): number {
  let h = 0;
  for (let i = 0; i < path.length; i++) h = (h * 31 + path.charCodeAt(i)) % 360;
  return h;
}

const EMOJI = ["🚆","📦","🌐","🔐","🧪","🛠️","📚","⚡","🎯","🚀","💳","🗂️","🐙","🔥","🌙","🦋"];

// Avatar box: chosen emoji, else a colored 2-letter monogram.
function Avatar(props: { path: string; name: string }) {
  return (
    <span class="ws-avatar" style={{ "--ws-hue": boxHue(props.path) }}>
      <Show when={collectionIcon(props.path)} fallback={<span class="ws-avatar-mono">{monogram(props.name)}</span>}>
        <span class="ws-avatar-emoji">{collectionIcon(props.path)}</span>
      </Show>
    </span>
  );
}

export default function WorkspaceRail() {
  const [menuOpen, setMenuOpen] = createSignal(false);
  const [iconFor, setIconFor] = createSignal<string | null>(null); // path whose picker is open

  const pick = async (path: string) => {
    setMenuOpen(false);
    if (path === collection()?.path) return;
    try {
      await switchCollection(path);
    } catch {
      /* moved/deleted — switchCollection already pruned recents */
    }
  };

  const openPicker = (path: string) => {
    setMenuOpen(false);
    setIconFor(path);
  };

  const openNew = async () => {
    setMenuOpen(false);
    await openCollectionDialog();
  };
  const openZip = async () => {
    setMenuOpen(false);
    await openZipCollectionDialog();
  };

  const unpinnedRecents = () => recents().filter((r) => !pinned().some((p) => p.path === r.path));

  return (
    <div class="ws-switch-wrap">
      <button
        class="ws-switch"
        title="Switch workspace"
        onClick={() => setMenuOpen(!menuOpen())}
        onContextMenu={(e) => {
          const c = collection();
          if (!c) return;
          e.preventDefault();
          openPicker(c.path);
        }}
      >
        <Show
          when={collection()}
          fallback={
            <>
              <Plus size={ICON.md} />
              <span class="ws-switch-name">Open collection</span>
            </>
          }
        >
          <Avatar path={collection()!.path} name={collection()!.name} />
          <span class="ws-switch-name">{collection()!.name}</span>
        </Show>
        <ChevronDown size={ICON.xs} class="ws-switch-caret" />
      </button>

      <Show when={menuOpen()}>
        <div class="menu-backdrop" onClick={() => setMenuOpen(false)} />
        <div class="coll-menu">
          <Show when={pinned().length > 0}>
            <div class="coll-menu-label">Workspaces</div>
            <For each={pinned()}>
              {(p) => (
                <button
                  class="coll-menu-item ws-menu-item"
                  classList={{ "ws-menu-active": p.path === collection()?.path }}
                  onClick={() => pick(p.path)}
                  onContextMenu={(e) => { e.preventDefault(); openPicker(p.path); }}
                  title={p.path}
                >
                  <Avatar path={p.path} name={p.name} />
                  <span class="coll-menu-item-name">{p.name}</span>
                  <Show when={p.path === collection()?.path}>
                    <Check size={ICON.sm} class="ws-menu-check" />
                  </Show>
                </button>
              )}
            </For>
            <div class="coll-menu-sep" />
          </Show>
          <Show when={unpinnedRecents().length > 0}>
            <For each={unpinnedRecents()}>
              {(r) => (
                <button class="coll-menu-item" onClick={() => pick(r.path)} title={r.path}>
                  <span class="coll-menu-item-name">{r.name}</span>
                  <span class="coll-menu-item-path">{r.path}</span>
                </button>
              )}
            </For>
            <div class="coll-menu-sep" />
          </Show>
          <button class="coll-menu-item coll-menu-open" onClick={openNew}>
            <FolderPlus size={ICON.xs} />
            <span class="coll-menu-item-name">Open collection…</span>
          </button>
          <button class="coll-menu-item coll-menu-open" onClick={openZip}>
            <FileArchive size={ICON.xs} />
            <span class="coll-menu-item-name">Open .zip collection…</span>
          </button>
        </div>
      </Show>

      <Show when={iconFor()}>
        {(path) => (
          <>
            <div class="menu-backdrop" onClick={() => setIconFor(null)} />
            <div class="ws-picker">
              <div class="ws-picker-grid">
                <For each={EMOJI}>
                  {(e) => (
                    <button
                      class="ws-picker-cell"
                      onClick={() => { setCollectionIcon(path(), e); setIconFor(null); }}
                    >
                      {e}
                    </button>
                  )}
                </For>
              </div>
              <div class="coll-menu-sep" />
              <button
                class="ws-picker-row"
                onClick={() => { setCollectionIcon(path(), ""); setIconFor(null); }}
              >
                Use letters
              </button>
              <button
                class="ws-picker-row ws-picker-danger"
                onClick={() => { unpin(path()); setIconFor(null); }}
              >
                Close workspace
              </button>
            </div>
          </>
        )}
      </Show>
    </div>
  );
}
