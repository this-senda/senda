// Self-contained fake Wails backend for visual capture. Injected via
// page.addInitScript before the app boots, so `window.go` exists and the app's
// own dev mock (lib/devMock.ts) sees it and steps aside. This mirrors the shape
// of the real bindings (bindings/senda/app) but returns rich, deterministic
// sample data so every panel — including newer ones like the mock server — has
// something to show. Keep field names in sync with src/test-stubs/models.ts and
// src/lib/api.ts.
//
// Exported as a string so it can be handed to Playwright's addInitScript, which
// serializes the function and runs it in the page before any app script.
export function installCaptureMock() {
  if (window.go) return; // real runtime present

  const tree = {
    name: "demo-api",
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
      {
        name: "users",
        path: "/demo/users",
        isDir: true,
        children: [
          { name: "list-users", path: "/demo/users/list-users.yaml", isDir: false },
          { name: "create-user", path: "/demo/users/create-user.yaml", isDir: false },
          { name: "get-user", path: "/demo/users/get-user.yaml", isDir: false },
        ],
      },
      { name: "comments", path: "/demo/comments.yaml", isDir: false },
      { name: "health", path: "/demo/health.yaml", isDir: false },
    ],
  };

  // Fully populated request so the Headers/Body/Tests/Script/Auth/Docs tabs all
  // render real content with badge counts.
  const createUser = {
    name: "create-user",
    method: "POST",
    url: "{{baseUrl}}/users",
    params: [
      { key: "verbose", value: "true", enabled: true },
      { key: "trace", value: "{{traceId}}", enabled: false },
    ],
    headers: [
      { key: "Content-Type", value: "application/json", enabled: true },
      { key: "Authorization", value: "Bearer {{token}}", enabled: true },
      { key: "X-Request-Id", value: "{{$uuid}}", enabled: true },
    ],
    body: {
      type: "json",
      raw: '{\n  "name": "Ada Lovelace",\n  "email": "ada@example.com",\n  "role": "admin"\n}',
    },
    auth: { type: "bearer", token: "{{token}}" },
    asserts: [
      { target: "status", op: "eq", value: "201", enabled: true },
      { target: "json.id", op: "exists", value: "", enabled: true },
      { target: "header.Content-Type", op: "contains", value: "application/json", enabled: true },
      { target: "duration", op: "lt", value: "500", enabled: true },
    ],
    preScript:
      "// Runs before the request is sent\n" +
      'senda.setVar("traceId", crypto.randomUUID());\n' +
      'senda.setVar("token", senda.getVar("token") || "demo-token");\n',
    postScript:
      "// Runs after the response arrives\n" +
      'pm.test("status is 201", () => pm.expect(res.status).to.equal(201));\n' +
      'pm.test("returns an id", () => pm.expect(res.json().id).to.be.a("string"));\n' +
      'senda.setVar("userId", res.json().id);\n',
    docs:
      "# Create user\n\n" +
      "Creates a new user account.\n\n" +
      "- **Auth**: Bearer token (`{{token}}`)\n" +
      "- **Returns**: `201 Created` with the new user record\n",
  };

  const listUsers = {
    name: "list-users",
    method: "GET",
    url: "{{baseUrl}}/users",
    params: [{ key: "limit", value: "20", enabled: true }],
    headers: [{ key: "Accept", value: "application/json", enabled: true }],
    body: { type: "none" },
    auth: { type: "inherit" },
    asserts: [{ target: "status", op: "eq", value: "200", enabled: true }],
    preScript: "",
    postScript: "",
    docs: "",
  };

  const reqFor = (path) => {
    if (String(path).includes("list-users")) return listUsers;
    return createUser;
  };

  const createdBody =
    '{\n  "id": "usr_8f2a3c",\n  "name": "Ada Lovelace",\n' +
    '  "email": "ada@example.com",\n  "role": "admin",\n  "created": true\n}';

  // Mock-server sample state (the newest feature). RouteInfo / Info / LogEntry
  // shapes mirror internal/mockserver/models.
  const mockRoutes = [
    { method: "GET", path: "/users", kind: "resource", status: 200 },
    { method: "POST", path: "/users", kind: "resource", status: 201 },
    { method: "GET", path: "/users/:id", kind: "resource", status: 200 },
    { method: "DELETE", path: "/users/:id", kind: "resource", status: 204 },
    {
      method: "POST",
      path: "/oauth/token",
      kind: "rule",
      status: 200,
      active: 200,
      variants: [
        { status: 200, desc: "valid token" },
        { status: 401, desc: "invalid client" },
      ],
    },
    { method: "GET", path: "/.well-known/openid-configuration", kind: "rule", status: 200 },
  ];
  const mockLog = [
    { method: "POST", path: "/oauth/token", status: 200, source: "rule", at: "12:04:01" },
    { method: "GET", path: "/users", status: 200, source: "resource", at: "12:04:02" },
    { method: "POST", path: "/users", status: 201, source: "resource", at: "12:04:03" },
    { method: "GET", path: "/users/usr_8f2a3c", status: 200, source: "resource", at: "12:04:04" },
    { method: "DELETE", path: "/users/usr_8f2a3c", status: 204, source: "resource", at: "12:04:05" },
  ];
  const mockInfo = {
    addr: "127.0.0.1:8787",
    cors: true,
    proxy: "https://api.demo.test",
    scenario: "",
    scenarios: ["happy-path", "rate-limited", "server-error"],
  };

  const App = {
    Ping: async () => "senda-capture-mock",
    PickDirectory: async () => "/demo",
    PickZipCollection: async () => "/demo",
    PickFile: async () => "",
    OpenCollection: async () => ({
      name: "demo-api",
      path: "/demo",
      vars: [
        { key: "baseUrl", value: "https://api.demo.test", enabled: true },
        { key: "token", value: "demo-token", enabled: true },
      ],
      tree,
    }),
    SaveCollection: async () => {},
    EncryptionStatus: async () => ({ enabled: false, keyAvailable: false, source: "" }),
    EnableEncryption: async () => {},
    DisableEncryption: async () => {},
    ExportKey: async () => "",
    ImportKey: async () => {},
    ReadFolderMeta: async (path) => ({
      name: String(path).split("/").pop() ?? "",
      path,
      color: "",
      tags: [],
      description: "",
      vars: [],
      auth: { type: "inherit" },
    }),
    ListEnvironments: async () => [
      { name: "dev", vars: [{ key: "baseUrl", value: "https://dev.api.demo.test", enabled: true }] },
      { name: "staging", vars: [{ key: "baseUrl", value: "https://staging.api.demo.test", enabled: true }] },
      { name: "prod", vars: [{ key: "baseUrl", value: "https://api.demo.test", enabled: true }] },
    ],
    SaveEnvironment: async () => {},
    ResolveScope: async () => [
      { key: "baseUrl", value: "https://dev.api.demo.test", source: "environment", enabled: true },
      { key: "token", value: "demo-token", source: "collection", enabled: true },
      { key: "traceId", value: "a1b2c3", source: "runtime", enabled: true },
    ],
    ReadRequest: async (path) => reqFor(path),
    SaveRequest: async () => {},
    DeleteRequest: async () => {},
    DeleteNode: async () => {},
    CreateFolder: async () => {},
    RenameNode: async () => {},
    MoveNode: async () => {},
    ExportFile: async () => {},
    SendRequest: async (req) => {
      const isList = String(req?.url ?? "").includes("/users") && (req?.method ?? "") === "GET";
      const body = isList
        ? '[\n  { "id": "usr_8f2a3c", "name": "Ada Lovelace" },\n  { "id": "usr_1d4e5f", "name": "Alan Turing" }\n]'
        : createdBody;
      return {
        status: isList ? 200 : 201,
        statusText: isList ? "OK" : "Created",
        durationMs: 142,
        sizeBytes: new TextEncoder().encode(body).length,
        headers: {
          "Content-Type": ["application/json; charset=utf-8"],
          Server: ["senda-demo"],
          "X-Request-Id": ["req_7f3a"],
        },
        body,
        truncated: false,
        asserts: [
          { target: "status", op: "eq", value: "201", pass: true, actual: "201" },
          { target: "json.id", op: "exists", value: "", pass: true, actual: "usr_8f2a3c" },
          { target: "header.Content-Type", op: "contains", value: "application/json", pass: true, actual: "application/json; charset=utf-8" },
          { target: "duration", op: "lt", value: "500", pass: true, actual: "142" },
        ],
        tests: [
          { name: "status is 201", pass: true },
          { name: "returns an id", pass: true },
        ],
      };
    },

    // codegen
    CodegenTargets: async () => ["curl", "fetch", "httpie", "python", "go"],
    GenerateCode: async (_req, target) => {
      const samples = {
        curl:
          "curl -X POST 'https://api.demo.test/users' \\\n" +
          "  -H 'Content-Type: application/json' \\\n" +
          "  -H 'Authorization: Bearer demo-token' \\\n" +
          '  -d \'{"name":"Ada Lovelace","email":"ada@example.com"}\'',
        fetch:
          "await fetch('https://api.demo.test/users', {\n" +
          "  method: 'POST',\n" +
          "  headers: {\n" +
          "    'Content-Type': 'application/json',\n" +
          "    'Authorization': 'Bearer demo-token',\n" +
          "  },\n" +
          "  body: JSON.stringify({ name: 'Ada Lovelace', email: 'ada@example.com' }),\n" +
          "});",
        httpie:
          "http POST api.demo.test/users \\\n" +
          "  Authorization:'Bearer demo-token' \\\n" +
          "  name='Ada Lovelace' email='ada@example.com'",
        python:
          "import requests\n\n" +
          "requests.post(\n" +
          "    'https://api.demo.test/users',\n" +
          "    headers={'Authorization': 'Bearer demo-token'},\n" +
          "    json={'name': 'Ada Lovelace', 'email': 'ada@example.com'},\n" +
          ")",
        go:
          'req, _ := http.NewRequest("POST", "https://api.demo.test/users", body)\n' +
          'req.Header.Set("Authorization", "Bearer demo-token")\n' +
          "resp, _ := http.DefaultClient.Do(req)",
      };
      return samples[target] ?? samples.curl;
    },

    // import
    ImportCurl: async () => createUser,
    ImportCollection: async () => ({ imported: 4 }),
    GenerateMocksFromOpenAPI: async () => ({ written: 6 }),

    // flows
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
    RunFlow: async () => [],

    // runner / load / security
    RunFolder: async () => [],
    RunLoad: async () => ({}),
    RunSecurityScan: async () => ({}),
    SecurityScanPlan: async () => ({ checks: [] }),
    SyncSecurityTemplates: async () => ({}),
    SecurityTemplatesState: async () => ({ present: false }),

    // source control (read-only git comparison)
    GitStatus: async () => ({
      repo: true,
      branch: "main",
      files: [
        { path: "users/create-user.yaml", display: "create-user", status: "modified", other: false },
        { path: "users/list-users.yaml", display: "list-users", status: "untracked", other: false },
        { path: ".gitignore", display: ".gitignore", status: "untracked", other: true },
      ],
    }),
    GitDiff: async (_collPath, path) =>
      path.endsWith(".gitignore")
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

    // history
    ListHistory: async () => [
      { method: "POST", url: "https://api.demo.test/users", status: 201, at: new Date(Date.now() - 60_000).toISOString() },
      { method: "GET", url: "https://api.demo.test/users", status: 200, at: new Date(Date.now() - 180_000).toISOString() },
      { method: "POST", url: "https://api.demo.test/auth/login", status: 200, at: new Date(Date.now() - 600_000).toISOString() },
      { method: "GET", url: "https://api.demo.test/users/usr_9000", status: 404, at: new Date(Date.now() - 900_000).toISOString() },
      { method: "DELETE", url: "https://api.demo.test/users/usr_1d4e5f", status: 204, at: new Date(Date.now() - 1_800_000).toISOString() },
    ],
    ClearHistory: async () => {},
    CollectionActivity: async () => ({}),

    // runtime vars / cookies
    ListRuntimeVars: async () => ({ token: "demo-token", traceId: "a1b2c3", userId: "usr_8f2a3c" }),
    ClearRuntimeVars: async () => {},
    ListCookies: async () => [],
    ClearCookies: async () => {},

    // websocket / sse
    ConnectWebSocket: async () => ({ id: "ws_1" }),
    ConnectSSE: async () => ({ id: "sse_1" }),

    // AI
    GenerateAssertions: async () => [],
    AIConfigured: async () => false,

    // faker tokens ({{$category.name}} body autocomplete). Real backend pulls
    // gofakeit's full catalog; this slice is enough to populate the dropdown.
    FakerTokens: async () => [
      { category: "person", name: "firstname", example: "Ada" },
      { category: "person", name: "lastname", example: "Lovelace" },
      { category: "person", name: "name", example: "Ada Lovelace" },
      { category: "person", name: "email", example: "ada@example.com" },
      { category: "person", name: "phone", example: "555-0142" },
      { category: "internet", name: "username", example: "ada_l" },
      { category: "internet", name: "url", example: "https://example.com" },
      { category: "internet", name: "ipv4", example: "192.168.0.42" },
      { category: "address", name: "city", example: "London" },
      { category: "address", name: "country", example: "United Kingdom" },
      { category: "company", name: "name", example: "Analytical Engines Ltd" },
      { category: "misc", name: "uuid", example: "f47ac10b-58cc-4372-a567-0e02b2c3d479" },
    ],

    // mock server (newest feature)
    StartMockServer: async () => mockInfo.addr,
    StopMockServer: async () => {},
    MockServerRoutes: async () => mockRoutes,
    MockServerLog: async () => mockLog,
    MockServerInfo: async () => mockInfo,
    MockPresets: async () => ["oauth", "crud", "rest-api"],
    ScaffoldMockPreset: async () => ["mocks/oauth/token.yaml", "mocks/oauth/userinfo.yaml"],
    PreviewMockRoutes: async () => mockRoutes,
    SetMockScenario: async (name) => { mockInfo.scenario = name; },
    SetMockRouteResponse: async () => {},
    ResetMockState: async () => {},
    SaveResponseAsMock: async () => ({}),
  };

  window.go = { main: { App } };
  window.runtime = window.runtime ?? {};
}
