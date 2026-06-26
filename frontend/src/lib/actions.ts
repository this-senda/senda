// Shared app actions, callable from buttons, keyboard shortcuts and the
// command palette alike.
import { api, BodyType } from "./api";
import {
  activeEnv,
  activePath,
  collection,
  ensurePinned,
  forgetRecent,
  markActiveSaved,
  rememberCollection,
  rememberRecent,
  refreshActivity,
  request,
  sending,
  setCollection,
  setEnvironments,
  setResponse,
  setSending,
} from "./store";
import { promptDialog, confirmDialog, alertDialog } from "./dialog";

// Collections already git-checked this session — avoid re-nagging on every
// external-change reload of the same folder.
const gitChecked = new Set<string>();

// maybeGuardGit warns, once per collection per session, when a git-tracked
// collection has secret/history files that aren't protected from commits, and
// offers to add them to .gitignore. Runs fire-and-forget so it never delays the
// open. Best-effort: any backend error is swallowed.
async function maybeGuardGit(path: string) {
  if (gitChecked.has(path)) return;
  gitChecked.add(path);
  let st: Awaited<ReturnType<typeof api.gitGuardStatus>>;
  try {
    st = await api.gitGuardStatus(path);
  } catch {
    return;
  }
  if (!st?.inGit) return;
  const unignored = st.unignored ?? [];
  const tracked = st.tracked ?? [];

  // Already-committed secrets: .gitignore won't remove them, only warn.
  if (!unignored.length) {
    if (tracked.length) {
      await alertDialog(
        `These secret/history files are already committed to git:\n${tracked
          .map((f) => "  " + f)
          .join("\n")}\n\nRemove them from tracking with:\n  git rm --cached <file>`,
      );
    }
    return;
  }

  let msg = `This collection is in git, but these secret/history files aren't ignored:\n${unignored
    .map((f) => "  " + f)
    .join("\n")}\n\nAdd them to .gitignore?`;
  if (tracked.length) {
    msg += `\n\nNote: ${tracked.join(
      ", ",
    )} are already committed — .gitignore won't remove them (use git rm --cached).`;
  }
  if (await confirmDialog(msg, { okLabel: "Add to .gitignore" })) {
    try {
      await api.gitGuardIgnore(path);
    } catch {
      /* best-effort */
    }
  }
}

// The in-flight send, kept so cancelSend can abort it (Wails calls are
// cancellable promises that propagate ctx cancellation to the backend).
let inFlight: ReturnType<typeof api.send> | undefined;

// sendActive sends the live editor request and stores the response.
export async function sendActive() {
  if (sending()) return;
  // ws/sse aren't HTTP — sending one fires a doomed http.Client call
  // ("unsupported protocol scheme"). Connect/send happens in their tab instead.
  if (request.body?.type === BodyType.BodyWebSocket || request.body?.type === BodyType.BodySSE) return;
  setSending(true);
  setResponse(null);
  try {
    const coll = collection();
    inFlight = api.send(request, coll?.path ?? "", activePath(), activeEnv());
    setResponse(await inFlight);
  } catch (e) {
    if (String(e).toLowerCase().includes("cancel")) {
      setResponse(null); // user aborted: back to the empty state
    } else {
      setResponse({
        status: 0,
        statusText: "",
        durationMs: 0,
        sizeBytes: 0,
        headers: {},
        body: "",
        truncated: false,
        error: String(e),
      } as any);
    }
  } finally {
    inFlight = undefined;
    setSending(false);
    void refreshActivity(collection()?.path ?? "");
  }
}

// cancelSend aborts the in-flight request, if any.
export function cancelSend() {
  inFlight?.cancel();
}

// saveActive persists the live request. A scratch tab (no backing file yet)
// prompts for a name and writes into the open collection root — otherwise a
// brand-new request has no way to be saved.
export async function saveActive() {
  let path = activePath();
  if (!path) {
    const coll = collection();
    if (!coll) return; // no collection open: nowhere to save
    const name = await promptDialog("Request name:", request.name || "new-request");
    if (!name) return;
    path = `${coll.path}/${name.replace(/\.ya?ml$/i, "")}.yaml`;
    await api.saveRequest(path, request);
    markActiveSaved(path);
    await refreshCollection(coll.path); // surface the new file in the sidebar
    return;
  }
  await api.saveRequest(path, request);
  markActiveSaved(path);
}

// refreshCollection re-reads a collection and its environments from disk.
export async function refreshCollection(path: string) {
  const coll = await api.openCollection(path);
  setCollection(coll);
  setEnvironments((await api.listEnvironments(path)) ?? []);
  rememberRecent(coll.name, path);
  ensurePinned(coll.name, path);
  void refreshActivity(path);
  void maybeGuardGit(path);
}

// openCollectionDialog shows the native folder picker and loads the collection.
export async function openCollectionDialog() {
  const path = await api.pickDirectory("Open collection folder");
  if (!path) return; // cancelled
  await refreshCollection(path);
  rememberCollection(path);
}

// openZipCollectionDialog shows the native file picker for a packed .zip
// collection and loads it. Separate from openCollectionDialog because the
// Linux/GTK native dialog cannot offer folder and file selection at once.
export async function openZipCollectionDialog() {
  const path = await api.pickZipCollection("Open .zip collection");
  if (!path) return; // cancelled
  await refreshCollection(path);
  rememberCollection(path);
}

// switchCollection opens a known workspace by path (from the switcher); if the
// folder is gone it drops the entry rather than leaving a dead row.
export async function switchCollection(path: string) {
  if (!path || path === collection()?.path) return;
  try {
    await refreshCollection(path);
    rememberCollection(path);
  } catch (e) {
    forgetRecent(path);
    throw e;
  }
}
