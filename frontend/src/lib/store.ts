// Central app state: open collection, open request tabs, environments, response.
// Kept deliberately small — request/response bodies are held transiently,
// never in one giant reactive blob (architecture §7).
//
// Tabs model: the singletons below (`request`, `response`, `dirty`, `sending`,
// `activePath`) are the LIVE editor model for whichever tab is active. The
// `tabs` array holds a per-tab snapshot; switching tabs saves the live model
// into the outgoing tab and reconciles the incoming tab back into the live one.
import { createSignal } from "solid-js";
import { createStore, reconcile, unwrap } from "solid-js/store";
import { Events } from "@wailsio/runtime";
import { api } from "./api";
import type { Collection, Environment, Request, Response, LoadTick, LoadSummary, Activity } from "./api";
import { blankRequest } from "./factory";

export const [collection, setCollection] = createSignal<Collection | null>(null);
export const [environments, setEnvironments] = createSignal<Environment[]>([]);
export const [activeEnv, setActiveEnv] = createSignal<string>("");

// --- sidebar recency pills -------------------------------------------------
// Maps a request's file path to its last-run Activity (timestamp + status),
// derived from the collection history. Folders roll this up from descendants.
// Refreshed after every send / folder run / load test and on collection open.
export const [activity, setActivity] = createSignal<Record<string, Activity>>({});

// refreshActivity reloads the last-run map for a collection. Best-effort: a
// failure just leaves the existing pills in place.
export async function refreshActivity(collPath: string) {
  if (!collPath) return;
  try {
    const map = await api.collectionActivity(collPath);
    setActivity((map as Record<string, Activity>) ?? {});
  } catch {
    // ignore — pills are non-critical
  }
}

// --- background process state (mock server, load test) --------------------
export const [mockServerAddr, setMockServerAddr] = createSignal("");   // "" = stopped
export const [showMockPanel, setShowMockPanel] = createSignal(false);
export const [showSpecPanel, setShowSpecPanel] = createSignal(false);

// Response pane collapsed state, persisted across launches.
export const [respCollapsed, setRespCollapsed] = createSignal(
  localStorage.getItem("senda.respCollapsed") === "1"
);
export function toggleResp(force?: boolean) {
  const next = force ?? !respCollapsed();
  setRespCollapsed(next);
  localStorage.setItem("senda.respCollapsed", next ? "1" : "0");
}

export type RunPanelTarget = { folderPath: string; folderName: string; initialTab: "run" | "load" };
export const [runPanelTarget, setRunPanelTarget] = createSignal<RunPanelTarget | null>(null);
export const [showRunPanel, setShowRunPanel] = createSignal(false);

// --- load test runtime ----------------------------------------------------
// The load test runs in the Go backend and streams ticks over a Wails event.
// Its lifecycle lives HERE (not in the LoadTest component) so closing the run
// modal does not unmount/cancel it — the test keeps running in the background,
// the status bar stays visible, and reopening the modal rebinds to live state.
export const [loadTestRunning, setLoadTestRunning] = createSignal(false);

export type LoadConfig = {
  vus: number;
  mode: "duration" | "iterations";
  duration: number;
  iterations: number;
  rampUp: number;
};
const blankLoadConfig = (): LoadConfig => ({
  vus: 10,
  mode: "duration",
  duration: 30,
  iterations: 100,
  rampUp: 0,
});
export const [loadConfig, setLoadConfig] = createSignal<LoadConfig>(blankLoadConfig());
export const [loadHistory, setLoadHistory] = createSignal<LoadTick[]>([]);
export const [loadSummary, setLoadSummary] = createSignal<LoadSummary | null>(null);
export const [loadError, setLoadError] = createSignal("");

let loadCall: ReturnType<typeof api.runLoad> | undefined;
let loadOffTick: (() => void) | undefined;

// startLoadTest kicks off a background load test against folderPath using the
// current loadConfig(). No-op if one is already running.
export async function startLoadTest(folderPath: string) {
  if (loadTestRunning()) return;
  loadOffTick?.();
  setLoadSummary(null);
  setLoadError("");
  setLoadHistory([]);
  setLoadTestRunning(true);
  loadOffTick = Events.On("load:tick", (e: any) => {
    setLoadHistory((prev) => [...prev, e.data as LoadTick]);
  });
  const cfg = loadConfig();
  try {
    loadCall = api.runLoad(folderPath, collection()?.path ?? "", activeEnv(), {
      vus: cfg.vus,
      duration: cfg.mode === "duration" ? cfg.duration : 0,
      iterations: cfg.mode === "iterations" ? cfg.iterations : 0,
      rampUp: cfg.rampUp,
    });
    const result = await loadCall;
    if (result) setLoadSummary(result);
  } catch (e: any) {
    if (!String(e).includes("cancel")) setLoadError(String(e));
  } finally {
    loadOffTick?.();
    loadOffTick = undefined;
    loadCall = undefined;
    setLoadTestRunning(false);
    void refreshActivity(collection()?.path ?? "");
  }
}

