import { createSignal, For, onCleanup, onMount, Show } from "solid-js";
import { X, RotateCcw, Plus } from "lucide-solid";
import { Events } from "@wailsio/runtime";
import { ICON } from "../lib/icons";
import { api, type MockRouteInfo, type MockLogEntry, type MockInfo } from "../lib/api";
import { collection, mockServerAddr, setMockServerAddr } from "../lib/store";

// Friendly labels for bundled presets; falls back to the raw name.
const PRESET_LABELS: Record<string, string> = {
  oauth: "OAuth2 / OIDC login",
};
const presetLabel = (name: string) => PRESET_LABELS[name] ?? name;

export default function MockServerPanel(props: { onClose: () => void }) {
  const [addr, setAddr] = createSignal(":8787");
  const [routes, setRoutes] = createSignal<MockRouteInfo[]>([]);
  const [preview, setPreview] = createSignal<MockRouteInfo[]>([]);
  const [log, setLog] = createSignal<MockLogEntry[]>([]);
  const [info, setInfo] = createSignal<MockInfo | null>(null);
  const [error, setError] = createSignal("");
  const [presets, setPresets] = createSignal<string[]>([]);
  const [preset, setPreset] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [notice, setNotice] = createSignal("");

  const running = () => mockServerAddr() !== "";
  // What the Routes section shows: live routes when running, else the on-disk
  // preview loaded without starting a server.
  const shownRoutes = () => (running() ? routes() : preview());

  let offLog: (() => void) | undefined;
  let offRoutes: (() => void) | undefined;

  const refreshRoutes = async () => setRoutes((await api.mockServerRoutes()) ?? []);
  const refreshInfo = async () => setInfo((await api.mockServerInfo()) ?? null);
  const refreshLog = async () => setLog((await api.mockServerLog()) ?? []);

  // Load the routes that exist in mocks/ on disk, without starting the server,
  // so the panel previews what's there before Start.
  const refreshPreview = async () => {
    const coll = collection();
    if (!coll) return;
    setPreview((await api.previewMockRoutes(coll.path)) ?? []);
  };

  // Write a bundled preset's YAML into the collection's mocks/, then refresh the
  // preview so the new routes appear immediately.
  const addPreset = async () => {
    const coll = collection();
    if (!coll) { setError("No collection open"); return; }
    const name = preset() || presets()[0];
    if (!name) return;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const written = (await api.scaffoldMockPreset(coll.path, name)) ?? [];
      await refreshPreview();
      setNotice(
        written.length > 0
          ? `Added ${written.length} file${written.length === 1 ? "" : "s"} for ${presetLabel(name)}.`
          : `${presetLabel(name)} already present in mocks/.`,
      );
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  // Wire up the live event listeners. Idempotent: detaches any prior pair first
  // so re-subscribing on remount can't leak duplicate handlers.
  const subscribe = () => {
    offLog?.();
    offRoutes?.();
    offLog = Events.On("mock:log", (e: any) => {
      // Wails v3 wraps the payload in `.data`.
      setLog((prev) => [...prev.slice(-199), e.data as MockLogEntry]);
    });
    offRoutes = Events.On("mock:routes", () => { void refreshRoutes(); void refreshInfo(); });
  };

  // The server lives in Go; this panel's routes/log/info are local signals that
  // reset whenever the panel closes. If the server is still running on (re)open,
  // re-attach listeners and pull current state so we don't show an empty panel.
  onMount(() => {
    void api.mockPresets().then((p) => setPresets(p ?? []));
    if (!running()) {
      void refreshPreview();
      return;
    }
    subscribe();
    void refreshRoutes();
    void refreshInfo();
    void refreshLog();
  });

  const start = async () => {
    setError("");
    try {
      const coll = collection();
      if (!coll) { setError("No collection open"); return; }
      subscribe();
      const bound = await api.startMockServer(coll.path, addr());
      setMockServerAddr(bound);
      await refreshRoutes();
      await refreshInfo();
    } catch (e) {
      setError(String(e));
      offLog?.();
      offRoutes?.();
    }
  };

  const stop = async () => {
    try { await api.stopMockServer(); } catch {}
    offLog?.();
    offRoutes?.();
    offLog = offRoutes = undefined;
    setMockServerAddr("");
    setLog([]);
    setInfo(null);
  };

  const changeScenario = async (name: string) => {
    await api.setMockScenario(name);
    await refreshInfo();
  };

  const resetState = async () => { await api.resetMockState(); };

  // Live-switch which response a single endpoint returns, without editing files.
  const setRouteResponse = async (method: string, path: string, status: number) => {
    await api.setMockRouteResponse(method, path, status);
    await refreshRoutes();
  };

  onCleanup(() => { offLog?.(); offRoutes?.(); });

  return (
    <div class="modal-backdrop" onClick={props.onClose}>
      <div class="modal modal-wide" onClick={(e) => e.stopPropagation()}>
        <div class="modal-head">
          <span class="modal-title">Mock Server</span>
          <button class="icon-btn" onClick={props.onClose}><X size={ICON.sm} /></button>
        </div>

        <div class="mock-config">
          <input
            class="mock-addr-input"
            value={addr()}
            onInput={(e) => setAddr(e.currentTarget.value)}
            placeholder=":8787"
            disabled={running()}
          />
          <Show when={!running()} fallback={
            <button class="btn mock-stop-btn" onClick={stop}>Stop</button>
          }>
            <button class="btn send-btn" onClick={start}>Start</button>
          </Show>
        </div>

        <Show when={error()}>
          <div class="modal-error">{error()}</div>
        </Show>
        <Show when={notice()}>
          <div class="mock-notice">{notice()}</div>
        </Show>

        <Show when={running() && info()}>
          {(i) => (
            <div class="mock-status">
              <span class="mock-live">● Running</span>
              <span class="mock-bound">{i().addr}</span>
              <Show when={i().cors}><span class="mock-badge">CORS</span></Show>
              <Show when={i().proxy}><span class="mock-badge">proxy → {i().proxy}</span></Show>
            </div>
          )}
        </Show>

        <Show when={running() && info()}>
          {(i) => (
            <div class="mock-toolbar">
              <Show when={i().scenarios.length > 0}>
                <label class="mock-scenario">
                  Scenario
                  <select
                    class="body-type-select"
                    value={i().scenario}
                    onChange={(e) => void changeScenario(e.currentTarget.value)}
                  >
                    <option value="">default</option>
                    <For each={i().scenarios}>{(s) => <option value={s}>{s}</option>}</For>
                  </select>
                </label>
              </Show>
              <button class="mini-btn" onClick={resetState} title="Restore resource records to their seeds">
                <RotateCcw size={ICON.xs} /> Reset state
              </button>
            </div>
          )}
        </Show>

        <Show when={shownRoutes().length > 0}>
          <div class="mock-section-head">
            Routes ({shownRoutes().length})
            <Show when={!running()}><span class="mock-section-sub">on disk · start to serve</span></Show>
          </div>
          <div class="mock-routes">
            <For each={shownRoutes()}>
              {(r) => (
                <div class="mock-route">
                  <span class={`method method-${(r.method || "get").toLowerCase()}`}>
                    {r.method || "ANY"}
                  </span>
                  <span class="mock-path">{r.path}</span>
                  <Show
                    when={running() && r.kind === "rule" && (r.variants?.length ?? 0) > 1}
                    fallback={
                      <Show when={r.kind === "resource"} fallback={
                        <span class="mock-status-code">{r.status || 200}</span>
                      }>
                        <span class="mock-badge">CRUD</span>
                      </Show>
                    }
                  >
                    <select
                      class={`mock-variant-select status-${Math.floor(((r.active || r.status) || 200) / 100)}xx`}
                      value={r.active || r.status || 200}
                      title="Switch which response this endpoint returns (live)"
                      onChange={(e) =>
                        void setRouteResponse(r.method, r.path, Number(e.currentTarget.value))
                      }
                    >
                      <For each={r.variants}>
                        {(v) => (
                          <option value={v.status}>
                            {v.status}{v.desc ? ` · ${v.desc}` : ""}
                          </option>
                        )}
                      </For>
                    </select>
                  </Show>
                </div>
              )}
            </For>
          </div>
        </Show>

        <Show when={running()}>
          <div class="mock-section-head">
            Request log
            <button class="mini-btn" onClick={refreshLog}>Refresh</button>
          </div>
          <div class="mock-log">
            <Show when={log().length === 0}>
              <div class="empty-hint">No requests yet.</div>
            </Show>
            <For each={log()}>
              {(entry) => (
                <div class="mock-log-entry">
                  <span class={`method method-${entry.method.toLowerCase()}`}>{entry.method}</span>
                  <span class="mock-log-path">{entry.path}</span>
                  <span class={`mock-log-status status-badge status-${Math.floor(entry.status / 100)}xx`}>
                    {entry.status}
                  </span>
                  <Show when={entry.source}><span class="mock-log-src">{entry.source}</span></Show>
                  <span class="mock-log-time">{entry.at}</span>
                </div>
              )}
            </For>
          </div>
        </Show>

        <Show when={!running()}>
          <Show when={presets().length > 0}>
            <div class="mock-section-head">Quickstart</div>
            <div class="mock-toolbar mock-quickstart">
              <select
                class="body-type-select"
                value={preset()}
                onChange={(e) => setPreset(e.currentTarget.value)}
              >
                <For each={presets()}>
                  {(p) => <option value={p}>{presetLabel(p)}</option>}
                </For>
              </select>
              <button class="mini-btn" onClick={addPreset} disabled={busy()}>
                <Plus size={ICON.xs} /> Add preset
              </button>
            </div>
          </Show>
          <div class="empty-hint mock-hint">
            Presets and OpenAPI-generated mocks write YAML into <code>mocks/</code>.
            Add your own there too — they hot-reload while the server runs.
          </div>
        </Show>
      </div>
    </div>
  );
}
