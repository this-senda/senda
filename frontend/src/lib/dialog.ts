// In-app replacements for the webview's native confirm()/prompt()/alert(),
// which can't be styled. Each returns a promise the caller awaits; a single
// <Dialog/> host (mounted in App) renders the current request and resolves it.
import { createSignal } from "solid-js";

type DialogKind = "confirm" | "prompt" | "alert";

export type DialogState = {
  kind: DialogKind;
  message: string;
  value: string; // prompt default / current text
  danger: boolean;
  okLabel: string;
  resolve: (result: boolean | string | null | void) => void;
};

const [dialog, setDialog] = createSignal<DialogState | null>(null);
export { dialog, setDialog };

// confirmDialog → true (OK) / false (Cancel). danger styles OK red for deletes.
export function confirmDialog(
  message: string,
  opts: { danger?: boolean; okLabel?: string } = {},
): Promise<boolean> {
  return new Promise((resolve) =>
    setDialog({
      kind: "confirm",
      message,
      value: "",
      danger: opts.danger ?? false,
      okLabel: opts.okLabel ?? "OK",
      resolve: resolve as DialogState["resolve"],
    }),
  );
}

// promptDialog → entered string (OK) / null (Cancel), mirroring window.prompt.
export function promptDialog(message: string, defaultValue = ""): Promise<string | null> {
  return new Promise((resolve) =>
    setDialog({
      kind: "prompt",
      message,
      value: defaultValue,
      danger: false,
      okLabel: "OK",
      resolve: resolve as DialogState["resolve"],
    }),
  );
}

export function alertDialog(message: string): Promise<void> {
  return new Promise((resolve) =>
    setDialog({
      kind: "alert",
      message,
      value: "",
      danger: false,
      okLabel: "OK",
      resolve: resolve as DialogState["resolve"],
    }),
  );
}
