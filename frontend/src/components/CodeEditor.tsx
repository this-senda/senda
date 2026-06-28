// CodeMirror 6 wrapper. CM6 only renders the visible viewport, so large
// payloads stay smooth. Used for request bodies and response display.
import { createEffect, onCleanup, onMount } from "solid-js";
import { EditorState } from "@codemirror/state";
import { EditorView, keymap, lineNumbers, tooltips } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import {
  autocompletion,
  completionKeymap,
  type Completion,
  type CompletionContext,
  type CompletionResult,
} from "@codemirror/autocomplete";
import { json } from "@codemirror/lang-json";
import { markdown } from "@codemirror/lang-markdown";
import { buildScope, triggerAt } from "../lib/vars";
import { fakerTokens } from "../lib/faker";
import { makeBodySchemaSource } from "../lib/jsonSchemaComplete";
import { graphql, updateSchema } from "cm6-graphql";
import type { GraphQLSchema } from "graphql";
import { HighlightStyle, syntaxHighlighting, bracketMatching } from "@codemirror/language";
import { tags as t } from "@lezer/highlight";

type Props = {
  value: string;
  language?: "json" | "text" | "graphql" | "markdown";
  readOnly?: boolean;
  onChange?: (v: string) => void;
  // GraphQL only: when set, enables schema-aware validation + autocomplete.
  // Syntax linting runs even without it.
  schema?: GraphQLSchema;
  // When set, typing "{{" offers collection/env variable completions. Used for
  // request bodies. Reads the client scope (secrets excluded, server-side).
  varComplete?: boolean;
  // When the request is linked to an OpenAPI operation, its requestBody JSON
  // Schema (refs inlined). Drives JSON-body key autocomplete. Read live so a
  // schema that loads after mount still applies.
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  bodySchema?: any;
};

// varCompletionSource feeds {{var}} names into CM autocomplete. Fires only
// inside an open, unclosed "{{" (triggerAt). Reads buildScope() at completion
// time so it always reflects the current collection + active env.
// ponytail: client scope only (no folder vars / secrets) — enough for bodies;
// switch to api.resolveScope like UrlField if folder-scoped vars matter.
// applyToken replaces the whole {{ token (typed prefix + any trailing
// name/braces) with `insert` + closing braces, so re-picking inside a closed
// {{x}} doesn't duplicate the braces. Caret lands just after "}}".
function applyToken(insert: string) {
  return (view: EditorView, _c: unknown, from: number) => {
    const after = view.state.doc.sliceString(from, from + 64);
    const eaten = after.match(/^[\w.$-]*\}{0,2}/)?.[0].length ?? 0;
    view.dispatch({
      changes: { from, to: from + eaten, insert: insert + "}}" },
      selection: { anchor: from + insert.length + 2 },
    });
  };
}

function varCompletionSource(ctx: CompletionContext): CompletionResult | null {
  const trig = triggerAt(ctx.state.doc.toString(), ctx.pos);
  if (!trig) return null;
  const scope = buildScope();
  const options: Completion[] = [...scope.keys()].sort().map((name) => ({
    label: name,
    type: "variable",
    detail: scope.get(name),
    apply: applyToken(name),
  }));
  // Faker tokens ({{$person.firstname}}…): only once the user starts a "$"
  // token, so plain {{ doesn't dump 300+ entries. Grouped by category via
  // `section`; gofakeit's own example shown as detail. CM does final filtering.
  if (trig.prefix.startsWith("$")) {
    for (const f of fakerTokens()) {
      const token = "$" + f.category + "." + f.name;
      options.push({
        label: token,
        type: "function",
        detail: f.example,
        section: f.category,
        apply: applyToken(token),
      });
    }
  }
  if (options.length === 0) return null;
  return { from: trig.start, options };
}

const theme = EditorView.theme(
  {
    "&": { backgroundColor: "transparent", height: "100%", fontSize: "13px" },
    ".cm-content": { fontFamily: "var(--mono)", caretColor: "var(--text)" },
    ".cm-gutters": {
      backgroundColor: "transparent",
      color: "var(--text-faint)",
      border: "none",
    },
    ".cm-activeLine": { backgroundColor: "var(--hover)" },
    ".cm-matchingBracket": {
      backgroundColor: "var(--accent-dim)",
      outline: "1px solid var(--accent)",
      borderRadius: "2px",
    },
    ".cm-nonmatchingBracket": { color: "var(--err)" },
    ".cm-activeLineGutter": { backgroundColor: "transparent" },
    "&.cm-focused": { outline: "none" },
    ".cm-tooltip": {
      backgroundColor: "var(--bg-elev2)",
      border: "1px solid var(--border)",
      borderRadius: "8px",
      color: "var(--text)",
      boxShadow: "0 8px 28px rgba(0,0,0,0.4)",
      overflow: "hidden",
    },
    ".cm-tooltip-autocomplete": { padding: "4px" },
    ".cm-tooltip-autocomplete > ul": {
      maxHeight: "20em",
      fontFamily: "var(--mono)",
      fontSize: "12.5px",
    },
    // Each row: icon · label · (example pushed right).
    ".cm-tooltip-autocomplete > ul > li": {
      display: "flex",
      alignItems: "center",
      gap: "8px",
      padding: "5px 9px",
      borderRadius: "5px",
      lineHeight: "1.4",
    },
    ".cm-tooltip-autocomplete > ul > li[aria-selected]": {
      backgroundColor: "var(--accent)",
      color: "var(--accent-fg)",
    },
    ".cm-completionLabel": { flex: "1 1 auto" },
    ".cm-completionMatchedText": {
      color: "var(--accent)",
      fontWeight: "700",
      textDecoration: "none",
    },
    "li[aria-selected] .cm-completionMatchedText": { color: "var(--accent-fg)" },
    // The faker example / var value, dimmed and right-aligned.
    ".cm-completionDetail": {
      marginLeft: "auto",
      paddingLeft: "12px",
      fontStyle: "normal",
      fontSize: "11px",
      color: "var(--text-faint)",
      maxWidth: "16em",
      overflow: "hidden",
      textOverflow: "ellipsis",
      whiteSpace: "nowrap",
    },
    "li[aria-selected] .cm-completionDetail": { color: "var(--accent-fg)", opacity: "0.8" },
    ".cm-completionIcon": {
      width: "1.1em",
      opacity: "0.7",
      color: "var(--text-dim)",
      fontSize: "90%",
    },
    "li[aria-selected] .cm-completionIcon": { color: "var(--accent-fg)" },
    // Sticky category header between groups.
    ".cm-completionSection": {
      position: "sticky",
      top: "0",
      padding: "5px 9px 3px",
      marginTop: "2px",
      fontFamily: "var(--mono)",
      fontSize: "10px",
      fontWeight: "700",
      color: "var(--text-faint)",
      textTransform: "uppercase",
      letterSpacing: "0.06em",
      backgroundColor: "var(--bg-elev2)",
    },
  },
  { dark: true }
);

