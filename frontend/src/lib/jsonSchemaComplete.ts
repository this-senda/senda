// CodeMirror completion source that suggests JSON object keys from an OpenAPI
// requestBody JSON Schema, scoped to the object under the cursor. The schema is
// produced server-side with refs already inlined (App.RequestBodySchema), so no
// $ref resolution is needed here.
import { syntaxTree } from "@codemirror/language";
import type { Completion, CompletionContext, CompletionResult } from "@codemirror/autocomplete";
import type { SyntaxNode } from "@lezer/common";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type JSONSchema = any;

// Arrays are transparent for key completion: keys live on the item schema, so
// step through `items` until we reach a non-array node.
function descendArrays(s: JSONSchema): JSONSchema {
  let cur = s;
  while (cur && cur.type === "array" && cur.items) cur = cur.items;
  return cur;
}

// Walk the schema from root down a path of property names, crossing arrays as
// needed. Returns the schema of the object that path points at, or null.
function resolveSchema(root: JSONSchema, names: string[]): JSONSchema | null {
  let cur = descendArrays(root);
  for (const name of names) {
    const props = cur?.properties;
    if (!props || !(name in props)) return null;
    cur = descendArrays(props[name]);
  }
  return cur ?? null;
}

function enclosingObject(node: SyntaxNode | null): SyntaxNode | null {
  for (let n: SyntaxNode | null = node; n; n = n.parent) {
    if (n.name === "Object") return n;
  }
  return null;
}

// Property-name chain from the document root down to objNode (excludes objNode's
// own keys), so resolveSchema can find the matching schema level.
function pathToObject(doc: string, objNode: SyntaxNode): string[] {
  const names: string[] = [];
  for (let n: SyntaxNode | null = objNode.parent; n; n = n.parent) {
    if (n.name === "Property") {
      const pn = n.getChild("PropertyName");
      if (pn) {
        try { names.unshift(JSON.parse(doc.slice(pn.from, pn.to))); } catch { /* skip */ }
      }
    }
  }
  return names;
}

function existingKeys(doc: string, objNode: SyntaxNode): Set<string> {
  const keys = new Set<string>();
  for (let c = objNode.firstChild; c; c = c.nextSibling) {
    if (c.name === "Property") {
      const pn = c.getChild("PropertyName");
      if (pn) { try { keys.add(JSON.parse(doc.slice(pn.from, pn.to))); } catch { /* skip */ } }
    }
  }
  return keys;
}

function typeLabel(s: JSONSchema): string {
  if (!s) return "";
  if (Array.isArray(s.type)) return s.type.join("|");
  if (s.type === "array" && s.items?.type) return `${s.items.type}[]`;
  return s.type ?? (s.properties ? "object" : "");
}

// makeBodySchemaSource returns a completion source reading the schema live via
// getSchema, so a schema that arrives after the editor mounts still applies.
// Returns null (no completions) whenever there's no schema or the cursor isn't
// at a key position — leaving other sources ({{var}}) unaffected.
export function makeBodySchemaSource(getSchema: () => JSONSchema | undefined) {
  return (ctx: CompletionContext): CompletionResult | null => {
    const schema = getSchema();
    if (!schema) return null;

    const node = syntaxTree(ctx.state).resolveInner(ctx.pos, -1);
    const typingKey = node.name === "PropertyName";
    // Inside a string/number value (and not a key) → user is typing a value.
    if ((node.name === "String" || node.name === "Number") && !typingKey) return null;

    const objNode = enclosingObject(typingKey ? node.parent?.parent ?? null : node);
    if (!objNode) return null;

    const doc = ctx.state.doc.toString();
    const target = resolveSchema(schema, pathToObject(doc, objNode));
    const props = target?.properties;
    if (!props) return null;

    const have = existingKeys(doc, objNode);
    const required: string[] = Array.isArray(target.required) ? target.required : [];
    const word = ctx.matchBefore(/[\w$-]*$/);
    const from = word ? word.from : ctx.pos;
    const quoted = from > 0 && doc[from - 1] === '"';

    const options: Completion[] = [];
    for (const key of Object.keys(props)) {
      if (have.has(key) && !typingKey) continue; // don't re-offer keys already present
      const ps = props[key];
      const req = required.includes(key);
      options.push({
        label: key,
        type: req ? "keyword" : "property",
        detail: [typeLabel(ps), req ? "required" : ""].filter(Boolean).join(" · "),
        apply: quoted ? key : `"${key}": `,
      });
    }
    if (options.length === 0) return null;
    return { from, options };
  };
}
