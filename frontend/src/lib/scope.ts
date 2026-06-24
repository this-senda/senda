// Store-bound variable scope builders, split out of ./vars so the pure grammar
// helpers stay free of store/Wails-runtime imports. Mirrors the Go pipeline's
// resolution (collection then active env, env wins), minus secrets — those live
// in *.secret.yaml server-side and never reach the client.
import { collection, environments, activeEnv } from "./store";
import { isSecret } from "./vars";
import type { KV } from "./api";

// buildScope flattens collection vars then active-environment vars (enabled,
// non-empty key) into a name->value map; env wins on conflicts.
export function buildScope(): Map<string, string> {
  const m = new Map<string, string>();
  const add = (kvs?: KV[]) => {
    for (const kv of kvs ?? []) {
      if (kv.enabled && kv.key) m.set(kv.key, kv.value);
    }
  };
  add(collection()?.vars);
  const env = environments().find((e) => e.name === activeEnv());
  add(env?.vars);
  return m;
}

// VarSource is where a variable's value comes from, surfaced in the pill popover.
export type VarSource = "collection" | "env" | "secret" | "missing";

// buildSources maps each known key to the layer it resolves from (env overrides
// collection, matching buildScope precedence). Secrets live server-side so they
// aren't listed here — sourceOf() infers them by name.
export function buildSources(): Map<string, VarSource> {
  const m = new Map<string, VarSource>();
  for (const kv of collection()?.vars ?? []) {
    if (kv.enabled && kv.key) m.set(kv.key, "collection");
  }
  const env = environments().find((e) => e.name === activeEnv());
  for (const kv of env?.vars ?? []) {
    if (kv.enabled && kv.key) m.set(kv.key, "env");
  }
  return m;
}

export function sourceOf(name: string, sources: Map<string, VarSource>): VarSource {
  if (sources.has(name)) return sources.get(name)!;
  if (isSecret(name)) return "secret";
  return "missing";
}
