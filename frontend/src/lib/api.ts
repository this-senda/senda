// Thin typed wrapper over the generated Wails bindings. All backend access
// goes through here so components stay free of binding paths and tests can
// mock a single module.
import * as App from "../../bindings/senda/internal/app/app";
import * as model from "../../bindings/senda/internal/model/models";
import * as mockserverModel from "../../bindings/senda/internal/mockserver/models";
import type { ScopeVar } from "../../bindings/senda/internal/app/models";
import type { SyncState } from "../../bindings/senda/internal/security/models";
import type { Activity } from "../../bindings/senda/internal/store/models";
import type { Status as GitStatus, Diff as GitDiff, ChangedFile, FieldDiff } from "../../bindings/senda/internal/scm/models";

export type { ScopeVar };

export type { SyncState };
export type { Activity };
export type { GitStatus, GitDiff, ChangedFile, FieldDiff };

export type Request = model.Request;
export const BodyType = model.BodyType;
export type Response = model.Response;
export type Collection = model.Collection;
export type Environment = model.Environment;
export type TreeNode = model.TreeNode;
export type KV = model.KV;
export type Auth = model.Auth;
export type TLSConfig = model.TLSConfig;
export type Assert = model.Assert;
export type AssertResult = model.AssertResult;
export type RunResult = model.RunResult;
export type HistoryEntry = model.HistoryEntry;
export type LoadOptions = model.LoadOptions;
export type LoadSummary = model.LoadSummary;
export type SecurityOptions = model.SecurityOptions;
export type SecuritySummary = model.SecuritySummary;
export type ScanPlan = model.ScanPlan;
export type WSSession = model.WSSession;
export type WSMessage = model.WSMessage;
export type SSESession = model.SSESession;
export type SSEEvent = model.SSEEvent;
export type MockLogEntry = mockserverModel.LogEntry;
export type MockRouteInfo = mockserverModel.RouteInfo;
export type MockInfo = mockserverModel.Info;
// SecurityCheck is emitted as a Wails event (not a binding return type) so
// it is not in the generated models. Mirror the Go struct here.
export interface SecurityCheck {
  templateId: string;
  name: string;
  severity: string;
  target: string;
  matched: boolean;
  matchedAt?: string;
  error?: string;
  description?: string;
  tags?: string[];
  reference?: string[];
  remediation?: string;
  owasp?: string;
  cwe?: string[];
}
// LoadTick is emitted as a Wails event (not a binding return type) so it is
// not in the generated models. Mirror the Go struct here.
export interface LoadTick {
  elapsed: number;
  total: number;
  errors: number;
  rps: number;
  p50: number;
  p95: number;
  p99: number;
  statusDist: Record<number, number>;
}

