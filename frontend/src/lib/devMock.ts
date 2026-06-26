// Dev-only fake Wails backend. The real `window.go` bridge is injected by the
// Wails native runtime and only exists inside the app's WebKitGTK window, so a
// plain browser (Firefox/Chrome) has no backend and the bindings throw
// "window.go is undefined". This installs a stand-in so the UI runs — and FPS
// can be compared — in any browser. Guarded by import.meta.env.DEV; never
// shipped in production builds.

// A heavy response so scrolling can be profiled across engines.
function bigComments(n = 1000): string {
  const arr = Array.from({ length: n }, (_, i) => ({
    postId: Math.floor(i / 5) + 1,
    id: i + 1,
    name: `comment author ${i}`,
    email: `user${i}@example.com`,
    body: `line ${i} — lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt`,
  }));
  return JSON.stringify(arr, null, 2);
}

const tree = {
  name: "demo",
  path: "/demo",
  isDir: true,
  children: [
    {
      name: "auth",
      path: "/demo/auth",
      isDir: true,
      children: [
        { name: "login", path: "/demo/auth/login.yaml", isDir: false },
        { name: "refresh", path: "/demo/auth/refresh.yaml", isDir: false },
      ],
    },
    { name: "comments", path: "/demo/comments.yaml", isDir: false },
    { name: "create-user", path: "/demo/create-user.yaml", isDir: false },
  ],
};

function reqFor(path: string) {
  const isComments = path.includes("comments");
  return {
    name: isComments ? "comments" : "create-user",
    method: isComments ? "GET" : "POST",
    url: isComments
      ? "https://jsonplaceholder.typicode.com/comments"
      : "{{baseUrl}}/users",
    params: [],
    headers: [{ key: "Accept", value: "application/json", enabled: true }],
    body: isComments
      ? { type: "none" }
      : { type: "json", raw: '{\n  "name": "Ada Lovelace"\n}' },
    auth: isComments
      ? { type: "inherit" }
      : { type: "bearer", token: "{{token}}" },
    docs: isComments
      ? ""
      : "# Create user\n\nCreates a new user account.\n\n- **Auth**: Bearer token\n",
  };
}

