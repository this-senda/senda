import { Show, createSignal, onMount } from "solid-js";
import { Server, Zap, ArrowUpCircle } from "lucide-solid";
import { Browser } from "@wailsio/runtime";
import { ICON } from "../lib/icons";
import { api } from "../lib/api";
import { mockServerAddr, setShowMockPanel, loadTestRunning, runPanelTarget, setRunPanelTarget, setShowRunPanel } from "../lib/store";

export default function StatusBar() {
  const [version, setVersion] = createSignal("");
  const [commit, setCommit] = createSignal("");
  const [date, setDate] = createSignal("");
  const [latest, setLatest] = createSignal("");
  const [updateURL, setUpdateURL] = createSignal("");

  onMount(async () => {
    try {
      const bi = await api.buildInfo();
      setVersion(bi.version);
      setCommit(bi.commit ?? "");
      setDate(bi.date ?? "");
    } catch {
      /* bindings unavailable (e.g. browser dev) — leave blank */
    }
    try {
      const u = await api.checkUpdate();
      setUpdateURL(u.url);
      if (u.available) setLatest(u.latest);
    } catch {
      /* offline or rate-limited — silently skip the update hint */
    }
  });

  // "1.2.3" -> "v1.2.3"; "dev" (or any non-numeric) stays as-is, no "vdev".
  const verLabel = () => (/^\d/.test(version()) ? `v${version()}` : version());

  // Tooltip with the full build stamp; the chip itself stays compact.
  const buildTitle = () => {
    const parts = [`Senda ${verLabel()}`];
    if (commit()) parts.push(`commit ${commit()}`);
    if (date()) parts.push(`built ${date()}`);
    return parts.join("\n");
  };

  return (
    <div class="status-bar">
      <Show when={loadTestRunning()}>
        <button
          class="status-bar-chip status-bar-load"
          onClick={() => {
            setRunPanelTarget((t) => (t ? { ...t, initialTab: "load" } : t));
            setShowRunPanel(true);
          }}
          title="Load test running — click to open"
        >
          <Zap size={ICON.xs} />
          <span>Load test{runPanelTarget() ? `: ${runPanelTarget()!.folderName}` : ""} running…</span>
        </button>
      </Show>
      <Show when={mockServerAddr()}>
        <button
          class="status-bar-chip status-bar-mock"
          onClick={() => setShowMockPanel(true)}
          title="Mock server running — click to open"
        >
          <Server size={ICON.xs} />
          <span>Mock {mockServerAddr()}</span>
        </button>
      </Show>

      <div class="status-bar-spacer" />

      <Show when={latest()}>
        <button
          class="status-bar-chip status-bar-update"
          onClick={() => Browser.OpenURL(updateURL())}
          title={`Update available: v${latest()} — click to open the releases page`}
        >
          <ArrowUpCircle size={ICON.xs} />
          <span>Update → v{latest()}</span>
        </button>
      </Show>
      <Show when={version()}>
        <button
          class="status-bar-build"
          onClick={() => updateURL() && Browser.OpenURL(updateURL())}
          title={buildTitle()}
        >
          {verLabel()}{commit() ? ` · ${commit()}` : ""}
        </button>
      </Show>
    </div>
  );
}
