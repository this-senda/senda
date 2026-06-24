// Client-side variable scope used to highlight {{var}} placeholders and drive
// autocomplete in the URL field. Mirrors the resolution the Go pipeline does,
// minus secrets — those live in *.secret.yaml files server-side and never reach
// the frontend, so isSecret() recognizes them by name instead.
import { collection, environments, activeEnv } from "./store";
import type { KV } from "./api";

// VAR_RE matches {{name}} placeholders (same grammar as internal/vars). A
// leading "$" marks a dynamic faker token (e.g. {{$email}}). Global flag:
// callers using .exec in a loop must reset lastIndex or use matchAll. For
// per-call use, build a fresh regex via varRe().
export function varRe(): RegExp {
  // Mirrors internal/vars: plain {{name}} or faker {{$ns.name(args)}}.
  return /\{\{\s*(\$?[\w.-]+(?:\([^)]*\))?)\s*\}\}/g;
}

const SECRET_KW = ["token", "secret", "password", "passwd", "api_key", "apikey", "client_secret"];

// isSecret reports whether a variable name looks like a credential. Secret
// values are not shipped to the client, so a matching name is treated as
// resolvable-but-masked rather than missing.
export function isSecret(name: string): boolean {
  const k = name.toLowerCase();
  return SECRET_KW.some((kw) => k.includes(kw));
}

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

export type VarStatus = "found" | "secret" | "missing";

export function varStatus(scope: Map<string, string>, name: string): VarStatus {
  if (name.startsWith("$")) return "found"; // dynamic faker token, resolved at send
  if (scope.has(name)) return "found";
  if (isSecret(name)) return "secret";
  return "missing";
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

export type Segment = { text: string; status?: VarStatus; name?: string; value?: string };

// splitVars breaks s into plain runs and {{var}} tokens, tagging each token
// with its resolution status (and name/value) for coloring and pills.
export function splitVars(s: string, scope: Map<string, string>): Segment[] {
  const out: Segment[] = [];
  const re = varRe();
  let last = 0;
  let mtch: RegExpExecArray | null;
  while ((mtch = re.exec(s)) !== null) {
    if (mtch.index > last) out.push({ text: s.slice(last, mtch.index) });
    const name = mtch[1];
    out.push({ text: mtch[0], status: varStatus(scope, name), name, value: scope.get(name) });
    last = mtch.index + mtch[0].length;
  }
  if (last < s.length) out.push({ text: s.slice(last) });
  return out;
}

// triggerAt detects an open {{ autocomplete trigger ending at caret: an
// unclosed "{{" with a name-prefix after it. Returns the prefix start index and
// the typed prefix, or null when not inside a trigger.
export function triggerAt(value: string, caret: number): { start: number; prefix: string } | null {
  const open = value.lastIndexOf("{{", caret);
  if (open < 0) return null;
  // A "}}" between the "{{" and the caret means this placeholder is closed.
  if (value.slice(open, caret).includes("}}")) return null;
  const prefix = value.slice(open + 2, caret);
  if (!/^[\w.$-]*$/.test(prefix)) return null;
  return { start: open + 2, prefix };
}
