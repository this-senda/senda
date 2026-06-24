// Shared app actions, callable from buttons, keyboard shortcuts and the
// command palette alike.
import { api } from "./api";
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

// The in-flight send, kept so cancelSend can abort it (Wails calls are
// cancellable promises that propagate ctx cancellation to the backend).
let inFlight: ReturnType<typeof api.send> | undefined;

// sendActive sends the live editor request and stores the response.
export async function sendActive() {
  if (sending()) return;
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
    const name = prompt("Request name:", request.name || "new-request");
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
