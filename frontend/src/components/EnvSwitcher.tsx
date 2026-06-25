// Active-environment selector in the top bar: a custom pill dropdown showing a
// per-env colour dot, the active env name, and a chevron. The gear opens the
// environment editor. Persists the choice across sessions.
import { createMemo, createSignal, For, onCleanup, onMount, Show } from "solid-js";
import { Check, ChevronDown, Settings } from "lucide-solid";
import { ICON } from "../lib/icons";
import { activeEnv, collection, environments, rememberEnv, setActiveEnv } from "../lib/store";
import { envColor } from "../lib/envColor";
import EnvEditor from "./EnvEditor";

export default function EnvSwitcher() {
  const [open, setOpen] = createSignal(false);
  const [showEditor, setShowEditor] = createSignal(false);
  let ref!: HTMLDivElement;

  const onDoc = (e: MouseEvent) => {
    if (ref && !ref.contains(e.target as Node)) setOpen(false);
  };
  onMount(() => document.addEventListener("mousedown", onDoc));
  onCleanup(() => document.removeEventListener("mousedown", onDoc));

  const current = createMemo(() => environments().find((e) => e.name === activeEnv()));

  const pick = (name: string) => {
    setActiveEnv(name);
    rememberEnv(name);
    setOpen(false);
  };

  return (
    <div class="env-switcher">
      <div class="env-select" ref={ref}>
        <button
          class="env-pill"
          onClick={() => setOpen(!open())}
          onKeyDown={(e) => e.key === "Escape" && setOpen(false)}
        >
          <span class="env-dot" style={{ background: envColor(current()?.color) }} />
          <span class="env-name">{activeEnv() || "none"}</span>
          <ChevronDown size={ICON.sm} class="env-chevron" />
        </button>
        <Show when={open()}>
          <div class="env-menu">
            <button class="env-opt" classList={{ selected: activeEnv() === "" }} onClick={() => pick("")}>
              <span class="env-dot" style={{ background: envColor(undefined) }} />
              <span class="env-opt-label">No environment</span>
              <Show when={activeEnv() === ""}>
                <Check size={14} class="env-check" />
              </Show>
            </button>
            <For each={environments()}>
              {(env) => (
                <button
                  class="env-opt"
                  classList={{ selected: env.name === activeEnv() }}
                  onClick={() => pick(env.name)}
                >
                  <span class="env-dot" style={{ background: envColor(env.color) }} />
                  <span class="env-opt-label">{env.name}</span>
                  <Show when={env.name === activeEnv()}>
                    <Check size={14} class="env-check" />
                  </Show>
                </button>
              )}
            </For>
          </div>
        </Show>
      </div>
      <span class="env-divider" />
      <button
        class="icon-btn"
        title="Edit environments"
        disabled={!collection()}
        onClick={() => setShowEditor(true)}
      >
        <Settings size={ICON.xxl} />
      </button>
      <Show when={showEditor()}>
        <EnvEditor onClose={() => setShowEditor(false)} />
      </Show>
    </div>
  );
}
