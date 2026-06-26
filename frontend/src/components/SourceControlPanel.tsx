// Read-only git comparison modal — Senda's take on Yaak's "Commit Changes" view,
// minus the writes. Left: the requests/files that changed versus HEAD, badged by
// status and split into the request tree vs "External file changes". Right: the
// semantic per-field diff of the selected entry (URL changed, header added, …),
// or a raw text diff for non-request files.
//
// Disk is the source of truth, so this just reads `GitStatus`/`GitDiff` and
// refetches on open — no staging, no commit. The collection dir is the git
// working tree; every call takes collection().path like the rest of the API.
import { createMemo, createResource, createSignal, For, Show } from "solid-js";
import { GitCompare, RefreshCw, X } from "lucide-solid";
import { ICON } from "../lib/icons";
import { api } from "../lib/api";
import type { ChangedFile, GitDiff } from "../lib/api";
import { collection } from "../lib/store";

const STATUS_LABEL: Record<string, string> = {
  modified: "modified",
  added: "added",
  deleted: "deleted",
  renamed: "renamed",
  untracked: "untracked",
};

export default function SourceControlPanel(props: { onClose: () => void }) {
  const collPath = () => collection()?.path ?? "";
  const [selected, setSelected] = createSignal<ChangedFile | null>(null);

  const [status, { refetch }] = createResource(collPath, async (p) =>
    p ? await api.gitStatus(p) : null
  );

  // Diff for the selected entry; re-runs whenever the selection changes.
  const [diff] = createResource<GitDiff | null, ChangedFile>(selected, async (f) =>
    f && collPath() ? await api.gitDiff(collPath(), f.path) : null
  );

  const requestFiles = createMemo(() => (status()?.files ?? []).filter((f) => !f.other));
  const otherFiles = createMemo(() => (status()?.files ?? []).filter((f) => f.other));
  const changed = createMemo(() => status()?.files?.length ?? 0);

  const row = (f: ChangedFile) => (
    <button
      class="scm-row"
      classList={{ active: selected()?.path === f.path }}
      onClick={() => setSelected(f)}
      title={f.path}
    >
      <span class="scm-name">{f.display}</span>
      <span class={`scm-badge scm-${f.status}`}>{STATUS_LABEL[f.status] ?? f.status}</span>
    </button>
  );

  return (
    <div class="modal-backdrop" onClick={props.onClose}>
      <div class="modal modal-wide scm-modal" onClick={(e) => e.stopPropagation()}>
        <div class="modal-head">
          <span class="modal-title">
            <GitCompare size={ICON.md} /> Changes
            <Show when={status()?.branch}>
              <span class="scm-branch">{status()!.branch}</span>
            </Show>
            <Show when={changed() > 0}>
              <span class="run-summary"> — {changed()} changed</span>
            </Show>
          </span>
          <div class="scm-head-actions">
            <button class="icon-btn" title="Refresh" onClick={() => refetch()}>
              <RefreshCw size={ICON.sm} class={status.loading ? "spin" : ""} />
            </button>
            <button class="icon-btn" title="Close" onClick={props.onClose}>
              <X size={ICON.md} />
            </button>
          </div>
        </div>

        <div class="modal-body scm-body">
          {/* left: changed entries */}
          <div class="scm-list">
            <Show
              when={status()?.repo}
              fallback={
                <div class="empty-hint">
                  {status.loading
                    ? "Loading…"
                    : "This collection isn't a git repository. Run git init in the collection folder to compare changes."}
                </div>
              }
            >
              <Show
                when={changed() > 0}
                fallback={<div class="empty-hint">No changes — working tree matches HEAD.</div>}
              >
                <For each={requestFiles()}>{row}</For>
                <Show when={otherFiles().length > 0}>
                  <div class="scm-section-head">External file changes</div>
                  <For each={otherFiles()}>{row}</For>
                </Show>
              </Show>
            </Show>
          </div>

          {/* right: diff for the selection */}
          <div class="scm-diff">
            <Show
              when={selected()}
              fallback={<div class="scm-diff-empty">Select a change to view diff</div>}
            >
              <Show when={!diff.loading} fallback={<div class="scm-diff-empty">Loading diff…</div>}>
                <Show when={diff()} keyed>
                  {(d) => (
                    <>
                      <div class="scm-diff-title">{d.display}</div>
                      <Show
                        when={d.fields && d.fields.length > 0}
                        fallback={
                          <Show
                            when={d.raw?.trim()}
                            fallback={<div class="scm-diff-empty">No field-level changes.</div>}
                          >
                            <pre class="scm-raw">{d.raw}</pre>
                          </Show>
                        }
                      >
                        <For each={d.fields}>
                          {(fd) => (
                            <div class={`scm-field scm-field-${fd.kind}`}>
                              <div class="scm-field-label">
                                {fd.label}
                                <span class={`scm-badge scm-${fd.kind}`}>{fd.kind}</span>
                              </div>
                              <Show when={fd.kind !== "added" && fd.old}>
                                <pre class="scm-old">{fd.old}</pre>
                              </Show>
                              <Show when={fd.kind !== "removed" && fd.new}>
                                <pre class="scm-new">{fd.new}</pre>
                              </Show>
                            </div>
                          )}
                        </For>
                      </Show>
                    </>
                  )}
                </Show>
              </Show>
            </Show>
          </div>
        </div>
      </div>
    </div>
  );
}