// stopLoadTest cancels the in-flight backend run (the finally above resets
// state). Safe to call when nothing is running.
export function stopLoadTest() {
  loadCall?.cancel();
}

// --- live editor model (the active tab) ----------------------------------
export const [request, setRequest] = createStore<Request>(blankRequest());
export const [activePath, setActivePath] = createSignal<string>("");
export const [dirty, setDirty] = createSignal(false);
export const [response, setResponse] = createSignal<Response | null>(null);
export const [sending, setSending] = createSignal(false);

// Bumped by Ctrl+L to focus the URL bar from anywhere (UrlField, in the
// titlebar, listens for the change). Counter, not boolean: every press fires.
export const [urlFocusTick, setUrlFocusTick] = createSignal(0);
export const focusUrl = () => setUrlFocusTick((n) => n + 1);

// Active sub-tab of the request editor (params/headers/… plus ws/sse). Lifted
// to the store so the URL bar — which now lives in the titlebar, outside
// RequestEditor — can still route ws/sse sends to the right tab.
export type ReqSubTab = "params" | "headers" | "auth" | "body" | "tests" | "script" | "docs" | "ws" | "sse";
export const [reqTab, setReqTab] = createSignal<ReqSubTab>("params");

// --- open request tabs ----------------------------------------------------
export type Tab = {
  id: string;
  path: string; // "" for an unsaved scratch tab
  title: string;
  model: Request; // snapshot; live edits live in the singletons above
  response: Response | null;
  dirty: boolean;
  sending: boolean;
};

function newId(): string {
  return crypto.randomUUID();
}

// Deep clone via JSON — Request is plain serializable data (round-trips through
// YAML). structuredClone chokes on the wails model class instances.
function clone<T>(v: T): T {
  return JSON.parse(JSON.stringify(v));
}

function basename(path: string): string {
  const seg = path.split("/").pop() ?? path;
  return seg.replace(/\.ya?ml$/i, "");
}

function tabTitle(req: Request, path: string): string {
  return path ? basename(path) : req.name || "untitled";
}

function makeTab(req: Request, path: string): Tab {
  return {
    id: newId(),
    path,
    title: tabTitle(req, path),
    model: clone(unwrap(req)),
    response: null,
    dirty: false,
    sending: false,
  };
}

const firstTab = makeTab(blankRequest(), "");
export const [tabs, setTabs] = createStore<Tab[]>([firstTab]);
export const [activeTabId, setActiveTabId] = createSignal<string>(firstTab.id);

function activeIndex(): number {
  return tabs.findIndex((t) => t.id === activeTabId());
}

// Write the live singletons back into the active tab's snapshot.
function syncActiveTab() {
  const i = activeIndex();
  if (i < 0) return;
  setTabs(i, {
    model: clone(unwrap(request)),
    response: response(),
    dirty: dirty(),
    sending: sending(),
    title: tabTitle(request, tabs[i].path),
  });
}

// Load a tab's snapshot into the live singletons.
function loadTab(t: Tab) {
  setRequest(reconcile(clone(t.model)));
  setResponse(t.response);
  setDirty(t.dirty);
  setSending(t.sending);
  setActivePath(t.path);
  setActiveTabId(t.id);
}

// resetToBlankTab replaces every open tab with a single fresh scratch tab and
// makes it active. The terminal state for "closed the last/only tabs".
function resetToBlankTab() {
  const fresh = makeTab(blankRequest(), "");
  setTabs([fresh]);
  loadTab(fresh);
  persistTabs();
}

export function switchTab(id: string) {
  if (id === activeTabId()) return;
  const target = tabs.find((t) => t.id === id);
  if (!target) return;
  syncActiveTab();
  loadTab(target);
  persistTabs();
}

// Open a request in a tab. Focus-if-open: a saved request already open is
// re-focused rather than duplicated.
export function openInTab(req: Request, path: string) {
  if (path) {
    const existing = tabs.find((t) => t.path === path);
    if (existing) {
      switchTab(existing.id);
      return;
    }
  }
  syncActiveTab();
  const t = makeTab(req, path);
  // Reuse the active tab if it's a clean, unsaved scratch — avoids leaving a
  // stray blank tab behind when the user opens their first real request.
  const i = activeIndex();
  const cur = i >= 0 ? tabs[i] : undefined;
  if (path && cur && !cur.path && !cur.dirty) {
    setTabs(i, t);
  } else {
    setTabs(tabs.length, t);
  }
  loadTab(t);
  persistTabs();
}

