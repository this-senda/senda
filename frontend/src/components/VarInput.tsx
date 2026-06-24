// Plain text input with a "{{" autocomplete popup for {{var}} + faker tokens.
// Used by KVEditor (params / headers / form fields). Shares the dropdown look
// with UrlField (.url-ac) but is self-contained on a value/onChange pair.
// ponytail: client scope only (buildScope, no folder vars / secrets) — same as
// the body editor; switch to api.resolveScope if folder-scoped vars matter here.
import { createSignal, For, Show } from "solid-js";
import { buildScope, triggerAt } from "../lib/vars";
import { fakerTokens } from "../lib/faker";

type Item = { label: string; detail: string };

type Props = {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  class?: string;
};

export default function VarInput(props: Props) {
  let inputRef: HTMLInputElement | undefined;
  const [open, setOpen] = createSignal(false);
  const [start, setStart] = createSignal(0);
  const [items, setItems] = createSignal<Item[]>([]);
  const [idx, setIdx] = createSignal(0);

  const caret = () => inputRef?.selectionStart ?? props.value.length;
  const close = () => setOpen(false);

  const refresh = () => {
    const trig = triggerAt(props.value, caret());
    if (!trig) return close();
    const scope = buildScope();
    const list: Item[] = [...scope.keys()]
      .filter((k) => k.startsWith(trig.prefix))
      .sort()
      .map((k) => ({ label: k, detail: scope.get(k) ?? "" }));
    // Faker tokens only once a "$" token starts, so plain {{ stays short.
    if (trig.prefix.startsWith("$")) {
      for (const f of fakerTokens()) {
        list.push({ label: "$" + f.category + "." + f.name, detail: f.example });
      }
    }
    if (list.length === 0) return close();
    setStart(trig.start);
    setItems(list);
    setIdx(0);
    setOpen(true);
  };

  // Replace the {{ token being completed with the chosen name + closing braces,
  // eating any leftover name/braces to the right so re-picking doesn't dupe.
  const accept = (label: string) => {
    if (!inputRef) return;
    const rest = props.value.slice(caret()).replace(/^[\w.$-]*\}{0,2}/, "");
    const next = props.value.slice(0, start()) + label + "}}" + rest;
    props.onChange(next);
    close();
    const pos = start() + label.length + 2;
    queueMicrotask(() => {
      inputRef!.focus();
      inputRef!.setSelectionRange(pos, pos);
    });
  };

  const onKeyDown = (e: KeyboardEvent) => {
    if (!open()) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setIdx((i) => (i + 1) % items().length);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setIdx((i) => (i - 1 + items().length) % items().length);
    } else if (e.key === "Enter" || e.key === "Tab") {
      e.preventDefault();
      accept(items()[idx()].label);
    } else if (e.key === "Escape") {
      e.preventDefault();
      close();
    }
  };

  return (
    <div class="var-input">
      <input
        ref={inputRef}
        class={props.class}
        placeholder={props.placeholder}
        value={props.value}
        spellcheck={false}
        autocomplete="off"
        onInput={(e) => {
          props.onChange(e.currentTarget.value);
          refresh();
        }}
        onClick={refresh}
        onKeyUp={(e) => {
          if (["ArrowLeft", "ArrowRight", "Home", "End"].includes(e.key)) refresh();
        }}
        onKeyDown={onKeyDown}
        onBlur={() => setTimeout(close, 120)}
      />
      <Show when={open()}>
        <ul class="url-ac">
          <For each={items()}>
            {(it, i) => (
              <li
                classList={{ active: i() === idx() }}
                onMouseDown={(e) => {
                  e.preventDefault();
                  accept(it.label);
                }}
                onMouseEnter={() => setIdx(i())}
              >
                <span class="url-ac-name">{it.label}</span>
                <span class="url-ac-val">{it.detail}</span>
              </li>
            )}
          </For>
        </ul>
      </Show>
    </div>
  );
}
