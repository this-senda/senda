// Single host for the in-app confirm/prompt/alert modal. Mounted once in App;
// driven by lib/dialog state. Enter confirms, Escape cancels.
import { createEffect, createSignal, Show } from "solid-js";
import { dialog, setDialog } from "../lib/dialog";

export default function Dialog() {
  const [text, setText] = createSignal("");
  let inputRef: HTMLInputElement | undefined;
  let okRef: HTMLButtonElement | undefined;

  // Seed the prompt field and focus the right control whenever a dialog opens.
  createEffect(() => {
    const d = dialog();
    if (!d) return;
    setText(d.value);
    queueMicrotask(() => (d.kind === "prompt" ? inputRef : okRef)?.focus());
  });

  const close = (result: boolean | string | null | void) => {
    const d = dialog();
    setDialog(null);
    d?.resolve(result);
  };
  const onOk = () => {
    const d = dialog();
    if (!d) return;
    close(d.kind === "prompt" ? text() : d.kind === "confirm" ? true : undefined);
  };
  const onCancel = () => {
    const d = dialog();
    if (!d) return;
    close(d.kind === "prompt" ? null : false);
  };
  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      onOk();
    } else if (e.key === "Escape") {
      e.preventDefault();
      onCancel();
    }
  };

  return (
    <Show when={dialog()}>
      {(d) => (
        <div class="dlg-backdrop" onMouseDown={onCancel}>
          <div class="dlg" onMouseDown={(e) => e.stopPropagation()} onKeyDown={onKeyDown}>
            <div class="dlg-msg">{d().message}</div>
            <Show when={d().kind === "prompt"}>
              <input
                class="dlg-input"
                ref={inputRef}
                value={text()}
                onInput={(e) => setText(e.currentTarget.value)}
                spellcheck={false}
              />
            </Show>
            <div class="dlg-actions">
              <Show when={d().kind !== "alert"}>
                <button class="dlg-btn" onClick={onCancel}>Cancel</button>
              </Show>
              <button
                class="dlg-btn dlg-ok"
                classList={{ "dlg-danger": d().danger }}
                ref={okRef}
                onClick={onOk}
              >
                {d().okLabel}
              </button>
            </div>
          </div>
        </div>
      )}
    </Show>
  );
}