export const api = {
  ping: () => App.Ping(),
  buildInfo: () => App.BuildInfo(),
  checkUpdate: () => App.CheckUpdate(),
  pickDirectory: (title: string) => App.PickDirectory(title),
  pickZipCollection: (title: string) => App.PickZipCollection(title),
  pickFile: (title: string) => App.PickFile(title),
  send: (req: Request, collPath: string, reqPath: string, envName: string) =>
    App.SendRequest(req, collPath, reqPath, envName),
  openCollection: (path: string) => App.OpenCollection(path),
  gitGuardStatus: (path: string) => App.GitGuardStatus(path),
  gitGuardIgnore: (path: string) => App.GitGuardIgnore(path),
  saveCollection: (coll: Collection) => App.SaveCollection(coll),
  readFolderMeta: (path: string) => App.ReadFolderMeta(path),
  resolveScope: (collPath: string, reqPath: string, envName: string) =>
    App.ResolveScope(collPath, reqPath, envName),
  fakerTokens: () => App.FakerTokens(),
  readRequest: (path: string) => App.ReadRequest(path),
  saveRequest: (path: string, req: Request) => App.SaveRequest(path, req),
  deleteRequest: (path: string) => App.DeleteRequest(path),
  deleteNode: (path: string) => App.DeleteNode(path),
  createFolder: (path: string) => App.CreateFolder(path),
  listEnvironments: (collPath: string) => App.ListEnvironments(collPath),
  saveEnvironment: (collPath: string, env: Environment) =>
    App.SaveEnvironment(collPath, env),

  // export
  exportFile: (filename: string, content: string) => App.ExportFile(filename, content),
  exportDocsHtml: (collPath: string, subPath = "") => App.ExportDocsHTML(collPath, subPath),

  // import / codegen
  importCurl: (cmd: string) => App.ImportCurl(cmd),
  importCollection: (collPath: string, format: string, data: string, destSubdir: string) =>
    App.ImportCollection(collPath, format, data, destSubdir),
  generateMocksFromOpenAPI: (collPath: string, data: string) =>
    App.GenerateMocksFromOpenAPI(collPath, data),
  generateCode: (req: Request, target: string) => App.GenerateCode(req, target),
  renderMarkdown: (md: string) => App.RenderMarkdown(md),
  codegenTargets: () => App.CodegenTargets(),

  // runner
  runFolder: (folderPath: string, collPath: string, envName: string) =>
    App.RunFolder(folderPath, collPath, envName),
  runLoad: (folderPath: string, collPath: string, envName: string, opts: LoadOptions) =>
    App.RunLoad(folderPath, collPath, envName, opts),
  runSecurityScan: (folderPath: string, collPath: string, envName: string, opts: SecurityOptions) =>
    App.RunSecurityScan(folderPath, collPath, envName, opts),
  securityScanPlan: (folderPath: string, collPath: string, envName: string, opts: SecurityOptions) =>
    App.SecurityScanPlan(folderPath, collPath, envName, opts),
  syncSecurityTemplates: (collPath: string, url: string, ref: string) =>
    App.SyncSecurityTemplates(collPath, url, ref),
  securityTemplatesState: (collPath: string) => App.SecurityTemplatesState(collPath),

  // source control: read-only git comparison (working tree vs HEAD)
  gitStatus: (collPath: string) => App.GitStatus(collPath),
  gitDiff: (collPath: string, path: string) => App.GitDiff(collPath, path),

  // tree ops
  renameNode: (path: string, newName: string) => App.RenameNode(path, newName),
  moveNode: (srcPath: string, destDir: string) => App.MoveNode(srcPath, destDir),

  // history
  listHistory: (collPath: string, limit: number) => App.ListHistory(collPath, limit),
  clearHistory: (collPath: string) => App.ClearHistory(collPath),

  // sidebar recency pills: last-run activity per request path
  collectionActivity: (collPath: string) => App.CollectionActivity(collPath),

  // runtime vars (set by scripts)
  listRuntimeVars: () => App.ListRuntimeVars(),
  clearRuntimeVars: () => App.ClearRuntimeVars(),

  // cookies
  listCookies: (url: string) => App.ListCookies(url),
  clearCookies: () => App.ClearCookies(),

  // WebSocket — interactive: open persistent conn, send/close by id.
  // Received messages + close arrive via the "ws:event" Wails event.
  openWebSocket: (req: Request, collPath: string, envName: string) =>
    App.OpenWebSocket(req, collPath, envName),
  sendWebSocketMessage: (id: string, message: string) => App.SendWebSocketMessage(id, message),
  closeWebSocket: (id: string) => App.CloseWebSocket(id),

  // SSE
  connectSSE: (req: Request, collPath: string, envName: string) =>
    App.ConnectSSE(req, collPath, envName),

  // AI assertion generation
  generateAssertions: (resp: Response) => App.GenerateAssertions(resp),
  aiConfigured: () => App.AIConfigured(),

  // Mock server
  startMockServer: (collPath: string, addr: string) => App.StartMockServer(collPath, addr),
  stopMockServer: () => App.StopMockServer(),
  mockServerRoutes: () => App.MockServerRoutes(),
  mockServerLog: () => App.MockServerLog(),
  mockServerInfo: () => App.MockServerInfo(),
  mockPresets: () => App.MockPresets(),
  scaffoldMockPreset: (collPath: string, preset: string) =>
    App.ScaffoldMockPreset(collPath, preset),
  previewMockRoutes: (collPath: string) => App.PreviewMockRoutes(collPath),
  setMockScenario: (name: string) => App.SetMockScenario(name),
  setMockRouteResponse: (method: string, path: string, status: number) =>
    App.SetMockRouteResponse(method, path, status),
  resetMockState: () => App.ResetMockState(),
  saveResponseAsMock: (
    collPath: string,
    name: string,
    method: string,
    path: string,
    status: number,
    headers: Record<string, string>,
    body: string,
  ) => App.SaveResponseAsMock(collPath, name, method, path, status, headers, body),
};
