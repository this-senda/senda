// Test stand-in for the generated Wails bindings module
// `bindings/senda/internal/app/app` (aliased in vite.config.ts, mode "test")
// (see models.ts for why stubs exist). Each export delegates to the dev mock
// backend (`window.go.main.App`, installed by lib/devMock.ts) when present,
// so `vite --mode test` serves a fully clickable UI in a plain browser; in
// unit tests without the mock every call resolves to undefined — tests should
// mock at the lib/api.ts boundary instead of relying on these.

function call(name: string, ...args: unknown[]): Promise<any> {
  const fn = (window as any).go?.main?.App?.[name];
  return typeof fn === "function" ? fn(...args) : Promise.resolve(undefined);
}

const method =
  (name: string) =>
  (...args: unknown[]) =>
    call(name, ...args);

export const Ping = method("Ping");
export const PickDirectory = method("PickDirectory");
export const PickZipCollection = method("PickZipCollection");
export const PickFile = method("PickFile");
export const PickImportFile = method("PickImportFile");
export const SendRequest = method("SendRequest");
export const OpenCollection = method("OpenCollection");
export const GitGuardStatus = method("GitGuardStatus");
export const GitGuardIgnore = method("GitGuardIgnore");
export const SaveCollection = method("SaveCollection");
export const ReadFolderMeta = method("ReadFolderMeta");
export const ResolveScope = method("ResolveScope");
export const FakerTokens = method("FakerTokens");
export const ReadRequest = method("ReadRequest");
export const SaveRequest = method("SaveRequest");
export const DeleteRequest = method("DeleteRequest");
export const DeleteNode = method("DeleteNode");
export const CreateFolder = method("CreateFolder");
export const ListEnvironments = method("ListEnvironments");
export const SaveEnvironment = method("SaveEnvironment");
export const ExportFile = method("ExportFile");
export const ImportCurl = method("ImportCurl");
export const ImportCollection = method("ImportCollection");
export const GenerateMocksFromOpenAPI = method("GenerateMocksFromOpenAPI");
export const GenerateMocksFromHAR = method("GenerateMocksFromHAR");
export const RequestToHAR = method("RequestToHAR");
export const GenerateCode = method("GenerateCode");
export const RenderMarkdown = method("RenderMarkdown");
export const CodegenTargets = method("CodegenTargets");
export const RunFolder = method("RunFolder");
export const ListFlows = method("ListFlows");
export const ReadFlow = method("ReadFlow");
export const ReadFlowRaw = method("ReadFlowRaw");
export const SaveFlowRaw = method("SaveFlowRaw");
export const DeleteFlow = method("DeleteFlow");
export const CreateFlow = method("CreateFlow");
export const ValidateFlow = method("ValidateFlow");
export const RunFlow = method("RunFlow");
export const RunLoad = method("RunLoad");
export const RunSecurityScan = method("RunSecurityScan");
export const SecurityScanPlan = method("SecurityScanPlan");
export const SyncSecurityTemplates = method("SyncSecurityTemplates");
export const SecurityTemplatesState = method("SecurityTemplatesState");
export const GitStatus = method("GitStatus");
export const GitDiff = method("GitDiff");
export const RenameNode = method("RenameNode");
export const MoveNode = method("MoveNode");
export const ListHistory = method("ListHistory");
export const ClearHistory = method("ClearHistory");
export const CollectionActivity = method("CollectionActivity");
export const ListRuntimeVars = method("ListRuntimeVars");
export const ClearRuntimeVars = method("ClearRuntimeVars");
export const ListCookies = method("ListCookies");
export const ClearCookies = method("ClearCookies");
export const ConnectWebSocket = method("ConnectWebSocket");
export const OpenWebSocket = method("OpenWebSocket");
export const SendWebSocketMessage = method("SendWebSocketMessage");
export const CloseWebSocket = method("CloseWebSocket");
export const ConnectSSE = method("ConnectSSE");
export const GenerateAssertions = method("GenerateAssertions");
export const AIConfigured = method("AIConfigured");
export const StartMockServer = method("StartMockServer");
export const StopMockServer = method("StopMockServer");
export const MockServerRoutes = method("MockServerRoutes");
export const MockServerLog = method("MockServerLog");
export const MockServerInfo = method("MockServerInfo");
export const MockPresets = method("MockPresets");
export const ScaffoldMockPreset = method("ScaffoldMockPreset");
export const PreviewMockRoutes = method("PreviewMockRoutes");
export const SetMockScenario = method("SetMockScenario");
export const SetMockRouteResponse = method("SetMockRouteResponse");
export const ResetMockState = method("ResetMockState");
export const SaveResponseAsMock = method("SaveResponseAsMock");