export function installDevMock() {
  const w = window as any;
  if (w.go) return; // real runtime present
  const body = bigComments();
  const size = new TextEncoder().encode(body).length;
  w.go = {
    main: {
      App: {
        Ping: async () => "senda-dev-mock",
        // Mirrors internal/docgen RenderFragment enough to exercise the Docs
        // preview iframe (headings, bold, paragraphs).
        RenderMarkdown: async (md: string) =>
          String(md)
            .split("\n")
            .map((l) =>
              l.startsWith("# ")
                ? `<h1>${l.slice(2)}</h1>`
                : l === ""
                  ? "<br>"
                  : `<p>${l.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")}</p>`,
            )
            .join("\n"),
        OpenCollection: async () => ({
          name: "demo-api",
          path: "/demo",
          vars: [{ key: "baseUrl", value: "https://api.demo.test", enabled: true }],
          tree,
        }),
        ListEnvironments: async () => [
          { name: "dev", vars: [{ key: "baseUrl", value: "https://dev.api", enabled: true }] },
          { name: "prod", vars: [{ key: "baseUrl", value: "https://api.demo.test", enabled: true }] },
        ],
        ListFlows: async () => [
          { name: "Fetch post author", path: "/demo/.senda/flows/fetch-post-author.flow.yaml" },
          { name: "Public data snapshot", path: "/demo/.senda/flows/public-data-snapshot.flow.yaml" },
          { name: "Fetch posts (loop)", path: "/demo/.senda/flows/fetch-posts-loop.flow.yaml" },
        ],
        ReadFlow: async () => ({
          name: "Fetch post author",
          path: "/demo/.senda/flows/fetch-post-author.flow.yaml",
          start: "getPost",
          nodes: {
            getPost: { type: "request", request: "Chaining/get-post.yaml", next: "check" },
            check: { type: "branch", cond: { left: "{{res.get-post.status}}", op: "eq", right: "200" }, onTrue: "setUid", onFalse: "" },
            setUid: { type: "setvar", var: "uid", from: "{{res.get-post.json.userId}}", next: "getUser" },
            getUser: { type: "request", request: "Chaining/get-user.yaml" },
          },
        }),
        RunFlow: async () => [
          { nodeId: "getPost", type: "request", result: { name: "Get post", path: "Chaining/get-post.yaml", method: "GET", url: "https://jsonplaceholder.typicode.com/posts/1", status: 200, durationMs: 31, sizeBytes: 292, ok: true, assertPass: 1, assertFail: 0, response: { status: 200, statusText: "OK", durationMs: 31, sizeBytes: 292, headers: { "Content-Type": ["application/json"] }, body: '{\n  "userId": 1,\n  "id": 1,\n  "title": "sunt aut facere"\n}', truncated: false } } },
          { nodeId: "check", type: "branch", branch: "true" },
          { nodeId: "setUid", type: "setvar" },
          { nodeId: "getUser", type: "request", result: { name: "Get post author", path: "Chaining/get-user.yaml", method: "GET", url: "https://jsonplaceholder.typicode.com/users/1", status: 200, durationMs: 18, sizeBytes: 509, ok: true, assertPass: 2, assertFail: 0, response: { status: 200, statusText: "OK", durationMs: 18, sizeBytes: 509, headers: { "Content-Type": ["application/json"] }, body: '{\n  "id": 1,\n  "name": "Leanne Graham",\n  "email": "Sincere@april.biz"\n}', truncated: false } } },
        ],
        ReadRequest: async (path: string) => reqFor(path),
        ReadFolderMeta: async (path: string) => ({ name: String(path).split("/").pop() ?? "", path, color: "", tags: [], description: "", vars: [], auth: { type: "inherit" } }),
        ResolveScope: async () => [],
        SaveRequest: async () => {},
        DeleteRequest: async () => {},
        DeleteNode: async () => {},
        CreateFolder: async () => {},
        SaveCollection: async () => {},
        SaveEnvironment: async () => {},
        PickFile: async () => "/picked/client-cert.pem",
        SendRequest: async (req: any) => {
          const big = String(req?.url ?? "").includes("comments");
          const payload = big ? body : '{\n  "id": "usr_8f2a",\n  "created": true\n}';
          return {
            status: big ? 200 : 201,
            statusText: big ? "OK" : "Created",
            durationMs: 29,
            sizeBytes: big ? size : payload.length,
            headers: { "Content-Type": ["application/json"], Server: ["senda-dev-mock"] },
            body: payload,
            truncated: false,
          };
        },
        GitStatus: async () => ({
          repo: true,
          branch: "main",
          files: [
            { path: "users/create-user.yaml", display: "create-user", status: "modified", other: false },
            { path: "users/list-users.yaml", display: "list-users", status: "untracked", other: false },
            { path: ".gitignore", display: ".gitignore", status: "untracked", other: true },
          ],
        }),
        GitDiff: async (_collPath: string, path: string) =>
          String(path).endsWith(".gitignore")
            ? { display: ".gitignore", fields: [], raw: "+node_modules/\n+dist/\n" }
            : {
                display: "create-user",
                fields: [
                  { label: "Method", old: "GET", new: "POST", kind: "changed" },
                  { label: "URL", old: "https://api.demo.test/users", new: "https://api.demo.test/v2/users", kind: "changed" },
                  { label: "Headers", old: "", new: "- key: Content-Type\n  value: application/json", kind: "added" },
                ],
                raw: "",
              },
      },
    },
  };
  w.runtime = w.runtime ?? {};
  // Seed a collection so the tree loads on first paint (mirrors a returning user).
  try {
    localStorage.setItem("senda.lastCollection", "/demo");
  } catch {
    /* ignore */
  }
  // eslint-disable-next-line no-console
  console.info("[senda] dev mock backend installed (no Wails runtime detected)");
}
