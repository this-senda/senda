// Standalone security-scan modal, opened from a tree node's context menu.
// Runs the builtin check pack (and, unless restricted to builtins, the
// collection's .security/ templates) against the node's resolved request URLs;
// every executed check — finding, pass or probe error — streams in live via
// "security:start"/"security:check" Wails events. Before a run it previews how
// many checks the current scope/filter will execute; after one it can export a
// self-contained HTML report.
import { createEffect, createMemo, createSignal, For, onCleanup, onMount, Show } from "solid-js";
import { Check, CircleAlert, Download, ExternalLink, RefreshCw, X } from "lucide-solid";
import { ICON } from "../lib/icons";
import { Browser, Events } from "@wailsio/runtime";
import { api } from "../lib/api";
import type { ScanPlan, SecurityCheck, SecuritySummary, SyncState } from "../lib/api";
import { activeEnv, collection } from "../lib/store";

// A curated, mostly-supported subset of the community nuclei templates is a
// sensible default source; users can point at any nuclei-template repo.
const DEFAULT_TEMPLATE_REPO = "https://github.com/projectdiscovery/nuclei-templates";

// Scope picks the template source: the embedded builtin pack only, or that pack
// plus the synced .security templates (optionally pre-filtered by tags so the
// common "just the OWASP/misconfig stuff" case is one click instead of a full
// nuclei run). Tags here only seed the editable Tags input — the user can still
// refine them.
const SCOPES = [
  {
    label: "Built-in pack (fast)",
    builtin: true,
    tags: "",
    hint: "~13 curated API checks. No template download needed.",
  },
  {
    label: "OWASP / misconfig (synced)",
    builtin: false,
    tags: "owasp,misconfig,exposure",
    hint: "Synced templates tagged owasp/misconfig/exposure, plus the builtin pack.",
  },
  {
    label: "Full nuclei (synced)",
    builtin: false,
    tags: "",
    hint: "Every supported synced http template, plus the builtin pack. Largest run.",
  },
] as const;

// Severity presets map to nuclei's comma-separated severity filter.
const SEVERITIES = [
  { label: "All severities", severity: "" },
  { label: "Medium and up", severity: "medium,high,critical" },
  { label: "High and critical", severity: "high,critical" },
] as const;

const SEVERITY_ORDER = ["critical", "high", "medium", "low", "info", "unknown"];

