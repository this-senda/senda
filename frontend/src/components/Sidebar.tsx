// Collection tree sidebar. Lets the user open a collection, browse/filter the
// folder/request tree, open a request into the editor, add/delete/rename, run
// a folder, import requests, and view history.
import { createEffect, createMemo, createSignal, For, onCleanup, Show } from "solid-js";
import { ChevronRight, ChevronsDownUp, ChevronsUpDown, Clock, Download, FilePlus, FileText, Folder, FolderPlus, MoreHorizontal, Pencil, Play, Plus, Search, Settings, ShieldCheck, X, Zap, Server } from "lucide-solid";
import { ICON } from "../lib/icons";
import { api } from "../lib/api";
import type { TreeNode } from "../lib/api";
import { activePath, activity, collection, openInTab, setShowMockPanel, setRunPanelTarget, setShowRunPanel } from "../lib/store";
import { refreshCollection } from "../lib/actions";
import { fmtAgo, nodeRecency } from "../lib/recency";
import { attachCtxDismiss } from "../lib/ctxMenu";
import { blankRequest } from "../lib/factory";
import { alertDialog, confirmDialog, promptDialog } from "../lib/dialog";
import CollectionSettings from "./CollectionSettings";
import FolderSettings from "./FolderSettings";
import ImportDialog from "./ImportDialog";
import HistoryPanel from "./HistoryPanel";
import SecurityScan from "./SecurityScan";

const refresh = refreshCollection;

// Expand/collapse-all command, broadcast to every TreeRow. `n` is a tick that
// changes on each press so rows re-apply `open` even after manual toggling;
// `open` is the target state. treeExpanded tracks the header button's icon.
const [expandCmd, setExpandCmd] = createSignal({ open: true, n: 0 });
const [treeExpanded, setTreeExpanded] = createSignal(true);
function toggleExpandAll() {
  const next = !treeExpanded();
  setTreeExpanded(next);
  setExpandCmd((c) => ({ open: next, n: c.n + 1 }));
}

// --- Pointer-based drag to move requests/folders ---
// WebKitGTK's HTML5 drag-and-drop doesn't reliably start a drag from a
// `draggable` element (it begins a text selection instead), so we track the
// drag with raw mouse events: source path while dragging, the folder path the
// cursor is over, and a flag to swallow the click that mouseup would fire.
const [dropPath, setDropPath] = createSignal<string | null>(null);
let dragJustEnded = false;

// trackDrag begins a drag from a row. It only activates past a small movement
// threshold so a plain click still opens the request. onDrop runs on release
// when the cursor is over a folder (folders carry data-drop-path).
function trackDrag(e: MouseEvent, src: string, onDrop: (src: string, dst: string) => void) {
  if (e.button !== 0) return;
  const sx = e.clientX;
  const sy = e.clientY;
  let active = false;
  const move = (ev: MouseEvent) => {
    if (!active) {
      if (Math.abs(ev.clientX - sx) + Math.abs(ev.clientY - sy) < 5) return;
      active = true;
      document.body.classList.add("dragging-node");
    }
    ev.preventDefault(); // suppress text selection while dragging
    const el = document.elementFromPoint(ev.clientX, ev.clientY) as HTMLElement | null;
    setDropPath(el?.closest<HTMLElement>("[data-drop-path]")?.dataset.dropPath ?? null);
  };
  const up = () => {
    window.removeEventListener("mousemove", move);
    window.removeEventListener("mouseup", up);
    document.body.classList.remove("dragging-node");
    const dst = dropPath();
    setDropPath(null);
    if (active) {
      dragJustEnded = true;
      setTimeout(() => (dragJustEnded = false), 0);
      if (dst) onDrop(src, dst);
    }
  };
  window.addEventListener("mousemove", move);
  window.addEventListener("mouseup", up);
}

// methodTag is the short sidebar badge for a request method (Bruno-style).
function methodTag(m: string | undefined): string {
  const up = (m || "GET").toUpperCase();
  if (up === "DELETE") return "DEL";
  if (up === "OPTIONS") return "OPT";
  return up;
}

