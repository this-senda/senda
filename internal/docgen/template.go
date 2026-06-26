package docgen

import (
	"html/template"
	"strings"
)

var siteTmpl = template.Must(template.New("site").Funcs(template.FuncMap{
	"lower": strings.ToLower,
	"sub":   func(a, b int) int { return a - b },
}).Parse(siteHTML))

// Lucide line icons (stroke=currentColor) — kept inline so the page stays a
// single self-contained file with no icon font or sprite sheet.
const (
	icnSearch  = `<svg class="icn" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>`
	icnDiamond = `<svg class="icn dia" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2.7 10.3a2.41 2.41 0 0 0 0 3.41l7.59 7.59a2.41 2.41 0 0 0 3.41 0l7.59-7.59a2.41 2.41 0 0 0 0-3.41L13.7 2.71a2.41 2.41 0 0 0-3.41 0Z"/></svg>`
)

const siteHTML = `<!DOCTYPE html>
<html lang="en" data-theme="light">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}}</title>
<style>
:root{
  --bg:#ffffff;--panel:#f6f6f9;--panel-2:#ececf0;--border:#e8e8ec;
  --text:#1a1a22;--muted:#52525b;--faint:#9b9ba6;--accent:#6366f1;
  --code-bg:#14151d;--code-panel:#191b24;--code-text:#e6edf3;--code-border:#262a36;--code-faint:#8b8fa3;
  --pill-bg:#ede9fe;--pill-text:#6d28d9;--req:#f472b6;--sidebar-w:286px;--head-h:71px;
  --c-key:#c4b5fd;--c-str:#86efac;--c-num:#fca5a5;--c-fn:#93c5fd;
}
:root[data-theme="dark"]{
  --bg:#0b0d12;--panel:#11141b;--panel-2:#161a23;--border:#1e2230;
  --text:#e8eaf0;--muted:#aab0c0;--faint:#6b7280;--accent:#818cf8;
  --pill-bg:#2a2152;--pill-text:#c4b5fd;
}
*{box-sizing:border-box}
html,body{margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Inter,Roboto,sans-serif;
  background:var(--bg);color:var(--text);font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased}
code,pre,.mono{font-family:ui-monospace,'SF Mono',Menlo,Consolas,monospace}
a{color:inherit;text-decoration:none}
.icn{width:15px;height:15px;flex:none}

.m{font-family:ui-monospace,monospace;font-size:11px;font-weight:700;letter-spacing:.03em}
.m-get{color:#2563eb}.m-post{color:#16a34a}.m-put{color:#d97706}
.m-patch{color:#7c3aed}.m-delete{color:#e11d48}.m-head,.m-options{color:#0891b2}

.grid{display:grid;grid-template-columns:var(--sidebar-w) minmax(0,1fr);grid-template-rows:var(--head-h) 1fr;min-height:100vh;max-width:1760px;margin:0 auto}

.brand-cell{grid-column:1;grid-row:1;position:sticky;top:0;z-index:30;display:flex;align-items:center;gap:11px;
  padding:0 18px;background:var(--bg);border-right:1px solid var(--border);border-bottom:1px solid var(--border)}
.logo{width:36px;height:36px;flex:none;border-radius:10px;background:linear-gradient(135deg,#818cf8,#6366f1);
  display:flex;align-items:center;justify-content:center}
.logo::after{content:"";width:13px;height:13px;border-radius:50%;border:2.5px solid #fff}
.brand-name{font-weight:600;font-size:16px;line-height:1.15;letter-spacing:-.01em}
.brand-sub{font-size:11px;color:var(--faint);letter-spacing:.04em;margin-top:1px}

.head-cell{grid-column:2;grid-row:1;position:sticky;top:0;z-index:30;display:flex;align-items:center;gap:14px;
  padding:0 40px;background:var(--bg);border-bottom:1px solid var(--border)}
.head-cell .host{font-family:ui-monospace,monospace;font-size:13px;color:var(--muted)}
.pill{font-size:11px;font-weight:600;padding:3px 9px;border-radius:99px;background:var(--pill-bg);color:var(--pill-text)}
.head-cell .spacer{flex:1}
.toggle{display:flex;align-items:center;gap:7px;font-size:13px;font-weight:500;padding:6px 13px;
  border:1px solid var(--border);border-radius:9px;background:var(--bg);color:var(--text);cursor:pointer}
.toggle:hover{background:var(--panel)}
.toggle .icn{width:14px;height:14px}

.sidebar{grid-column:1;grid-row:2;position:sticky;top:var(--head-h);align-self:start;
  height:calc(100vh - var(--head-h));overflow-y:auto;padding:20px 16px 32px;border-right:1px solid var(--border)}
.search{display:flex;align-items:center;gap:8px;margin:0 2px 22px;padding:0 11px;
  border:1px solid var(--border);border-radius:10px;background:var(--panel);color:var(--faint)}
.search .icn{width:14px;height:14px}
.search input{flex:1;border:0;background:transparent;outline:none;color:var(--text);font:inherit;font-size:13px;padding:9px 0}
.search input::placeholder{color:var(--faint)}
.search .kbd{font-size:11px;padding:1px 6px;border-radius:5px;background:var(--bg);border:1px solid var(--border);color:var(--faint)}
.navgroup{margin-bottom:20px}
.navgroup h4{margin:0 8px 8px;font-size:11px;font-weight:600;letter-spacing:.08em;text-transform:uppercase;color:var(--faint)}
.navlink{display:flex;align-items:center;gap:10px;padding:6px 10px;border-radius:8px;font-size:14px;color:var(--muted);cursor:pointer}
.navlink:hover{background:var(--panel)}
.navlink.active{background:var(--pill-bg);color:var(--accent);font-weight:500}
.navlink .m{width:38px;flex:none}
.navlink .dia{width:13px;height:13px;color:var(--faint)}
.foot{margin:6px 8px 0;padding-top:16px;border-top:1px solid var(--border);font-size:13px;color:var(--faint);display:flex;align-items:center;gap:8px}
.foot .live{width:7px;height:7px;border-radius:99px;background:#22c55e}

main{grid-column:2;grid-row:2;padding:42px 48px 140px;min-width:0}
section{scroll-margin-top:90px;margin-bottom:0;padding-bottom:0}
.sec-divider{border:0;border-top:1px solid var(--border);margin:32px 0}
.eyebrow{font-size:12px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--accent)}
h1{font-size:44px;line-height:1.08;font-weight:800;margin:12px 0 20px;letter-spacing:-.025em}
h2{font-size:28px;font-weight:700;margin:0 0 12px;letter-spacing:-.015em}
h3{font-size:11px;font-weight:700;color:var(--faint);text-transform:uppercase;letter-spacing:.08em;margin:26px 0 6px}
.lead{font-size:17px;color:var(--muted);max-width:64ch}
.lead code,.prose code{background:var(--panel);padding:.12em .42em;border-radius:5px;font-size:.85em;color:var(--text)}
.split{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,480px);gap:48px;align-items:start}
.prose{min-width:0}
.prose p{margin:0 0 14px}

.flow{display:flex;align-items:center;gap:11px;flex-wrap:wrap;margin:26px 0 8px}
.flow .card{display:flex;align-items:baseline;gap:9px;border:1px solid var(--border);border-radius:9px;padding:9px 14px;background:var(--bg)}
.flow .card .p{font-family:ui-monospace,monospace;font-size:13px;color:var(--muted)}
.flow .more{color:var(--muted);font-size:13px;text-decoration:none}.flow .more:hover{border-color:var(--accent);color:var(--accent)}

.route-box{display:flex;align-items:center;gap:12px;width:100%;border:1px solid var(--border);border-left:3px solid var(--faint);
  border-radius:10px;padding:11px 16px;background:var(--bg);margin:0 0 18px}
.route-box.r-get{border-left-color:#2563eb}.route-box.r-post{border-left-color:#16a34a}
.route-box.r-put{border-left-color:#d97706}.route-box.r-patch{border-left-color:#7c3aed}.route-box.r-delete{border-left-color:#e11d48}
.route-box .path{font-family:ui-monospace,monospace;font-size:14px;color:var(--muted)}
.ep-name{font-size:21px;font-weight:700;letter-spacing:-.01em;margin:0 0 12px}
.summary{font-size:18px;font-weight:700;margin:18px 0 8px;letter-spacing:-.01em}

.param{padding:14px 0;border-bottom:1px solid var(--border)}
.param:last-child{border-bottom:0}
.pname{font-family:ui-monospace,monospace;font-weight:600;font-size:14px;color:var(--text)}
.ptype{font-family:ui-monospace,monospace;font-size:13px;color:var(--faint);margin-left:9px}
.preq{font-size:10px;font-weight:600;letter-spacing:.03em;text-transform:uppercase;margin-left:9px;color:var(--req)}
.popt{font-size:10px;font-weight:600;letter-spacing:.03em;text-transform:uppercase;margin-left:9px;color:var(--faint)}
.pdesc{color:var(--muted);margin-top:5px;font-size:14px}

table{width:100%;border-collapse:collapse;margin:4px 0 16px;font-size:14px}
th,td{text-align:left;padding:9px 12px;border-bottom:1px solid var(--border);vertical-align:top}
th{font-size:11px;text-transform:uppercase;letter-spacing:.05em;color:var(--faint);font-weight:600}
td code{font-family:ui-monospace,monospace;font-size:13px;color:var(--accent)}

.code{background:var(--code-bg);border:1px solid var(--code-border);border-radius:12px;overflow:hidden;position:sticky;top:calc(var(--head-h) + 20px)}
.code+.code{margin-top:18px}
.bar{display:flex;align-items:center;gap:7px;padding:11px 15px;background:var(--code-panel);border-bottom:1px solid var(--code-border)}
.bar .dot{width:11px;height:11px;border-radius:99px;flex:none}
.bar .t{margin-left:6px;font-family:ui-monospace,monospace;font-size:12px;color:var(--code-faint)}
.bar .spacer{flex:1}
.tabs{display:flex;gap:2px}
.tab{font-family:ui-monospace,monospace;font-size:12px;color:var(--code-faint);background:transparent;border:0;padding:3px 9px;border-radius:6px;cursor:pointer}
.tab.active{color:#fff;background:rgba(255,255,255,.08)}
.resp-bar{justify-content:space-between}
.resp-bar .status{display:flex;align-items:center;gap:7px;font-family:ui-monospace,monospace;font-size:12px;color:var(--code-text)}
.resp-bar .status .live{width:7px;height:7px;border-radius:99px;background:#22c55e}
.resp-bar .status.err{color:#fca5a5}.resp-bar .status.err .live{background:#f87171}
.reset-btn{font-family:ui-monospace,monospace;font-size:12px;color:var(--code-faint);background:transparent;border:1px solid var(--code-border);border-radius:6px;padding:2px 9px;cursor:pointer}
.reset-btn:hover{color:#fff;border-color:var(--accent)}
.code pre{margin:0;padding:16px;overflow:auto;max-height:65vh;font-size:13px;line-height:1.6;color:var(--code-text);white-space:pre}
.code pre[hidden]{display:none}
.inline-code{background:var(--code-bg);border:1px solid var(--code-border);border-radius:11px;margin:6px 0 0;overflow:hidden}
.inline-code pre{margin:0;padding:16px;overflow-x:auto;font-size:13px;line-height:1.6;color:var(--code-text)}

/* Try it — live browser sends, on by default */
.send-btn{font-family:ui-monospace,monospace;font-size:12px;font-weight:600;color:#fff;background:var(--accent);
  border:0;border-radius:6px;padding:3px 12px;cursor:pointer;align-items:center}
.send-btn:hover{filter:brightness(1.08)}
.try-url-row{padding:10px 15px;border-bottom:1px solid var(--code-border);background:var(--code-panel)}
.try-url{width:100%;font-family:ui-monospace,monospace;font-size:12px;color:var(--code-text);
  background:#0b0c12;border:1px solid var(--code-border);border-radius:6px;padding:6px 9px;outline:none}
.try-url:focus{border-color:var(--accent)}
.live-resp{border-top:1px solid var(--code-border)}
.live-status{padding:9px 15px;font-family:ui-monospace,monospace;font-size:12px;color:#86efac;background:var(--code-panel)}
.live-status.err{color:#fca5a5}
.live-resp pre{margin:0;padding:16px;overflow:auto;max-height:60vh;font-size:13px;line-height:1.6;color:var(--code-text)}
.j-key{color:#7dd3fc}.j-str{color:#86efac}.j-num{color:#fca5a5}.j-kw{color:#c4b5fd}.j-punc{color:var(--code-faint)}

@media(max-width:700px){
  .grid{grid-template-columns:1fr;grid-template-rows:var(--head-h) 1fr}
  .brand-cell,.sidebar{display:none}
  .head-cell{grid-column:1}main{grid-column:1;padding:28px 22px 80px}
  .split{grid-template-columns:1fr}
}
</style>
</head>
<body>
<div class="grid">
  <div class="brand-cell">
    <div class="logo"></div>
    <div>
      <div class="brand-name">{{.Brand}}</div>
      <div class="brand-sub">API{{if .Version}} · {{.Version}}{{end}}</div>
    </div>
  </div>
  <header class="head-cell">
    <span class="host">{{if .Host}}{{.Host}}{{else}}{{.Brand}}{{end}}</span>
    {{if .Version}}<span class="pill">{{.Version}} · stable</span>{{end}}
    <span class="spacer"></span>
    <button class="toggle" onclick="toggleTheme()"><span id="theme-icn"></span><span id="theme-label">Dark</span></button>
  </header>

  <nav class="sidebar">
    <div class="search">` + icnSearch + `<input id="nav-search" type="text" placeholder="Search" autocomplete="off"><span class="kbd">⌘K</span></div>
    <div id="nav-groups">
    {{range .Groups}}
    <div class="navgroup">
      <h4>{{.Title}}</h4>
      {{range .Links}}
      <a class="navlink" href="#{{.ID}}">
        {{if .Method}}<span class="m m-{{lower .Method}}">{{.Method}}</span>
        {{else if .Diamond}}` + icnDiamond + `{{end}}
        <span class="nav-label">{{.Label}}</span>
      </a>
      {{end}}
    </div>
    {{end}}
    </div>
    <div class="foot"><span class="live"></span>Generated by Senda</div>
  </nav>

  <main>
    <section id="introduction">
      <div class="eyebrow">Introduction</div>
      <h1>{{.Title}}</h1>
      {{if .Intro}}<div class="lead prose">{{.Intro}}</div>{{end}}
      {{if .Items}}
      <div class="flow">
        {{range $i, $e := .Items}}{{if lt $i 6}}
        <a class="card" href="#{{$e.ID}}"><span class="m m-{{lower $e.Method}}">{{$e.Method}}</span><span class="p">{{$e.Path}}</span></a>
        {{end}}{{end}}
        {{if gt (len .Items) 6}}<a class="card more" href="#{{(index .Items 6).ID}}">+{{sub (len .Items) 6}} more</a>{{end}}
      </div>
      {{end}}
    </section>
    <hr class="sec-divider">

    {{if .HasAuth}}
    <section id="authentication">
      <div class="split">
        <div class="prose">
          <h2>Authentication</h2>
          {{.AuthDesc}}
        </div>
        {{if .AuthCurl}}
        <div class="code">
          <div class="bar"><span class="dot" style="background:#ec6a5e"></span><span class="dot" style="background:#f3bf4f"></span><span class="dot" style="background:#61c554"></span><span class="t">Authenticated request</span></div>
          <pre>{{.AuthCurl}}</pre>
        </div>
        {{end}}
      </div>
    </section>
    <hr class="sec-divider">
    {{end}}

    {{range .Items}}
    <section id="{{.ID}}">
      <div class="split">
        <div class="prose">
          {{if .Name}}<h2 class="ep-name">{{.Name}}</h2>{{end}}
          <div class="route-box r-{{lower .Method}}"><span class="m m-{{lower .Method}}">{{.Method}}</span><span class="path">{{.Path}}</span></div>
          {{if .Docs}}<div class="ep-docs">{{.Docs}}</div>{{end}}
          {{if .PathParams}}
          <h3>Path Parameters</h3>
          {{range .PathParams}}{{template "param" .}}{{end}}
          {{end}}
          {{if .Params}}
          <h3>Query Parameters</h3>
          {{range .Params}}{{template "param" .}}{{end}}
          {{end}}
          {{if .BodyParams}}
          <h3>Body Parameters</h3>
          {{range .BodyParams}}{{template "param" .}}{{end}}
          {{end}}
          {{if .Headers}}
          <h3>Headers</h3>
          {{range .Headers}}<div class="param"><span class="pname">{{.Key}}</span>{{if .Value}}<span class="ptype">{{.Value}}</span>{{end}}{{if .Desc}}<div class="pdesc">{{.Desc}}</div>{{end}}</div>{{end}}
          {{end}}
          {{if .Asserts}}
          <h3>Assertions</h3>
          <table><thead><tr><th>Target</th><th>Op</th><th>Expected</th></tr></thead><tbody>
          {{range .Asserts}}<tr><td><code>{{.Target}}</code></td><td class="mono">{{.Op}}</td><td class="mono">{{.Value}}</td></tr>{{end}}
          </tbody></table>
          {{end}}
        </div>
        <div>
          {{if .Curl}}
          <div class="code" data-req="{{.ReqJSON}}">
            <div class="bar"><span class="dot" style="background:#ec6a5e"></span><span class="dot" style="background:#f3bf4f"></span><span class="dot" style="background:#61c554"></span><span class="t">Request</span><span class="spacer"></span><button class="send-btn" onclick="sendReq(this.closest('.code'))">▶ Send</button><span class="tabs"><button class="tab active" data-lang="curl">cURL</button>{{if .JS}}<button class="tab" data-lang="js">JS</button>{{end}}{{if .Python}}<button class="tab" data-lang="python">Python</button>{{end}}</span></div>
            <div class="try-url-row"><input class="try-url" value="{{.URL}}" spellcheck="false"></div>
            <pre data-lang="curl">{{.Curl}}</pre>
            {{if .JS}}<pre data-lang="js" hidden>{{.JS}}</pre>{{end}}
            {{if .Python}}<pre data-lang="python" hidden>{{.Python}}</pre>{{end}}
            <div class="live-resp" hidden></div>
          </div>
          {{end}}
          {{if .RespExample}}
          <div class="code resp-panel" data-label="{{if .RespStatus}}{{.RespStatus}}{{else}}Response{{end}}">
            <div class="bar resp-bar"><span class="status"><span class="live"></span><span class="resp-label">{{if .RespStatus}}{{.RespStatus}}{{else}}Response{{end}}</span></span><span class="spacer"></span><button class="reset-btn" hidden onclick="resetResp(this.closest('.resp-panel'))">Reset</button><span class="t">Example</span></div>
            <pre class="j resp-body" data-example="{{.RespExample}}">{{.RespExample}}</pre>
          </div>
          {{else if .RespSchema}}
          <div class="code">
            <div class="bar resp-bar"><span class="status"><span class="live"></span>Response schema</span><span class="t">JSON</span></div>
            <pre class="j">{{.RespSchema}}</pre>
          </div>
          {{end}}
        </div>
      </div>
    </section>
    <hr class="sec-divider">
    {{end}}

    <section id="errors">
      <h2>Errors</h2>
      <p class="lead">Senda uses conventional HTTP status codes to indicate the success or failure of a request. Codes in the <code>2xx</code> range indicate success, <code>4xx</code> a problem with the request, and <code>5xx</code> an error on the server.</p>
      <table><thead><tr><th>Status</th><th>Meaning</th></tr></thead><tbody>
        <tr><td><code>200</code> OK</td><td>The request succeeded.</td></tr>
        <tr><td><code>201</code> Created</td><td>The resource was created.</td></tr>
        <tr><td><code>400</code> Bad Request</td><td>Malformed request or a missing required parameter.</td></tr>
        <tr><td><code>401</code> Unauthorized</td><td>No valid credentials were provided.</td></tr>
        <tr><td><code>404</code> Not Found</td><td>The requested resource does not exist.</td></tr>
        <tr><td><code>429</code> Too Many Requests</td><td>Rate limit exceeded; retry later.</td></tr>
        <tr><td><code>500</code> Server Error</td><td>Something went wrong on the server.</td></tr>
      </tbody></table>
    </section>

    {{range .Schemas}}
    <hr class="sec-divider">
    <section id="{{.ID}}">
      <h2>{{.Name}}</h2>
      <div class="inline-code"><pre>{{.Code}}</pre></div>
    </section>
    {{end}}
  </main>
</div>

<script>
const root=document.documentElement;
const MOON='<svg class="icn" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/></svg>';
const SUN='<svg class="icn" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M6.3 17.7l-1.4 1.4M19.1 4.9l-1.4 1.4"/></svg>';
const saved=localStorage.getItem('senda-theme');
if(saved)root.setAttribute('data-theme',saved);
function syncTheme(){const dark=root.getAttribute('data-theme')==='dark';
  document.getElementById('theme-label').textContent=dark?'Light':'Dark';
  document.getElementById('theme-icn').innerHTML=dark?SUN:MOON;}
function toggleTheme(){const next=root.getAttribute('data-theme')==='dark'?'light':'dark';
  root.setAttribute('data-theme',next);localStorage.setItem('senda-theme',next);syncTheme();}
syncTheme();

// "Try it" — live browser sends (always on)
function esc(s){return s.replace(/[&<>]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]));}
// hlJSON: highlight already-escaped JSON text (quotes stay literal; esc only touches & < >)
function hlJSON(s){return s.replace(/("(?:\\.|[^"\\])*")(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?/g,
  (m,str,colon,kw)=>{
    if(str!==undefined)return '<span class="j-'+(colon?'key':'str')+'">'+str+'</span>'+(colon?'<span class="j-punc">'+colon+'</span>':'');
    if(kw!==undefined)return '<span class="j-kw">'+m+'</span>';
    return '<span class="j-num">'+m+'</span>';});}
function sendReq(panel){
  const d=JSON.parse(panel.dataset.req);
  const url=(panel.querySelector('.try-url')||{}).value||d.url;
  const headers={};(d.headers||[]).forEach(([k,v])=>headers[k]=v);
  const opts={method:d.method,headers};
  if(d.body&&d.method!=='GET'&&d.method!=='HEAD')opts.body=d.body;
  // Prefer the sibling Response panel (live result replaces the baked example,
  // Reset restores it); fall back to an inline strip when there's no example.
  const rp=panel.closest('.split').querySelector('.resp-panel');
  const fallback=panel.querySelector('.live-resp');
  const render=(label,bodyHTML,isErr)=>{
    if(rp){
      rp.querySelector('.resp-label').textContent=label;
      rp.querySelector('.status').classList.toggle('err',!!isErr);
      rp.querySelector('.resp-body').innerHTML=bodyHTML;
      rp.querySelector('.reset-btn').hidden=false;
    }else{
      fallback.hidden=false;
      fallback.innerHTML='<div class="live-status'+(isErr?' err':'')+'">'+esc(label)+'</div>'+(bodyHTML?'<pre>'+bodyHTML+'</pre>':'');
    }
  };
  render('Sending…','',false);
  const t0=performance.now();
  fetch(url,opts).then(async r=>{
    const ms=Math.round(performance.now()-t0);let txt=await r.text();let isJSON=false;
    try{txt=JSON.stringify(JSON.parse(txt),null,2);isJSON=true;}catch(e){}
    render(r.status+' '+r.statusText+' · '+ms+'ms',isJSON?hlJSON(esc(txt)):esc(txt),!r.ok);
  }).catch(e=>render(String(e)+
    ' — request failed (likely CORS: the API must allow browser origins, or variables / path params need filling in above)','',true));
}
// Reset restores the baked example response after a live send.
function resetResp(rp){
  rp.querySelector('.resp-label').textContent=rp.dataset.label||'Response';
  rp.querySelector('.status').classList.remove('err');
  const pre=rp.querySelector('.resp-body');
  pre.innerHTML=hlJSON(esc(pre.dataset.example));
  rp.querySelector('.reset-btn').hidden=true;
}

// highlight static JSON blocks (already HTML-escaped by the template)
document.querySelectorAll('pre.j').forEach(p=>{p.innerHTML=hlJSON(p.innerHTML);});

// language tabs (per panel)
document.querySelectorAll('.code .tab').forEach(t=>t.addEventListener('click',()=>{
  const panel=t.closest('.code');
  panel.querySelectorAll('.tab').forEach(x=>x.classList.toggle('active',x===t));
  panel.querySelectorAll('pre[data-lang]').forEach(p=>p.hidden=p.dataset.lang!==t.dataset.lang);
}));

// search filter
const sInput=document.getElementById('nav-search');
sInput.addEventListener('input',()=>{
  const q=sInput.value.trim().toLowerCase();
  document.querySelectorAll('.navgroup').forEach(g=>{
    let any=false;
    g.querySelectorAll('.navlink').forEach(l=>{
      const hit=l.querySelector('.nav-label').textContent.toLowerCase().includes(q);
      l.style.display=hit?'':'none';if(hit)any=true;
    });
    g.style.display=any?'':'none';
  });
});
addEventListener('keydown',e=>{if((e.metaKey||e.ctrlKey)&&e.key.toLowerCase()==='k'){e.preventDefault();sInput.focus();}});

// scrollspy
const links=[...document.querySelectorAll('.navlink')];
const byId=id=>links.find(l=>l.getAttribute('href')==='#'+id);
document.querySelectorAll('main section').forEach(s=>new IntersectionObserver(es=>es.forEach(e=>{
  if(e.isIntersecting){links.forEach(l=>l.classList.remove('active'));const a=byId(e.target.id);if(a)a.classList.add('active');}
}),{rootMargin:'-12% 0px -80% 0px'}).observe(s));
</script>
{{define "param"}}<div class="param"><span class="pname">{{.Key}}</span>{{if .Type}}<span class="ptype">{{.Type}}</span>{{end}}{{if .Enabled}}<span class="preq">required</span>{{else}}<span class="popt">optional</span>{{end}}{{if .Desc}}<div class="pdesc">{{.Desc}}</div>{{end}}</div>{{end}}
</body>
</html>`