const esc = (s: string) =>
  String(s ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");

export default function SecurityScan(props: {
  folderPath: string;
  folderName: string;
  onClose: () => void;
}) {
  const [scope, setScope] = createSignal(0);
  const [severity, setSeverity] = createSignal(0);
  const [tags, setTags] = createSignal("");
  const [running, setRunning] = createSignal(false);
  const [targets, setTargets] = createSignal<string[]>([]);
  const [checks, setChecks] = createSignal<SecurityCheck[]>([]);
  const [summary, setSummary] = createSignal<SecuritySummary | null>(null);
  const [error, setError] = createSignal("");
  const [expanded, setExpanded] = createSignal<string | null>(null);

  // run-size preview for the current options (no traffic sent)
  const [plan, setPlan] = createSignal<ScanPlan | null>(null);
  const [planning, setPlanning] = createSignal(false);
  // monotonic id so a slow earlier preview can't overwrite a newer one
  let planSeq = 0;

  // template-repo sync state
  const [repoUrl, setRepoUrl] = createSignal(DEFAULT_TEMPLATE_REPO);
  const [syncState, setSyncState] = createSignal<SyncState | null>(null);
  const [syncing, setSyncing] = createSignal(false);
  const [syncError, setSyncError] = createSignal("");

  const buildOpts = () => ({
    severity: SEVERITIES[severity()].severity,
    tags: tags()
      .split(",")
      .map((t) => t.trim())
      .filter(Boolean),
    builtin: SCOPES[scope()].builtin,
    rateLimit: 0,
    timeout: 0,
  });

  onMount(async () => {
    const coll = collection();
    if (!coll) return;
    try {
      const st = await api.securityTemplatesState(coll.path);
      if (st?.url) {
        setSyncState(st);
        setRepoUrl(st.url);
      }
    } catch {
      /* never synced; keep defaults */
    }
  });

  // Picking a scope reseeds the tag filter with that scope's defaults.
  const pickScope = (i: number) => {
    setScope(i);
    setTags(SCOPES[i].tags);
  };

  // Refresh the run-size preview whenever the options (or synced template set)
  // change, debounced so typing tags doesn't spam the backend. Counting a large
  // synced template set is slow on the first call (then cached), so a stale
  // result is dropped via planSeq rather than shown as the answer.
  const refreshPlan = async () => {
    const coll = collection();
    const seq = ++planSeq;
    try {
      const p = await api.securityScanPlan(props.folderPath, coll?.path ?? "", activeEnv(), buildOpts());
      if (seq === planSeq) setPlan(p);
    } catch {
      if (seq === planSeq) setPlan(null);
    } finally {
      if (seq === planSeq) setPlanning(false);
    }
  };
  createEffect(() => {
    scope();
    severity();
    tags();
    syncState();
    if (running()) {
      setPlanning(false);
      return;
    }
    // Show "counting…" the instant options change, not the previous number.
    setPlanning(true);
    const id = setTimeout(refreshPlan, 250);
    onCleanup(() => clearTimeout(id));
  });

  const sync = async () => {
    const coll = collection();
    if (!coll || syncing()) return;
    setSyncing(true);
    setSyncError("");
    try {
      const st = await api.syncSecurityTemplates(coll.path, repoUrl(), "");
      setSyncState(st);
    } catch (e) {
      setSyncError(String(e));
    } finally {
      setSyncing(false);
    }
  };

  const syncedAgo = () => {
    const at = syncState()?.syncedAt;
    if (!at) return "";
    return new Date(at).toLocaleString();
  };

  const findings = createMemo(() =>
    checks()
      .filter((c) => c.matched)
      .sort((a, b) => SEVERITY_ORDER.indexOf(a.severity) - SEVERITY_ORDER.indexOf(b.severity))
  );
  const passed = createMemo(() => checks().filter((c) => !c.matched && !c.error));
  const errored = createMemo(() => checks().filter((c) => !c.matched && c.error));

  const rowKey = (c: SecurityCheck) => `${c.templateId}@${c.target}`;
  const toggle = (c: SecurityCheck) => {
    const k = rowKey(c);
    setExpanded(expanded() === k ? null : k);
  };

  let call: ReturnType<typeof api.runSecurityScan> | undefined;
  let offEvents: (() => void)[] = [];

  const start = async () => {
    offEvents.forEach((f) => f());
    offEvents = [];
    setSummary(null);
    setError("");
    setChecks([]);
    setTargets([]);
    setExpanded(null);
    setRunning(true);
    offEvents.push(
      Events.On("security:start", (e: any) => setTargets(e.data?.targets ?? [])),
      Events.On("security:check", (e: any) =>
        setChecks((prev) => [...prev, e.data as SecurityCheck])
      ),
    );
    try {
      const coll = collection();
      call = api.runSecurityScan(props.folderPath, coll?.path ?? "", activeEnv(), buildOpts());
      const result = await call;
      if (result) setSummary(result);
    } catch (e: any) {
      if (!String(e).includes("cancel")) setError(String(e));
    } finally {
      offEvents.forEach((f) => f());
      offEvents = [];
      setRunning(false);
    }
  };

  const stop = () => call?.cancel();

  onCleanup(() => {
    call?.cancel();
    offEvents.forEach((f) => f());
  });

  const close = () => {
    call?.cancel();
    props.onClose();
  };

  // Build a self-contained HTML report from the completed run and hand it to
  // the OS save dialog via the export binding.
  const downloadReport = () => {
    const s = summary();
    if (!s) return;
    const stamp = new Date();
    const a = (url: string, label: string) =>
      `<a href="${esc(url)}" target="_blank" rel="noreferrer">${esc(label)}</a>`;
    // Every doc/reference link for a finding: explicit references, CVEs, CWEs.
    const linksHtml = (c: SecurityCheck) => {
      const out: string[] = [];
      for (const id of cveIds(c)) out.push(a(cveUrl(id), id));
      for (const id of c.cwe ?? []) if (cweUrl(id)) out.push(a(cweUrl(id), id));
      for (const url of c.reference ?? []) out.push(a(url, shortUrl(url)));
      return out.length ? `<div class="refs"><b>Refs:</b> ${out.join(" · ")}</div>` : "";
    };
    const sevRow = (c: SecurityCheck) => `
      <tr>
        <td><span class="sev sev-${esc(c.severity)}">${esc(c.severity)}</span></td>
        <td>${esc(c.name)}<div class="tid">${esc(c.templateId)}</div></td>
        <td>${c.owasp ? a(owaspUrl(), c.owasp) : ""}</td>
        <td class="mono">${esc(c.matchedAt ?? c.target)}</td>
        <td>${esc(c.description ?? "")}${
          c.remediation ? `<div class="rem"><b>Fix:</b> ${esc(c.remediation)}</div>` : ""
        }${linksHtml(c)}</td>
      </tr>`;
    const errRow = (c: SecurityCheck) => `
      <tr><td>${esc(c.name)}<div class="tid">${esc(c.templateId)}</div></td>
      <td class="mono">${esc(c.target)}</td><td>${esc(c.error ?? "")}</td></tr>`;
    const sevSummary = SEVERITY_ORDER.filter((sev) => (s.bySeverity ?? {})[sev])
      .map((sev) => `<span class="sev sev-${sev}">${sev} ×${(s.bySeverity ?? {})[sev]}</span>`)
      .join(" ");
    // Coverage breakdown: how the executed checks distribute across severity and
    // OWASP category. Gives the report substance even when there are 0 findings —
    // it documents what was actually checked, not just the pass/fail count.
    const tally = (key: (c: SecurityCheck) => string) => {
      const m = new Map<string, number>();
      for (const c of checks()) {
        const k = key(c);
        if (k) m.set(k, (m.get(k) ?? 0) + 1);
      }
      return m;
    };
    const bySev = tally((c) => c.severity || "unknown");
    const byCat = tally((c) => c.owasp ?? "");
    const sevBreakdown = SEVERITY_ORDER.filter((sev) => bySev.get(sev))
      .map((sev) => `<tr><td><span class="sev sev-${sev}">${sev}</span></td><td>${bySev.get(sev)}</td></tr>`)
      .join("");
    const catBreakdown = [...byCat.entries()]
      .sort((x, y) => y[1] - x[1])
      .map(([cat, n]) => `<tr><td>${esc(cat)}</td><td>${n}</td></tr>`)
      .join("");
    const coverageHtml = checks().length
      ? `<h2>Checks performed (${checks().length})</h2>
<div class="cov">
  <table><tr><th>Severity</th><th>Count</th></tr>${sevBreakdown}</table>
  ${catBreakdown ? `<table><tr><th>OWASP category</th><th>Count</th></tr>${catBreakdown}</table>` : ""}
</div>`
      : "";
    const html = `<!doctype html><html><head><meta charset="utf-8">
<title>Security report — ${esc(props.folderName)}</title>
<style>
  body{font:14px/1.5 -apple-system,Segoe UI,Roboto,sans-serif;margin:32px;color:#1a1a1a;background:#fafafa}
  h1{font-size:20px;margin:0 0 4px} .meta{color:#666;font-size:12px;margin-bottom:20px}
  .cards{display:flex;gap:12px;flex-wrap:wrap;margin:16px 0}
  .card{background:#fff;border:1px solid #e3e3e3;border-radius:8px;padding:10px 16px;min-width:90px}
  .card b{display:block;font-size:22px} .card span{font-size:11px;color:#666;text-transform:uppercase;letter-spacing:.04em}
  table{width:100%;border-collapse:collapse;background:#fff;border:1px solid #e3e3e3;border-radius:8px;overflow:hidden;margin:8px 0 24px}
  th,td{text-align:left;padding:8px 12px;border-bottom:1px solid #eee;vertical-align:top;font-size:13px}
  th{background:#f3f3f3;font-size:11px;text-transform:uppercase;letter-spacing:.04em;color:#555}
  .mono{font-family:ui-monospace,Menlo,monospace;font-size:12px;word-break:break-all}
  .tid{color:#999;font-size:11px;font-family:ui-monospace,monospace}
  .rem{color:#444;margin-top:4px;font-size:12px}
  a{color:#0969da;text-decoration:none} a:hover{text-decoration:underline}
  .refs{margin-top:6px;font-size:12px;display:flex;flex-wrap:wrap;gap:6px;align-items:baseline}
  .refs b{color:#666;font-weight:600}
  .sev{font-size:11px;text-transform:uppercase;padding:1px 6px;border-radius:4px;border:1px solid;font-family:ui-monospace,monospace}
  .sev-critical{color:#c00;border-color:#c00} .sev-high{color:#e0670f;border-color:#e0670f}
  .sev-medium{color:#b58900;border-color:#b58900} .sev-low{color:#3a7;border-color:#3a7}
  .sev-info,.sev-unknown{color:#888;border-color:#888}
  h2{font-size:15px;margin:20px 0 4px} .empty{color:#888}
  .cov{display:flex;gap:24px;flex-wrap:wrap;align-items:flex-start}
  .cov table{width:auto;min-width:220px}
</style></head><body>
<h1>Security scan — ${esc(props.folderName)}</h1>
<div class="meta">${esc(stamp.toLocaleString())} · ${s.targets} target(s) · ${s.checks} check(s) · ${s.duration.toFixed(1)}s</div>
<div class="cards">
  <div class="card"><b>${s.findings}</b><span>Findings</span></div>
  <div class="card"><b>${s.passed}</b><span>Passed</span></div>
  <div class="card"><b>${s.errors}</b><span>Errored</span></div>
  <div class="card"><b>${s.targets}</b><span>Targets</span></div>
</div>
<div>${sevSummary}</div>
${coverageHtml}
<h2>Findings (${findings().length})</h2>
${
  findings().length
    ? `<table><tr><th>Severity</th><th>Check</th><th>OWASP</th><th>URL</th><th>Detail</th></tr>${findings()
        .map(sevRow)
        .join("")}</table>`
    : `<p class="empty">No findings — all executed checks passed.</p>`
}
${
  errored().length
    ? `<h2>Errored (${errored().length})</h2><table><tr><th>Check</th><th>Target</th><th>Error</th></tr>${errored()
        .map(errRow)
        .join("")}</table>`
    : ""
}
<h2>Targets</h2>
<ul class="mono">${targets().map((t) => `<li>${esc(t)}</li>`).join("")}</ul>
</body></html>`;
    const safe = props.folderName.replace(/[^\w.-]+/g, "-");
    const ts = stamp.toISOString().slice(0, 10);
    api.exportFile(`security-report-${safe}-${ts}.html`, html);
  };

  // Open external docs in the user's real browser (not the webview).
  const openUrl = (url: string) => void Browser.OpenURL(url);
  const cweUrl = (id: string) => {
    const n = id.replace(/\D/g, "");
    return n ? `https://cwe.mitre.org/data/definitions/${n}.html` : "";
  };
  const cveUrl = (id: string) => `https://nvd.nist.gov/vuln/detail/${id}`;
  // The OWASP API Top 10 (2023) index — the per-finding owasp string names a
  // category within it.
  const owaspUrl = () => "https://owasp.org/API-Security/editions/2023/en/0x11-t10/";
  const shortUrl = (u: string) => u.replace(/^https?:\/\//, "").replace(/\/$/, "");
  // CVE ids aren't a dedicated field; pull them out of the id, tags and refs.
  const cveIds = (c: SecurityCheck) => {
    const hay = [c.templateId, ...(c.tags ?? []), ...(c.reference ?? [])].join(" ");
    return [...new Set((hay.match(/CVE-\d{4}-\d{4,}/gi) ?? []).map((s) => s.toUpperCase()))];
  };
  // Best single "open the docs" link for a finding, in priority order: an
  // explicit reference URL, then CVE, then CWE, then the OWASP category page.
  const docUrl = (c: SecurityCheck) => {
    if (c.reference?.length) return c.reference[0];
    const cves = cveIds(c);
    if (cves.length) return cveUrl(cves[0]);
    if (c.cwe?.length && cweUrl(c.cwe[0])) return cweUrl(c.cwe[0]);
    if (c.owasp) return owaspUrl();
    return "";
  };

  const link = (label: string, url: string) => (
    <button class="sec-link" onClick={() => openUrl(url)} title={url}>
      {label}
      <ExternalLink size={ICON.xs} />
    </button>
  );

  const detail = (c: SecurityCheck) => {
    const cves = cveIds(c);
    return (
      <div class="sec-detail">
        <div class="sec-detail-row">
          <span class="sec-detail-label">Template</span>
          <span>{c.templateId}</span>
        </div>
        <div class="sec-detail-row">
          <span class="sec-detail-label">Target</span>
          <span>{c.target}</span>
        </div>
        <Show when={c.matchedAt}>
          <div class="sec-detail-row">
            <span class="sec-detail-label">Matched at</span>
            <span>{c.matchedAt}</span>
          </div>
        </Show>
        <Show when={c.error}>
          <div class="sec-detail-row">
            <span class="sec-detail-label">Error</span>
            <span class="sec-detail-err">{c.error}</span>
          </div>
        </Show>
        <Show when={c.description}>
          <div class="sec-detail-row">
            <span class="sec-detail-label">Description</span>
            <span>{c.description}</span>
          </div>
        </Show>
        <Show when={c.remediation}>
          <div class="sec-detail-row">
            <span class="sec-detail-label">Remediation</span>
            <span>{c.remediation}</span>
          </div>
        </Show>
        <Show when={c.owasp}>
          <div class="sec-detail-row">
            <span class="sec-detail-label">OWASP</span>
            <span class="sec-link-row">{link(c.owasp!, owaspUrl())}</span>
          </div>
        </Show>
        <Show when={cves.length}>
          <div class="sec-detail-row">
            <span class="sec-detail-label">CVE</span>
            <span class="sec-link-row">
              <For each={cves}>{(id) => link(id, cveUrl(id))}</For>
            </span>
          </div>
        </Show>
        <Show when={c.cwe?.length}>
          <div class="sec-detail-row">
            <span class="sec-detail-label">CWE</span>
            <span class="sec-link-row">
              <For each={c.cwe!}>
                {(id) => (cweUrl(id) ? link(id, cweUrl(id)) : <span>{id}</span>)}
              </For>
            </span>
          </div>
        </Show>
        <Show when={c.reference?.length}>
          <div class="sec-detail-row">
            <span class="sec-detail-label">References</span>
            <span class="sec-link-row sec-link-col">
              <For each={c.reference!}>{(url) => link(shortUrl(url), url)}</For>
            </span>
          </div>
        </Show>
        <Show when={c.tags?.length}>
          <div class="sec-detail-row">
            <span class="sec-detail-label">Tags</span>
            <span>{c.tags!.join(", ")}</span>
          </div>
        </Show>
      </div>
    );
  };

  return (
    <div class="modal-backdrop" onClick={close}>
      <div class="modal modal-wide" onClick={(e) => e.stopPropagation()}>
        <div class="modal-head">
          <span class="modal-title">
            Security Scan — {props.folderName}
            <Show when={summary()}>
              <span class="run-summary"> — {summary()!.findings} finding(s)</span>
            </Show>
          </span>
          <button class="icon-btn" title="Close" onClick={close}>
            <X size={ICON.md} />
          </button>
        </div>

        <div class="modal-body">
          <div class="load-panel">
            <div class="sec-warning">
              Scans send real attack probes to the request URLs. Only scan APIs you
              own or are authorized to test. Runs the built-in check pack plus any
              nuclei-format http templates in the collection's{" "}
              <code>.security/</code> folder.
            </div>

            <div class="sec-sync">
              <input
                class="load-input sec-sync-url"
                type="text"
                placeholder="nuclei-template git repo URL"
                value={repoUrl()}
                onInput={(e) => setRepoUrl(e.currentTarget.value)}
                disabled={syncing()}
              />
              <button
                class="btn sec-sync-btn"
                onClick={sync}
                disabled={syncing() || !collection()}
              >
                <RefreshCw size={ICON.xs} class={syncing() ? "spin" : ""} />
                {syncing() ? "Updating…" : "Update templates"}
              </button>
              <Show when={syncState()}>
                <span class="sec-sync-info">
                  {syncState()!.templates} templates · {syncedAgo()}
                  <Show when={syncState()!.commit}>
                    {" "}· {syncState()!.commit!.slice(0, 7)}
                  </Show>
                </span>
              </Show>
            </div>
            <Show when={syncError()}>
              <div class="modal-error">{syncError()}</div>
            </Show>

            <div class="load-config">
              <label class="load-field">
                <span class="load-label">Scope</span>
                <select
                  class="load-select"
                  value={scope()}
                  onChange={(e) => pickScope(+e.currentTarget.value)}
                  disabled={running()}
                >
                  <For each={SCOPES}>{(s, i) => <option value={i()}>{s.label}</option>}</For>
                </select>
              </label>
              <label class="load-field">
                <span class="load-label">Severity</span>
                <select
                  class="load-select"
                  value={severity()}
                  onChange={(e) => setSeverity(+e.currentTarget.value)}
                  disabled={running()}
                >
                  <For each={SEVERITIES}>{(p, i) => <option value={i()}>{p.label}</option>}</For>
                </select>
              </label>
              <label class="load-field">
                <span class="load-label">Tags</span>
                <input
                  class="load-input sec-tags-input"
                  type="text"
                  placeholder="e.g. misconfig,exposure,cve"
                  value={tags()}
                  onInput={(e) => setTags(e.currentTarget.value)}
                  disabled={running()}
                />
              </label>
              <button class="btn load-start-btn" onClick={() => (running() ? stop() : start())}>
                {running() ? "Stop" : "Scan"}
              </button>
            </div>

            <div class="sec-scope-hint">{SCOPES[scope()].hint}</div>

            <Show when={!running()}>
              <Show
                when={!planning() && plan()}
                fallback={
                  <Show when={planning()}>
                    <div class="sec-plan">Counting checks…</div>
                  </Show>
                }
              >
                {(p) => (
                  <div class="sec-plan" classList={{ "sec-plan-empty": p().checks === 0 }}>
                    <Show
                      when={p().checks > 0}
                      fallback={
                        <span>
                          {p().targets === 0
                            ? "No resolvable URLs under this folder — check the environment."
                            : "No templates match this scope/filter — sync templates or widen it."}
                        </span>
                      }
                    >
                      <span>
                        Will run <b>{p().checks}</b> check(s) — {p().templates} template(s) ×{" "}
                        {p().targets} target(s)
                      </span>
                    </Show>
                  </div>
                )}
              </Show>
            </Show>

            <Show when={error()}>
              <div class="modal-error">{error()}</div>
            </Show>

            <Show when={running()}>
              <div class="empty-hint">
                Scanning {targets().length > 0 ? `${targets().length} target(s)` : ""}… findings
                appear below as they match.
              </div>
            </Show>

            <Show when={summary()}>
              {(s) => (
                <div class="sec-summary">
                  <span>{s().targets} target(s)</span>
                  <span>{s().checks} check(s)</span>
                  <span class="sec-summary-pass">{s().passed} passed</span>
                  <span classList={{ "sec-summary-fail": s().findings > 0 }}>
                    {s().findings} finding(s)
                  </span>
                  <Show when={s().errors > 0}>
                    <span class="sec-summary-err">{s().errors} errored</span>
                  </Show>
                  <span>{s().duration.toFixed(1)}s</span>
                  <For each={SEVERITY_ORDER.filter((sev) => (s().bySeverity ?? {})[sev])}>
                    {(sev) => (
                      <span class={`sec-badge sec-${sev}`}>
                        {sev} ×{s().bySeverity[sev]}
                      </span>
                    )}
                  </For>
                  <button class="btn sec-report-btn" onClick={downloadReport}>
                    <Download size={ICON.xs} />
                    Download report
                  </button>
                </div>
              )}
            </Show>

            <div class="sec-list">
              <For each={findings()}>
                {(f) => (
                  <div class="sec-row" onClick={() => toggle(f)}>
                    <div class="sec-row-head">
                      <span class={`sec-badge sec-${f.severity}`}>{f.severity}</span>
                      <span class="sec-name">{f.name}</span>
                      <Show when={f.owasp}>
                        <span class="sec-badge sec-owasp" title={f.owasp}>
                          {f.owasp!.split(" ")[0]}
                        </span>
                      </Show>
                      <span class="sec-matched" title={f.matchedAt}>{f.matchedAt}</span>
                      <Show when={docUrl(f)}>
                        <button
                          class="icon-btn sec-doc-btn"
                          title={`Open docs — ${docUrl(f)}`}
                          onClick={(e) => {
                            e.stopPropagation();
                            openUrl(docUrl(f));
                          }}
                        >
                          <ExternalLink size={ICON.sm} />
                        </button>
                      </Show>
                    </div>
                    <Show when={expanded() === rowKey(f)}>{detail(f)}</Show>
                  </div>
                )}
              </For>
            </div>

            <Show when={!running() && summary() && checks().length > 0 && findings().length === 0}>
              <div class="empty-hint">No findings — all executed checks passed.</div>
            </Show>
            <Show when={!running() && !summary() && checks().length === 0}>
              <div class="empty-hint">Configure and press Scan to test the endpoints.</div>
            </Show>

            <Show when={errored().length > 0}>
              <div class="sec-section-head">Errored ({errored().length})</div>
              <div class="sec-list sec-list-muted">
                <For each={errored()}>
                  {(c) => (
                    <div class="sec-row" onClick={() => toggle(c)}>
                      <div class="sec-row-head">
                        <span class="sec-pass-icon sec-err-icon">
                          <CircleAlert size={ICON.xs} />
                        </span>
                        <span class="sec-name">{c.name}</span>
                        <span class="sec-matched" title={c.error}>{c.error}</span>
                      </div>
                      <Show when={expanded() === rowKey(c)}>{detail(c)}</Show>
                    </div>
                  )}
                </For>
              </div>
            </Show>

            <Show when={passed().length > 0}>
              <div class="sec-section-head">Passed ({passed().length})</div>
              <div class="sec-list sec-list-muted">
                <For each={passed()}>
                  {(c) => (
                    <div class="sec-row" onClick={() => toggle(c)}>
                      <div class="sec-row-head">
                        <span class="sec-pass-icon">
                          <Check size={ICON.xs} />
                        </span>
                        <span class="sec-name">{c.name}</span>
                        <span class="sec-matched" title={c.target}>{c.target}</span>
                      </div>
                      <Show when={expanded() === rowKey(c)}>{detail(c)}</Show>
                    </div>
                  )}
                </For>
              </div>
            </Show>
          </div>
        </div>

        <div class="modal-foot">
          <button class="btn" onClick={close}>
            {running() ? "Cancel" : "Close"}
          </button>
        </div>
      </div>
    </div>
  );
}