// filterTree prunes the tree to nodes whose name matches q (case-insensitive);
// folders survive when any descendant matches.
function filterTree(node: TreeNode, q: string): TreeNode | null {
  if (!node.isDir) {
    return node.name.toLowerCase().includes(q) ? node : null;
  }
  const children = (node.children ?? [])
    .map((c) => (c ? filterTree(c, q) : null))
    .filter((c): c is TreeNode => c !== null);
  const selfMatch =
    node.name.toLowerCase().includes(q) ||
    (node.tags ?? []).some((t) => t.toLowerCase().includes(q));
  if (children.length === 0 && !selfMatch) return null;
  return { ...node, children };
}

export default function Sidebar() {
  const [showSettings, setShowSettings] = createSignal(false);
  const [showImport, setShowImport] = createSignal(false);
  const [showHistory, setShowHistory] = createSignal(false);
  const [scanTarget, setScanTarget] = createSignal<TreeNode | null>(null);
  const [search, setSearch] = createSignal("");

  // Collection-level actions menu (right-click the header row or click its ⋯).
  const [collMenu, setCollMenu] = createSignal<{ x: number; y: number } | null>(null);
  let closeCollCtx: (() => void) | null = null;
  const openCollMenu = (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setCollMenu({ x: e.clientX, y: e.clientY });
    closeCollCtx = attachCtxDismiss(() => setCollMenu(null));
  };
  onCleanup(() => closeCollCtx?.());

  const visibleTree = createMemo(() => {
    const tree = collection()?.tree;
    if (!tree) return null;
    const q = search().trim().toLowerCase();
    if (!q) return tree;
    return { ...tree, children: (tree.children ?? []).filter(Boolean).map((c) => filterTree(c!, q)).filter((c): c is TreeNode => c !== null) };
  });

  const newRequest = async () => {
    const coll = collection();
    if (!coll) return;
    const name = await promptDialog("Request name:", "new-request");
    if (!name) return;
    const path = `${coll.path}/${name}.yaml`;
    await api.saveRequest(path, blankRequest(name));
    await refresh(coll.path);
    openInTab(blankRequest(name), path);
  };

  const exportDocs = async () => {
    const coll = collection();
    if (!coll) return;
    const html = await api.exportDocsHtml(coll.path);
    await api.exportFile(`${coll.name || "api"}-docs.html`, html);
  };

  return (
    <aside class="sidebar">
      <div class="sidebar-head">
        <Show when={collection()}>
          <span class="sidebar-title" title={collection()!.path}>{collection()!.name}</span>
        </Show>
        <div class="sidebar-actions">
          <button class="icon-btn" title="New request" onClick={newRequest} disabled={!collection()}>
            <Plus size={ICON.xl} />
          </button>
          <button
            class="icon-btn"
            title={treeExpanded() ? "Collapse all" : "Expand all"}
            onClick={toggleExpandAll}
            disabled={!collection()}
          >
            <Show when={treeExpanded()} fallback={<ChevronsUpDown size={ICON.xl} />}>
              <ChevronsDownUp size={ICON.xl} />
            </Show>
          </button>
          <button
            class="icon-btn"
            title="History"
            onClick={() => setShowHistory(true)}
            disabled={!collection()}
          >
            <Clock size={ICON.xl} />
          </button>
          {/* Overflow: all collection-scoped actions live under one menu so the
              toolbar stays just the frequent tree verbs (see commit). */}
          <button
            class="icon-btn coll-overflow"
            title="Collection actions"
            onClick={openCollMenu}
            disabled={!collection()}
          >
            <MoreHorizontal size={ICON.xl} />
          </button>
        </div>
        <Show when={collMenu()}>
          <div
            class="ctx-menu"
            style={{ left: `${collMenu()!.x}px`, top: `${collMenu()!.y}px` }}
            onClick={(e) => e.stopPropagation()}
          >
            <button class="ctx-item" onClick={() => { closeCollCtx?.(); setShowImport(true); }}>
              <Download size={ICON.sm} /> Import collection
            </button>
            <button class="ctx-item" onClick={() => { closeCollCtx?.(); setShowMockPanel(true); }}>
              <Server size={ICON.sm} /> Mock server
            </button>
            <button class="ctx-item" onClick={() => { closeCollCtx?.(); exportDocs(); }}>
              <FileText size={ICON.sm} /> Export docs
            </button>
            <div class="ctx-sep" />
            <button class="ctx-item" onClick={() => { closeCollCtx?.(); setShowSettings(true); }}>
              <Settings size={ICON.sm} /> Collection settings
            </button>
          </div>
        </Show>
      </div>
      <Show when={showSettings()}>
        <CollectionSettings onClose={() => setShowSettings(false)} />
      </Show>
      <Show when={showImport()}>
        <ImportDialog onClose={() => setShowImport(false)} />
      </Show>
      <Show when={showHistory()}>
        <HistoryPanel onClose={() => setShowHistory(false)} />
      </Show>
      <Show when={scanTarget()}>
        <SecurityScan
          folderPath={scanTarget()!.path}
          folderName={scanTarget()!.name}
          onClose={() => setScanTarget(null)}
        />
      </Show>
      <Show when={collection()}>
        <div class="tree-search">
          <Search size={ICON.xs} />
          <input
            type="text"
            placeholder="Filter requests"
            value={search()}
            onInput={(e) => setSearch(e.currentTarget.value)}
          />
          <Show when={search()}>
            <button class="icon-btn tiny-static" title="Clear" onClick={() => setSearch("")}>
              <X size={ICON.xs} />
            </button>
          </Show>
        </div>
      </Show>
      <div
        class="tree"
        data-drop-path={collection()?.path}
      >
        <Show when={visibleTree()} fallback={<div class="empty-hint">Open a collection to begin.</div>}>
          <For each={visibleTree()!.children ?? []}>
            {(node) => (
              <TreeRow
                node={node!}
                depth={0}
                onRefresh={() => refresh(collection()!.path)}
                onRun={(n, tab) => { setRunPanelTarget({ folderPath: n.path, folderName: n.name, initialTab: tab }); setShowRunPanel(true); }}
                onScan={(n) => setScanTarget(n)}
              />
            )}
          </For>
        </Show>
      </div>
    </aside>
  );
}

