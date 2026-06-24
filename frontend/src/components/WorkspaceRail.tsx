// Workspace rail in the titlebar (Insomnia-style): each open collection is a
// rounded icon box showing a chosen emoji or a 2-letter monogram. Click to
// switch, right-click to set its icon or close it, + to open another. Still
// single active root — switching swaps the open collection.
import { createSignal, For, Show } from "solid-js";
import { ChevronDown, FileArchive, FolderPlus, Plus } from "lucide-solid";
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

export default function WorkspaceRail() {
  const [addOpen, setAddOpen] = createSignal(false);
  const [iconFor, setIconFor] = createSignal<string | null>(null); // path whose picker is open

  const pick = async (path: string) => {
    setAddOpen(false);
    if (path === collection()?.path) return;
    try {
      await switchCollection(path);
    } catch {
      /* moved/deleted — switchCollection already pruned recents */
    }
  };

  const openNew = async () => {
    setAddOpen(false);
    await openCollectionDialog();
  };
  const openZip = async () => {
    setAddOpen(false);
    await openZipCollectionDialog();
  };

  const unpinnedRecents = () => recents().filter((r) => !pinned().some((p) => p.path === r.path));

  return (
    <div class="ws-rail">
      <For each={pinned()}>
        {(p) => (
          <div class="ws-box-wrap">
            <button
              class="ws-box"
              classList={{ "ws-box-active": p.path === collection()?.path }}
              style={{ "--ws-hue": boxHue(p.path) }}
              title={p.name}
              onClick={() => pick(p.path)}
              onContextMenu={(e) => {
                e.preventDefault();
                setIconFor(iconFor() === p.path ? null : p.path);
              }}
            >
              <Show when={collectionIcon(p.path)} fallback={<span class="ws-box-mono">{monogram(p.name)}</span>}>
                <span class="ws-box-emoji">{collectionIcon(p.path)}</span>
              </Show>
            </button>

            <Show when={iconFor() === p.path}>
              <div class="menu-backdrop" onClick={() => setIconFor(null)} />
              <div class="ws-picker">
                <div class="ws-picker-grid">
                  <For each={EMOJI}>
                    {(e) => (
                      <button
                        class="ws-picker-cell"
                        onClick={() => {
                          setCollectionIcon(p.path, e);
                          setIconFor(null);
                        }}
                      >
                        {e}
                      </button>
                    )}
                  </For>
                </div>
                <div class="coll-menu-sep" />
                <button
                  class="ws-picker-row"
                  onClick={() => {
                    setCollectionIcon(p.path, "");
                    setIconFor(null);
                  }}
                >
                  Use letters
                </button>
                <button
                  class="ws-picker-row ws-picker-danger"
                  onClick={() => {
                    unpin(p.path);
                    setIconFor(null);
                  }}
                >
                  Close workspace
                </button>
              </div>
            </Show>
          </div>
        )}
      </For>

      <div class="ws-box-wrap">
        <button class="ws-box ws-add-box" title="Open collection" onClick={() => setAddOpen(!addOpen())}>
          <Plus size={ICON.md} />
          <ChevronDown size={ICON.xs} class="ws-add-caret" />
        </button>
        <Show when={addOpen()}>
          <div class="menu-backdrop" onClick={() => setAddOpen(false)} />
          <div class="coll-menu">
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
      </div>
    </div>
  );
}