// Token colours for JSON + GraphQL, drawn from the senda palette so the editor
// stops rendering flat monochrome. Tags double up (e.g. propertyName covers
// JSON keys and GraphQL fields).
const highlight = syntaxHighlighting(
  HighlightStyle.define([
    { tag: [t.keyword, t.operatorKeyword], color: "var(--syn-keyword)" }, // query/mutation/fragment, true/false/null
    { tag: [t.string, t.special(t.string)], color: "var(--syn-string)" },
    { tag: [t.number, t.bool, t.null], color: "var(--syn-number)" },
    { tag: [t.propertyName, t.definition(t.propertyName)], color: "var(--syn-property)" }, // JSON keys, GraphQL fields
    { tag: [t.typeName, t.className, t.namespace], color: "var(--syn-type)" },
    { tag: [t.variableName, t.atom, t.labelName], color: "var(--syn-variable)" },
    { tag: [t.comment, t.lineComment, t.blockComment], color: "var(--text-faint)", fontStyle: "italic" },
    { tag: [t.punctuation, t.brace, t.bracket, t.separator], color: "var(--text-dim)" },
    // Markdown (docs editor). Heading levels are distinct tags, so list them.
    { tag: [t.heading1, t.heading2, t.heading3, t.heading4, t.heading5, t.heading6], color: "var(--syn-keyword)", fontWeight: "700" },
    { tag: t.strong, fontWeight: "700", color: "var(--syn-property)" },
    { tag: t.emphasis, fontStyle: "italic" },
    { tag: t.strikethrough, textDecoration: "line-through" },
    { tag: t.monospace, color: "var(--syn-string)" },
    { tag: [t.link, t.url], color: "var(--syn-variable)", textDecoration: "underline" },
    { tag: t.quote, color: "var(--text-dim)", fontStyle: "italic" },
    { tag: t.processingInstruction, color: "var(--text-faint)" }, // markup chars: # * - > `
  ]),
);

export default function CodeEditor(props: Props) {
  let host!: HTMLDivElement;
  let view: EditorView | undefined;

  onMount(() => {
    // Read-only (response) editors skip line-wrapping + history: uniform line
    // height lets CM6 use fast fixed-height virtualization (no per-scroll
    // re-measure), and there's nothing to undo. Editable bodies keep both.
    const ro = !!props.readOnly;
    const extensions = [
      lineNumbers(),
      theme,
      highlight,
      bracketMatching(),
      // Reparent popups to <body> so the autocomplete dropdown isn't clipped by
      // the editor pane's overflow.
      tooltips({ parent: document.body }),
      EditorState.readOnly.of(ro),
      ...(ro
        ? []
        : [
            history(),
            autocompletion(
              props.varComplete
                ? { override: [varCompletionSource, makeBodySchemaSource(() => props.bodySchema)] }
                : undefined,
            ),
            keymap.of([...completionKeymap, ...defaultKeymap, ...historyKeymap]),
            EditorView.lineWrapping,
          ]),
      ...(props.language === "json" ? [json()] : []),
      ...(props.language === "graphql" ? graphql(props.schema) : []),
      ...(props.language === "markdown" ? [markdown()] : []),
      EditorView.updateListener.of((u) => {
        if (u.docChanged && props.onChange) {
          props.onChange(u.state.doc.toString());
        }
      }),
    ];

    view = new EditorView({
      state: EditorState.create({ doc: props.value, extensions }),
      parent: host,
    });
  });

  // Sync external value changes (e.g. new response, request switch) into CM.
  createEffect(() => {
    const next = props.value;
    if (!view) return;
    const current = view.state.doc.toString();
    if (current !== next) {
      view.dispatch({
        changes: { from: 0, to: current.length, insert: next },
      });
    }
  });

  // Push a freshly introspected schema into the live GraphQL editor so lint +
  // autocomplete pick it up without rebuilding the view.
  createEffect(() => {
    const s = props.schema;
    if (!view || props.language !== "graphql") return;
    updateSchema(view, s);
  });

  onCleanup(() => view?.destroy());

  return <div class="code-editor" ref={host} />;
}
