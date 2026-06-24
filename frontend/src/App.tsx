// Top-level layout: title bar + three panes (sidebar | request | response).
import { createSignal, onCleanup, onMount, Show } from "solid-js";
import Sidebar from "./components/Sidebar";
import TabBar from "./components/TabBar";
import RequestEditor from "./components/RequestEditor";
import ResponseViewer from "./components/ResponseViewer";
import EnvSwitcher from "./components/EnvSwitcher";
import WorkspaceRail from "./components/WorkspaceRail";
import FpsMeter from "./components/FpsMeter";
import CommandPalette from "./components/CommandPalette";
import ThemeSettings from "./components/ThemeSettings";
import MockServerPanel from "./components/MockServerPanel";
import RunResults from "./components/RunResults";
import StatusBar from "./components/StatusBar";
import { Events } from "@wailsio/runtime";
import { Palette } from "lucide-solid";
import { ICON } from "./lib/icons";
import { api } from "./lib/api";
import { initTheme } from "./lib/theme";
import { refreshCollection, saveActive, sendActive } from "./lib/actions";
import { tabCycleDir } from "./lib/keymap";
import {
  activeTabId,
  closeTab,
  collection,
  cycleTab,
  lastCollection,
  lastEnv,
  newTab,
  openInTab,
  savedTabs,
  setActiveEnv,
  showMockPanel,
  setShowMockPanel,
  runPanelTarget,
  showRunPanel,
  setShowRunPanel,
  switchTab,
  tabs,
} from "./lib/store";

// Clamp helper for splitter drags.
const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v));

