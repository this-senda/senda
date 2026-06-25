// Wrap a docgen HTML fragment in a themed document for the sandboxed Docs
// preview iframe. Colors come from the app's current CSS theme vars so the
// preview matches light/dark; falls back to dark defaults when unset.
export function docsSrcdoc(body: string): string {
  const cs = getComputedStyle(document.documentElement);
  const v = (name: string, fb: string) => cs.getPropertyValue(name).trim() || fb;
  const text = v("--text", "#e6e8ec");
  const bg = v("--bg", "#111215");
  const accent = v("--accent", "#6e8bff");
  const border = v("--border-soft", "#20242a");
  return `<!DOCTYPE html><meta charset="utf-8"><style>
    body{font:14px/1.6 system-ui,-apple-system,sans-serif;color:${text};background:${bg};margin:0;padding:12px 16px}
    h1,h2,h3{line-height:1.25;margin:1em 0 .4em}
    h1{font-size:1.5em;border-bottom:1px solid ${border};padding-bottom:.3em}
    h2{font-size:1.25em}h3{font-size:1.05em}
    a{color:${accent}}
    code{background:rgba(127,127,127,.18);padding:.1em .35em;border-radius:3px;font-size:.9em}
    pre{background:rgba(127,127,127,.12);padding:.8em;border-radius:5px;overflow:auto}
    pre code{background:none;padding:0}
    table{border-collapse:collapse;margin:.5em 0}
    th,td{border:1px solid ${border};padding:.35em .6em;text-align:left}
    hr{border:none;border-top:1px solid ${border};margin:1.2em 0}
  </style>${body}`;
}