// RecencyPill shows when a request (or any request under a folder) last ran:
// a colour-coded dot (fresh/recent/stale/error) plus a compact label. Folders
// whose descendants include a non-2xx run go red and show the failing count
// (e.g. "2✗") instead of the "time ago". Absent until something has run.
function RecencyPill(props: { node: TreeNode }) {
  const rec = createMemo(() => nodeRecency(props.node, activity()));
  const title = (r: ReturnType<typeof nodeRecency> & {}) => {
    const when = r.at ? `last run ${new Date(r.at).toLocaleString()}` : "no successful run";
    return r.failing > 0
      ? `${r.failing} of ${r.ran} request${r.ran === 1 ? "" : "s"} not 2xx · ${when}`
      : `Last run ${new Date(r.at).toLocaleString()}`;
  };
  return (
    <Show when={rec()}>
      {(r) => (
        <span class={`recency-pill ${r().cls}`} title={title(r())}>
          <span class="recency-dot" />
          <span class="recency-ago">{r().failing > 0 ? `${r().failing}✗` : fmtAgo(r().at)}</span>
        </span>
      )}
    </Show>
  );
}

function TreeRow(props: {
  node: TreeNode;
  depth: number;
  onRefresh: () => void;
  onRun: (n: TreeNode, tab: "run" | "load") => void;
  onScan: (n: TreeNode) => void;
}) {
  const [open, setOpen] = createSignal(true);
  // Re-apply the expand/collapse-all command whenever its tick changes.
  createEffect(() => {
    const cmd = expandCmd();
    if (cmd.n > 0) setOpen(cmd.open);
  });
  const [ctxMenu, setCtxMenu] = createSignal<{ x: number; y: number } | null>(null);
  let closeCtx: (() => void) | null = null;
  const pad = () => ({ "padding-left": `${8 + props.depth * 14}px` });

  const openReq = async () => {
    const req = await api.readRequest(props.node.path);
    openInTab(req, props.node.path);
  };

  const del = async (e: MouseEvent) => {
    e.stopPropagation();
    if (!(await confirmDialog(`Delete ${props.node.name}?`, { danger: true, okLabel: "Delete" }))) return;
    await api.deleteRequest(props.node.path);
    props.onRefresh();
  };

  const rename = async (e: MouseEvent) => {
    e.stopPropagation();
    const next = await promptDialog(`Rename ${props.node.name} to:`, props.node.name);
    if (!next || next === props.node.name) return;
    try {
      await api.renameNode(props.node.path, next);
      props.onRefresh();
    } catch (err) {
      await alertDialog("Rename failed: " + err);
    }
  };

  const run = (e: MouseEvent) => {
    e.stopPropagation();
    props.onRun(props.node, "run");
  };

  const [showSettings, setShowSettings] = createSignal(false);

  // Delete a folder (recursively) after confirming.
  const delFolder = async (e: MouseEvent) => {
    e.stopPropagation();
    if (!(await confirmDialog(`Delete folder "${props.node.name}" and everything inside it?`, { danger: true, okLabel: "Delete" }))) return;
    await api.deleteNode(props.node.path);
    props.onRefresh();
  };

  // Create a new request file directly inside this folder and open it.
  const newReqHere = async (e: MouseEvent) => {
    e.stopPropagation();
    const name = await promptDialog("Request name:", "new-request");
    if (!name) return;
    const path = `${props.node.path}/${name}.yaml`;
    await api.saveRequest(path, blankRequest(name));
    props.onRefresh();
    openInTab(blankRequest(name), path);
  };

  // Export docs for just this folder's subtree (backend scopes by subPath).
  const exportDocs = async () => {
    const coll = collection();
    if (!coll) return;
    const html = await api.exportDocsHtml(coll.path, props.node.path);
    await api.exportFile(`${props.node.name}-docs.html`, html);
  };

  // Create a new sub-folder inside this folder.
  const newFolderHere = async (e: MouseEvent) => {
    e.stopPropagation();
    const name = await promptDialog("Folder name:", "new-folder");
    if (!name) return;
    try {
      await api.createFolder(`${props.node.path}/${name}`);
      props.onRefresh();
    } catch (err) {
      await alertDialog("Create folder failed: " + err);
    }
  };

  const openCtxMenu = (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setCtxMenu({ x: e.clientX, y: e.clientY });
    closeCtx = attachCtxDismiss(() => setCtxMenu(null));
  };
  onCleanup(() => closeCtx?.());

  // --- drag & drop move (pointer-based, see trackDrag) ---
  const onMoveDrop = async (src: string, dst: string) => {
    if (src === dst || dst.startsWith(src + "/")) return;
    try {
      await api.moveNode(src, dst);
      props.onRefresh();
    } catch (err) {
      await alertDialog("Move failed: " + err);
    }
  };
  const startDrag = (e: MouseEvent) => trackDrag(e, props.node.path, onMoveDrop);

  return (
    <div class="tree-node">
      <Show
        when={props.node.isDir}
        fallback={
          <div
            class="tree-leaf"
            classList={{ active: activePath() === props.node.path }}
            style={pad()}
            onMouseDown={startDrag}
            onClick={() => { if (!dragJustEnded) openReq(); }}
            onContextMenu={openCtxMenu}
          >
            <span class={`leaf-method method-${(props.node.method || "get").toLowerCase()}`}>
              {methodTag(props.node.method)}
            </span>
            <span class="leaf-name">{props.node.name}</span>
            <RecencyPill node={props.node} />
            <button class="icon-btn tiny" onClick={rename} title="Rename">
              <Pencil size={ICON.xxl} />
            </button>
            <button class="icon-btn tiny" onClick={del} title="Delete">
              <X size={ICON.xxl} />
            </button>
            <Show when={ctxMenu()}>
              <div
                class="ctx-menu"
                style={{ left: `${ctxMenu()!.x}px`, top: `${ctxMenu()!.y}px` }}
                onClick={(e) => e.stopPropagation()}
              >
                <button class="ctx-item" onClick={() => { closeCtx?.(); openReq(); }}>
                  Open
                </button>
                <button class="ctx-item" onClick={() => { closeCtx?.(); props.onRun(props.node, "load"); }}>
                  <Zap size={ICON.sm} /> Load test
                </button>
                <button class="ctx-item" onClick={() => { closeCtx?.(); props.onScan(props.node); }}>
                  <ShieldCheck size={ICON.sm} /> Security scan
                </button>
                <button class="ctx-item" onClick={(e) => { closeCtx?.(); rename(e); }}>
                  <Pencil size={ICON.sm} /> Rename
                </button>
                <button class="ctx-item ctx-item-danger" onClick={(e) => { closeCtx?.(); del(e); }}>
                  <X size={ICON.sm} /> Delete
                </button>
              </div>
            </Show>
          </div>
        }
      >
        <div
          class="tree-folder"
          classList={{ "drop-hover": dropPath() === props.node.path }}
          data-drop-path={props.node.path}
          style={pad()}
          onMouseDown={startDrag}
          onClick={() => { if (!dragJustEnded) setOpen(!open()); }}
          onContextMenu={openCtxMenu}
        >
          <span class="caret" classList={{ open: open() }}>
            <ChevronRight size={ICON.lg} />
          </span>
          <span class="folder-icon" style={props.node.color ? { color: props.node.color } : undefined}>
            <Folder size={ICON.lg} />
          </span>
          <span class="folder-name">{props.node.name}</span>
          <Show when={(props.node.tags ?? []).length}>
            <span class="folder-tags">
              <For each={props.node.tags!}>{(t) => <span class="folder-tag">{t}</span>}</For>
            </span>
          </Show>
          <RecencyPill node={props.node} />
          <button class="icon-btn tiny" onClick={run} title="Run folder">
            <Play size={ICON.xxl} />
          </button>
          <button class="icon-btn tiny" onClick={rename} title="Rename">
            <Pencil size={ICON.xxl} />
          </button>
        </div>

        {/* Right-click context menu */}
        <Show when={ctxMenu()}>
          <div
            class="ctx-menu"
            style={{ left: `${ctxMenu()!.x}px`, top: `${ctxMenu()!.y}px` }}
            onClick={(e) => e.stopPropagation()}
          >
            <button
              class="ctx-item"
              onClick={() => { closeCtx?.(); props.onRun(props.node, "run"); }}
            >
              <Play size={ICON.sm} /> Run folder
            </button>
            <button
              class="ctx-item"
              onClick={() => { closeCtx?.(); props.onRun(props.node, "load"); }}
            >
              <Zap size={ICON.sm} /> Load test
            </button>
            <button
              class="ctx-item"
              onClick={() => { closeCtx?.(); props.onScan(props.node); }}
            >
              <ShieldCheck size={ICON.sm} /> Security scan
            </button>
            <div class="ctx-sep" />
            <button class="ctx-item" onClick={(e) => { closeCtx?.(); newReqHere(e); }}>
              <FilePlus size={ICON.sm} /> New request
            </button>
            <button class="ctx-item" onClick={(e) => { closeCtx?.(); newFolderHere(e); }}>
              <FolderPlus size={ICON.sm} /> New folder
            </button>
            <button class="ctx-item" onClick={(e) => { closeCtx?.(); rename(e); }}>
              <Pencil size={ICON.sm} /> Rename
            </button>
            <button class="ctx-item" onClick={() => { closeCtx?.(); exportDocs(); }}>
              <FileText size={ICON.sm} /> Export docs
            </button>
            <button class="ctx-item" onClick={() => { closeCtx?.(); setShowSettings(true); }}>
              <Settings size={ICON.sm} /> Folder settings
            </button>
            <button class="ctx-item ctx-item-danger" onClick={(e) => { closeCtx?.(); delFolder(e); }}>
              <X size={ICON.sm} /> Delete folder
            </button>
          </div>
        </Show>

        <Show when={showSettings()}>
          <FolderSettings
            path={props.node.path}
            name={props.node.name}
            onClose={() => setShowSettings(false)}
            onSaved={props.onRefresh}
          />
        </Show>

        <Show when={open()}>
          <For each={props.node.children ?? []}>
            {(child) => (
              <TreeRow
                node={child!}
                depth={props.depth + 1}
                onRefresh={props.onRefresh}
                onRun={props.onRun}
                onScan={props.onScan}
              />
            )}
          </For>
        </Show>
      </Show>
    </div>
  );
}
