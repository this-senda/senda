import { createEffect, createResource, createSignal, For, Show } from "solid-js";
import { X, Plus, Trash2 } from "lucide-solid";
import { ICON } from "../lib/icons";
import { api, type SpecOp, type SpecField } from "../lib/api";
import { collection } from "../lib/store";

// Common JSON Schema types offered in the field-type dropdown. "x[]" maps to an
// array of x on save (see backend setFieldType).
const FIELD_TYPES = ["string", "integer", "number", "boolean", "object", "string[]", "integer[]", "number[]", "boolean[]"];

// Structured form editor for the collection's stored OpenAPI specs. Edits one
// operation at a time (summary, description, request-body fields) and writes the
// change back in place — the rest of the document is preserved untouched.
// Method/path editing, parameters, responses and adding operations are not yet
// covered; edit those in the spec file directly.
export default function SpecEditorPanel(props: { onClose: () => void }) {
  const [files] = createResource(() => collection()?.path ?? null, (p) => api.listSpecs(p));
  const [activeFile, setActiveFile] = createSignal("");
  const [text, setText] = createSignal("");
  const [ops, setOps] = createSignal<SpecOp[]>([]);
  const [selectedOp, setSelectedOp] = createSignal("");

  const [summary, setSummary] = createSignal("");
  const [description, setDescription] = createSignal("");
  const [hasBody, setHasBody] = createSignal(false);
  const [fields, setFields] = createSignal<SpecField[]>([]);
  const [method, setMethod] = createSignal("");
  const [path, setPath] = createSignal("");

  const [dirty, setDirty] = createSignal(false);
  const [error, setError] = createSignal("");
  const [saved, setSaved] = createSignal(false);

  const selectOp = async (opId: string) => {
    setSelectedOp(opId);
    setSaved(false);
    setError("");
    try {
      const d = await api.specOperationDetail(text(), opId);
      setMethod(d.method);
      setPath(d.path);
      setSummary(d.summary);
      setDescription(d.description);
      setHasBody(d.hasBody);
      setFields((d.bodyFields ?? []).map((f) => ({ ...f })));
      setDirty(false);
    } catch (e) {
      setError(String(e));
    }
  };

  const loadFile = async (file: string) => {
    const c = collection();
    if (!c) return;
    setActiveFile(file);
    try {
      const raw = await api.readSpec(c.path, file);
      setText(raw);
      const list = (await api.specOperations(raw)) ?? [];
      setOps(list);
      if (list.length > 0) await selectOp(list[0].operationId);
      else setSelectedOp("");
    } catch (e) {
      setError(String(e));
    }
  };

  // Pick the first spec once the list loads.
  createEffect(() => {
    const list = files();
    if (list && list.length > 0 && !activeFile()) void loadFile(list[0]);
  });

  const touch = () => { setDirty(true); setSaved(false); };
  const updateField = (i: number, patch: Partial<SpecField>) => {
    setFields((fs) => fs.map((f, j) => (j === i ? { ...f, ...patch } : f)));
    touch();
  };
  const addField = () => { setFields((fs) => [...fs, { name: "", type: "string", required: false, desc: "" }]); touch(); };
  const removeField = (i: number) => { setFields((fs) => fs.filter((_, j) => j !== i)); touch(); };

  const save = async () => {
    const c = collection();
    if (!c || !selectedOp()) return;
    setError("");
    try {
      const raw = await api.updateSpecOperation(c.path, activeFile(), selectedOp(), {
        operationId: selectedOp(),
        method: method(),
        path: path(),
        summary: summary(),
        description: description(),
        hasBody: hasBody(),
        bodyFields: fields(),
      });
      setText(raw);
      setOps((await api.specOperations(raw)) ?? []);
      setDirty(false);
      setSaved(true);
    } catch (e) {
      setError(String(e));
    }
  };

  return (
    <div class="modal-backdrop" onClick={props.onClose}>
      <div class="modal modal-wide" onClick={(e) => e.stopPropagation()}>
        <div class="modal-head">
          <span class="modal-title">OpenAPI Specs</span>
          <button class="icon-btn" onClick={props.onClose}><X size={ICON.sm} /></button>
        </div>

        <Show
          when={(files()?.length ?? 0) > 0}
          fallback={
            <div class="empty-hint spec-empty">
              No specs yet. Import an OpenAPI document (collection menu → Import) and it's
              saved here, editable, with its requests linked for body schema hints.
            </div>
          }
        >
          <Show when={(files()?.length ?? 0) > 1}>
            <div class="spec-filerow">
              <select class="body-type-select" value={activeFile()} onChange={(e) => void loadFile(e.currentTarget.value)}>
                <For each={files() ?? []}>{(f) => <option value={f}>{f}</option>}</For>
              </select>
            </div>
          </Show>

          <div class="spec-layout">
            <div class="spec-files">
              <For each={ops()}>
                {(o) => (
                  <button
                    class={`spec-file${o.operationId === selectedOp() ? " active" : ""}`}
                    onClick={() => void selectOp(o.operationId)}
                  >
                    <span class={`method method-${o.method.toLowerCase()}`}>{o.method}</span>
                    <span class="spec-op-path">{o.path}</span>
                  </button>
                )}
              </For>
              <Show when={ops().length === 0}>
                <div class="empty-hint">No operations in this spec.</div>
              </Show>
            </div>

            <div class="spec-main">
              <Show when={selectedOp()} fallback={<div class="empty-hint">Select an operation.</div>}>
                <div class="spec-form">
                  <div class="spec-op-head">
                    <span class={`method method-${method().toLowerCase()}`}>{method()}</span>
                    <span class="spec-op-pathbig">{path()}</span>
                  </div>

                  <label class="spec-form-label">Summary
                    <input class="dlg-input" value={summary()} onInput={(e) => { setSummary(e.currentTarget.value); touch(); }} />
                  </label>
                  <label class="spec-form-label">Description
                    <textarea class="spec-textarea" value={description()} onInput={(e) => { setDescription(e.currentTarget.value); touch(); }} />
                  </label>

                  <div class="spec-form-section">
                    Request body
                    <Show when={!hasBody()}><span class="spec-form-sub">no JSON request body</span></Show>
                  </div>
                  <Show when={hasBody()}>
                    <div class="spec-fields">
                      <For each={fields()}>
                        {(f, i) => (
                          <div class="spec-field-row">
                            <input class="dlg-input spec-field-name" placeholder="name" value={f.name} onInput={(e) => updateField(i(), { name: e.currentTarget.value })} />
                            <select class="body-type-select" value={f.type} onChange={(e) => updateField(i(), { type: e.currentTarget.value })}>
                              <For each={FIELD_TYPES.includes(f.type) ? FIELD_TYPES : [f.type, ...FIELD_TYPES]}>
                                {(t) => <option value={t}>{t}</option>}
                              </For>
                            </select>
                            <label class="spec-req" title="Required">
                              <input type="checkbox" checked={f.required} onChange={(e) => updateField(i(), { required: e.currentTarget.checked })} /> req
                            </label>
                            <input class="dlg-input spec-field-desc" placeholder="description" value={f.desc} onInput={(e) => updateField(i(), { desc: e.currentTarget.value })} />
                            <button class="icon-btn" title="Remove field" onClick={() => removeField(i())}><Trash2 size={ICON.xs} /></button>
                          </div>
                        )}
                      </For>
                      <button class="mini-btn" onClick={addField}><Plus size={ICON.xs} /> Add field</button>
                    </div>
                  </Show>
                </div>
              </Show>

              <div class="spec-foot">
                <Show when={error()}><span class="spec-err-count">{error()}</span></Show>
                <Show when={saved()}><span class="spec-saved">saved</span></Show>
                <button class="btn send-btn" onClick={save} disabled={!dirty()}>Save</button>
              </div>
            </div>
          </div>
        </Show>
      </div>
    </div>
  );
}