export function newTab() {
  openInTab(blankRequest(), "");
}

// cycleTab activates the neighbouring tab (dir +1 = right, -1 = left, wraps).
export function cycleTab(dir: number) {
  if (tabs.length < 2) return;
  const i = activeIndex();
  if (i < 0) return;
  switchTab(tabs[(i + dir + tabs.length) % tabs.length].id);
}

export function closeTab(id: string) {
  const i = tabs.findIndex((t) => t.id === id);
  if (i < 0) return;
  const wasActive = id === activeTabId();
  const next = tabs.filter((t) => t.id !== id);
  if (next.length === 0) {
    resetToBlankTab();
    return;
  }
  setTabs(next);
  if (wasActive) {
    const neighbour = next[Math.min(i, next.length - 1)];
    loadTab(neighbour);
  }
  persistTabs();
}

export function closeOtherTabs(id: string) {
  const keep = tabs.find((t) => t.id === id);
  if (!keep) return;
  setTabs([keep]);
  loadTab(keep);
  persistTabs();
}

export function closeTabsToRight(id: string) {
  const i = tabs.findIndex((t) => t.id === id);
  if (i < 0) return;
  const next = tabs.slice(0, i + 1);
  setTabs(next);
  if (!next.some((t) => t.id === activeTabId())) loadTab(next[next.length - 1]);
  persistTabs();
}

export function closeTabsToLeft(id: string) {
  const i = tabs.findIndex((t) => t.id === id);
  if (i <= 0) return;
  const next = tabs.slice(i);
  setTabs(next);
  if (!next.some((t) => t.id === activeTabId())) loadTab(next[0]);
  persistTabs();
}

// closeSavedTabs closes every clean, saved tab; unsaved or dirty tabs stay.
export function closeSavedTabs() {
  syncActiveTab();
  const next = tabs.filter((t) => !t.path || t.dirty);
  if (next.length === tabs.length) return;
  if (next.length === 0) {
    resetToBlankTab();
    return;
  }
  const activeStays = next.some((t) => t.id === activeTabId());
  setTabs(next);
  if (!activeStays) loadTab(next[next.length - 1]);
  persistTabs();
}

export function closeAllTabs() {
  resetToBlankTab();
}

// cloneTab duplicates a tab's request into a new unsaved scratch tab.
export function cloneTab(id: string) {
  const src = tabs.find((t) => t.id === id);
  if (!src) return;
  syncActiveTab();
  const model = id === activeTabId() ? unwrap(request) : src.model;
  const copy = clone(model) as Request;
  copy.name = (copy.name || "untitled") + " copy";
  const t = makeTab(copy, "");
  setTabs(tabs.length, t);
  loadTab(t);
  persistTabs();
}

// revertPath returns the disk path a tab can be reverted to, or "" when the
// tab is unsaved (nothing to revert to). The caller reads the file and passes
// the fresh request to revertTabTo.
export function revertPath(id: string): string {
  const t = tabs.find((tb) => tb.id === id);
  return t?.path ?? "";
}

// revertTabTo replaces a tab's model with a freshly-read request and clears
// its dirty flag.
export function revertTabTo(id: string, req: Request) {
  const i = tabs.findIndex((t) => t.id === id);
  if (i < 0) return;
  setTabs(i, { model: clone(req), dirty: false, title: tabTitle(req, tabs[i].path) });
  if (id === activeTabId()) {
    setRequest(reconcile(clone(req)));
    setDirty(false);
  }
  persistTabs();
}

// Mark the active tab clean after a successful save (keeps title in sync too).
export function markActiveSaved(path: string) {
  setActivePath(path);
  setDirty(false);
  const i = activeIndex();
  if (i >= 0) {
    setTabs(i, { dirty: false, path, title: tabTitle(request, path) });
  }
  persistTabs();
}

// loadRequest replaces the editor model wholesale — kept for back-compat,
// now routes through the tab layer.
export function loadRequest(req: Request, path: string) {
  openInTab(req, path);
}

const LAST_COLLECTION = "senda.lastCollection";
const RECENT_COLLECTIONS = "senda.recentCollections";
const ACTIVE_ENV = "senda.activeEnv";
const OPEN_TABS = "senda.openTabs";

const RECENT_MAX = 12;

// A workspace entry: enough to render the switcher without reopening the
// collection from disk.
export type RecentCollection = { name: string; path: string };

