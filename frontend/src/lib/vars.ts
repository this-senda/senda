// Pure {{var}} grammar + status helpers, shared by the URL field and body
// editor. Kept free of store/Wails imports so it (and its tests) don't pull the
// runtime; the store-bound scope builders live in ./scope.
// Secrets live in *.secret.yaml server-side and never reach the frontend, so
// isSecret() recognizes them by name instead.

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

export type VarStatus = "found" | "secret" | "missing";

export function varStatus(scope: Map<string, string>, name: string): VarStatus {
  if (name.startsWith("$")) return "found"; // dynamic faker token, resolved at send
  if (scope.has(name)) return "found";
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