export default function App() {
  // F8 toggles the FPS overlay; choice persists across launches.
  const [showFps, setShowFps] = createSignal(localStorage.getItem("senda.fps") === "1");
  const [showPalette, setShowPalette] = createSignal(false);
  const [showTheme, setShowTheme] = createSignal(false);

  // Apply the persisted theme and follow OS light/dark changes.
  onMount(() => onCleanup(initTheme()));

  // Pane sizes: sidebar width in px, then the request pane's share of the
  // remaining space. Both persist across launches.
  const [sideW, setSideW] = createSignal(
    clamp(Number(localStorage.getItem("senda.sideW")) || 250, 160, 480)
  );
  const [split, setSplit] = createSignal(
    clamp(Number(localStorage.getItem("senda.split")) || 0.5, 0.2, 0.8)
  );

  const dragSplitter = (which: "side" | "mid") => (e: MouseEvent) => {
    e.preventDefault();
    document.body.classList.add("dragging-splitter");
    const move = (ev: MouseEvent) => {
      if (which === "side") {
        setSideW(clamp(ev.clientX, 160, 480));
      } else {
        const rest = window.innerWidth - sideW();
        if (rest > 0) setSplit(clamp((ev.clientX - sideW()) / rest, 0.2, 0.8));
      }
    };
    const up = () => {
      document.body.classList.remove("dragging-splitter");
      window.removeEventListener("mousemove", move);
      window.removeEventListener("mouseup", up);
      localStorage.setItem("senda.sideW", String(sideW()));
      localStorage.setItem("senda.split", String(split()));
    };
    window.addEventListener("mousemove", move);
    window.addEventListener("mouseup", up);
  };
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "F8") {
      const next = !showFps();
      setShowFps(next);
      localStorage.setItem("senda.fps", next ? "1" : "0");
      return;
    }
    const mod = e.ctrlKey || e.metaKey;
    if (!mod) return;
    // Ctrl+Tab cycles even while the palette is open; everything else below
    // is suppressed so palette typing can't trigger app actions. tabCycleDir
    // keys off e.code (see keymap.ts — Shift+Tab's ISO_Left_Tab keysym).
    // stopImmediatePropagation + capture-phase listening (see onMount) beat
    // the native focus-traversal default.
    const dir = tabCycleDir(e);
    if (dir) {
      e.preventDefault();
      e.stopImmediatePropagation();
      cycleTab(dir);
      return;
    }
    if (showPalette()) return;
    switch (e.key.toLowerCase()) {
      case "t":
      case "n":
        e.preventDefault();
        newTab();
        break;
      case "w":
        e.preventDefault();
        closeTab(activeTabId());
        break;
      case "s":
        e.preventDefault();
        void saveActive();
        break;
      case "enter":
        e.preventDefault();
        void sendActive();
        break;
      case "k":
      case "p":
        e.preventDefault();
        setShowPalette(true);
        break;
      case "pagedown":
        e.preventDefault();
        cycleTab(1);
        break;
      case "pageup":
        e.preventDefault();
        cycleTab(-1);
        break;
    }
  };
  onMount(() => window.addEventListener("keydown", onKey, { capture: true }));
  onCleanup(() => window.removeEventListener("keydown", onKey, { capture: true }));

  // Swallow drops that miss a registered dropzone. Without this the webview
  // performs its default drop action and navigates the whole window to a
  // wails:// URL — e.g. dragging a request onto a non-folder row. Real
  // dropzones stopPropagation, so this only catches unhandled drops.
  const eatDrag = (e: DragEvent) => e.preventDefault();
  onMount(() => {
    window.addEventListener("dragover", eatDrag);
    window.addEventListener("drop", eatDrag);
  });
  onCleanup(() => {
    window.removeEventListener("dragover", eatDrag);
    window.removeEventListener("drop", eatDrag);
  });

  // External file changes (git pull, $EDITOR) refresh the open collection.
  onMount(() => {
    const off = Events.On("collection:changed", () => {
      const c = collection();
      if (c) void refreshCollection(c.path);
    });
    onCleanup(off);
  });

  onMount(async () => {
    setActiveEnv(lastEnv());
    const path = lastCollection();
    if (path) {
      try {
        await refreshCollection(path);
      } catch {
        /* collection moved or deleted; ignore */
      }
    }
    // Reopen previously open request tabs (skip any that moved/deleted).
    const { paths, active } = savedTabs();
    for (const p of paths) {
      try {
        openInTab(await api.readRequest(p), p);
      } catch {
        /* file gone; skip */
      }
    }
    if (active) {
      const t = tabs.find((tb) => tb.path === active);
      if (t) switchTab(t.id);
    }
  });

  return (
    <div class="app">
      <Show when={showFps()}>
        <FpsMeter />
      </Show>
      <Show when={showPalette()}>
        <CommandPalette
          onClose={() => setShowPalette(false)}
          onOpenTheme={() => setShowTheme(true)}
        />
      </Show>
      <Show when={showTheme()}>
        <ThemeSettings onClose={() => setShowTheme(false)} />
      </Show>
      <header class="titlebar">
        <div class="titlebar-left">
          <span class="brand">Senda</span>
          <WorkspaceRail />
        </div>
        <div class="titlebar-actions">
          <EnvSwitcher />
          <button
            class="icon-btn"
            title="Appearance"
            onClick={() => setShowTheme(true)}
          >
            <Palette size={ICON.xxl} />
          </button>
        </div>
      </header>
      <div
        class="panes"
        style={{
          "grid-template-columns": `${sideW()}px 5px ${split()}fr 5px ${1 - split()}fr`,
        }}
      >
        <Sidebar />
        <div class="splitter" onMouseDown={dragSplitter("side")} />
        <main class="center">
          <TabBar />
          <RequestEditor />
        </main>
        <div class="splitter" onMouseDown={dragSplitter("mid")} />
        <section class="right">
          <ResponseViewer />
        </section>
      </div>
      <StatusBar />
      <Show when={showMockPanel()}>
        <MockServerPanel onClose={() => setShowMockPanel(false)} />
      </Show>
      <Show when={showRunPanel() && runPanelTarget()}>
        <RunResults
          folderPath={runPanelTarget()!.folderPath}
          folderName={runPanelTarget()!.folderName}
          initialTab={runPanelTarget()!.initialTab}
          onClose={() => setShowRunPanel(false)}
        />
      </Show>
    </div>
  );
}
