// Collection-level settings modal. Currently edits the collection's default
// auth, which per-request "Inherit" auth falls back to. Saves to senda.meta.yaml via
// SaveCollection and updates the in-memory collection.
import { createSignal, Show } from "solid-js";
import { X } from "lucide-solid";
import { ICON } from "../lib/icons";
import { api } from "../lib/api";
import type { Auth, Collection, TLSConfig } from "../lib/api";
import { collection, setCollection } from "../lib/store";
import { blankAuth } from "../lib/factory";
import AuthEditor from "./AuthEditor";

const blankTLS = (): TLSConfig => ({ certFile: "", keyFile: "", caFile: "", insecure: false });

export default function CollectionSettings(props: { onClose: () => void }) {
  const coll = collection()!;
  const [auth, setAuth] = createSignal<Auth>(
    (coll.auth && coll.auth.type ? coll.auth : blankAuth()) as Auth
  );
  const [proxy, setProxy] = createSignal(coll.proxy ?? "");
  const [tls, setTls] = createSignal<TLSConfig>(coll.tls ? { ...blankTLS(), ...coll.tls } : blankTLS());
  const patchTls = (p: Partial<TLSConfig>) => setTls({ ...tls(), ...p });
  const browse = async (set: (path: string) => void) => {
    const path = await api.pickFile("Select certificate file");
    if (path) set(path); // "" = cancelled
  };
  const [saving, setSaving] = createSignal(false);
  const [error, setError] = createSignal("");

  const save = async () => {
    setSaving(true);
    setError("");
    try {
      const next: Collection = { ...coll, auth: auth(), proxy: proxy(), tls: tls() };
      await api.saveCollection(next);
      setCollection(next);
      props.onClose();
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div class="modal-backdrop" onClick={props.onClose}>
      <div class="modal" onClick={(e) => e.stopPropagation()}>
        <div class="modal-head">
          <span class="modal-title">{coll.name} — Settings</span>
          <button class="icon-btn" title="Close" onClick={props.onClose}>
            <X size={ICON.md} />
          </button>
        </div>
        <div class="modal-body">
          <div class="modal-section-label">Default authentication</div>
          <AuthEditor
            auth={auth()}
            onChange={setAuth}
            allowInherit={false}
          />

          <div class="modal-section-label">Network</div>
          <div class="net-fields">
            <p class="modal-hint">
              Proxy and client certificate for this collection's requests. Values support{" "}
              <code>{"{{var}}"}</code> so machine-specific URLs and paths stay out of git.
            </p>
            <input
              class="net-input"
              type="text"
              placeholder="Proxy URL (e.g. http://host:8080) — blank uses system/env"
              value={proxy()}
              onInput={(e) => setProxy(e.currentTarget.value)}
            />
            <div class="net-file">
              <input
                class="net-input"
                type="text"
                placeholder="Client cert file (.pem)"
                value={tls().certFile}
                onInput={(e) => patchTls({ certFile: e.currentTarget.value })}
              />
              <button class="btn ghost" onClick={() => browse((p) => patchTls({ certFile: p }))}>
                Browse…
              </button>
            </div>
            <div class="net-file">
              <input
                class="net-input"
                type="text"
                placeholder="Client key file"
                value={tls().keyFile}
                onInput={(e) => patchTls({ keyFile: e.currentTarget.value })}
              />
              <button class="btn ghost" onClick={() => browse((p) => patchTls({ keyFile: p }))}>
                Browse…
              </button>
            </div>
            <div class="net-file">
              <input
                class="net-input"
                type="text"
                placeholder="Custom CA bundle (.pem) — blank uses system roots"
                value={tls().caFile}
                onInput={(e) => patchTls({ caFile: e.currentTarget.value })}
              />
              <button class="btn ghost" onClick={() => browse((p) => patchTls({ caFile: p }))}>
                Browse…
              </button>
            </div>
            <label class="net-insecure">
              <input
                type="checkbox"
                checked={tls().insecure}
                onChange={(e) => patchTls({ insecure: e.currentTarget.checked })}
              />
              Skip TLS verification (insecure — like <code>curl -k</code>)
            </label>
          </div>
        </div>
        <Show when={error()}>
          <div class="modal-error">{error()}</div>
        </Show>
        <div class="modal-foot">
          <button class="btn ghost" onClick={props.onClose}>
            Cancel
          </button>
          <button class="btn" onClick={save} disabled={saving()}>
            {saving() ? "Saving…" : "Save"}
          </button>
        </div>
      </div>
    </div>
  );
}