export function rememberCollection(path: string) {
  localStorage.setItem(LAST_COLLECTION, path);
}
export function lastCollection(): string | null {
  return localStorage.getItem(LAST_COLLECTION);
}

// recentCollections returns the known workspaces, most-recently-opened first.
export function recentCollections(): RecentCollection[] {
  try {
    const raw = localStorage.getItem(RECENT_COLLECTIONS);
    const list = raw ? JSON.parse(raw) : [];
    return Array.isArray(list) ? list.filter((e) => e?.path) : [];
  } catch {
    return [];
  }
}

// recentCollectionsSignal mirrors the persisted list so the switcher updates
// live as collections are opened or pruned.
const [recents, setRecents] = createSignal<RecentCollection[]>(recentCollections());
export { recents };

function writeRecents(list: RecentCollection[]) {
  localStorage.setItem(RECENT_COLLECTIONS, JSON.stringify(list));
  setRecents(list);
}

// rememberRecent moves a collection to the front of the recents list (deduped
// by path) and refreshes its display name.
export function rememberRecent(name: string, path: string) {
  if (!path) return;
  const next = [{ name: name || path, path }, ...recentCollections().filter((e) => e.path !== path)];
  writeRecents(next.slice(0, RECENT_MAX));
}

// forgetRecent drops a collection from the list (e.g. it moved or was deleted).
export function forgetRecent(path: string) {
  writeRecents(recentCollections().filter((e) => e.path !== path));
}
// Pinned collections are a user-curated subset shown as pills in the titlebar.
// Still single active root: clicking a pin swaps the open collection (same as
// recents). Stored separately so pins survive recents pruning.
const PINNED_COLLECTIONS = "senda.pinnedCollections";

export function pinnedCollections(): RecentCollection[] {
  try {
    const raw = localStorage.getItem(PINNED_COLLECTIONS);
    const list = raw ? JSON.parse(raw) : [];
    return Array.isArray(list) ? list.filter((e) => e?.path) : [];
  } catch {
    return [];
  }
}

const [pinned, setPinned] = createSignal<RecentCollection[]>(pinnedCollections());
export { pinned };

function writePinned(list: RecentCollection[]) {
  localStorage.setItem(PINNED_COLLECTIONS, JSON.stringify(list));
  setPinned(list);
}

export function isPinned(path: string): boolean {
  return pinned().some((e) => e.path === path);
}

// ensurePinned adds a collection to the pill bar if not already there (called
// when a collection is opened — your open workspaces just appear as tabs).
export function ensurePinned(name: string, path: string) {
  if (!path || isPinned(path)) return;
  writePinned([...pinned(), { name: name || path, path }]);
}

// unpin removes a collection's box from the rail.
export function unpin(path: string) {
  writePinned(pinned().filter((e) => e.path !== path));
}

// Per-collection icon (an emoji) chosen by the user; absent → monogram fallback.
const COLLECTION_ICONS = "senda.collectionIcons";

function readIcons(): Record<string, string> {
  try {
    const raw = localStorage.getItem(COLLECTION_ICONS);
    const m = raw ? JSON.parse(raw) : {};
    return m && typeof m === "object" ? m : {};
  } catch {
    return {};
  }
}

const [icons, setIcons] = createSignal<Record<string, string>>(readIcons());

export function collectionIcon(path: string): string | undefined {
  return icons()[path];
}

// setCollectionIcon stores an emoji for a path (empty string clears it).
export function setCollectionIcon(path: string, icon: string) {
  if (!path) return;
  const next = { ...icons() };
  if (icon) next[path] = icon;
  else delete next[path];
  localStorage.setItem(COLLECTION_ICONS, JSON.stringify(next));
  setIcons(next);
}

export function rememberEnv(name: string) {
  localStorage.setItem(ACTIVE_ENV, name);
}
export function lastEnv(): string {
  return localStorage.getItem(ACTIVE_ENV) ?? "";
}

// Persist only saved tabs (those with a path) plus the active path.
export function persistTabs() {
  syncActiveTab();
  const paths = tabs.filter((t) => t.path).map((t) => t.path);
  const active = tabs.find((t) => t.id === activeTabId())?.path ?? "";
  localStorage.setItem(OPEN_TABS, JSON.stringify({ paths, active }));
}

export function savedTabs(): { paths: string[]; active: string } {
  try {
    const raw = localStorage.getItem(OPEN_TABS);
    if (!raw) return { paths: [], active: "" };
    const parsed = JSON.parse(raw);
    return {
      paths: Array.isArray(parsed.paths) ? parsed.paths : [],
      active: typeof parsed.active === "string" ? parsed.active : "",
    };
  } catch {
    return { paths: [], active: "" };
  }
}
