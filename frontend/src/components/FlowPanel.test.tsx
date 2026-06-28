import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@solidjs/testing-library";

// Mock the Go binding boundary and the Wails event runtime (no real backend).
vi.mock("../lib/api", () => ({
  api: {
    listFlows: vi.fn(() =>
      Promise.resolve([{ name: "signup", path: "/c/.senda/flows/signup.flow.yaml" }]),
    ),
    readFlow: vi.fn(() =>
      Promise.resolve({
        name: "signup",
        path: "/c/.senda/flows/signup.flow.yaml",
        start: "login",
        nodes: {
          login: { type: "request", request: "auth/login.yaml", next: "check" },
          check: { type: "branch", cond: { left: "{{res.login.status}}", op: "eq", right: "200" }, onTrue: "", onFalse: "" },
        },
      }),
    ),
    runFlow: vi.fn(() =>
      Promise.resolve([
        {
          nodeId: "login",
          type: "request",
          result: { name: "login", path: "login.yaml", method: "GET", url: "https://x/login", status: 200, ok: true, assertPass: 0, assertFail: 0, durationMs: 1, sizeBytes: 0, response: { status: 200, body: '{"token":"sekret"}', headers: {}, durationMs: 1, sizeBytes: 0, truncated: false } },
        },
        { nodeId: "check", type: "branch", branch: "true" },
      ]),
    ),
    readFlowRaw: vi.fn(() => Promise.resolve("name: signup\nstart: login\nnodes: {}\n")),
    saveFlowRaw: vi.fn(() => Promise.resolve()),
    deleteFlow: vi.fn(() => Promise.resolve()),
    createFlow: vi.fn(() => Promise.resolve("/c/.senda/flows/newflow.flow.yaml")),
    validateFlow: vi.fn(() => Promise.resolve(["node \"login\": next edge targets missing node \"x\""])),
  },
}));
vi.mock("@wailsio/runtime", () => ({ Events: { On: () => () => {} } }));
// Mock the dialog helpers (no <Dialog/> mounted here) and CM6 host (jsdom can't
// measure it) so the editor flows are driveable.
vi.mock("../lib/dialog", () => ({
  confirmDialog: vi.fn(() => Promise.resolve(true)),
  promptDialog: vi.fn(() => Promise.resolve("newflow")),
}));
vi.mock("./CodeEditor", () => ({
  default: (p: any) => (
    <textarea class="code-editor" value={p.value} onInput={(e: any) => p.onChange?.(e.currentTarget.value)} />
  ),
}));

import { api } from "../lib/api";
import FlowPanel from "./FlowPanel";
import { setCollection } from "../lib/store";

beforeEach(() => {
  vi.clearAllMocks();
  setCollection({ name: "c", path: "/c" } as any);
});

describe("FlowPanel", () => {
  it("lists flows from the backend", async () => {
    render(() => <FlowPanel onClose={() => {}} />);
    await waitFor(() => expect(screen.getByText("signup")).toBeInTheDocument());
    expect(api.listFlows).toHaveBeenCalledWith("/c");
  });

  it("shows a flow's node graph before running", async () => {
    render(() => <FlowPanel onClose={() => {}} />);
    fireEvent.click(await screen.findByText("signup"));
    // graph renders the start badge + the branch summary, no run yet.
    await waitFor(() => expect(screen.getByText("start")).toBeInTheDocument());
    expect(screen.getByText(/eq 200/)).toBeInTheDocument();
    expect(api.readFlow).toHaveBeenCalledWith("/c/.senda/flows/signup.flow.yaml");
    expect(api.runFlow).not.toHaveBeenCalled();
  });

  it("runs the selected flow and renders its steps", async () => {
    render(() => <FlowPanel onClose={() => {}} />);
    fireEvent.click(await screen.findByText("signup"));
    fireEvent.click(await screen.findByText("Run flow"));
    await waitFor(() => expect(screen.getByText("200")).toBeInTheDocument());
    expect(api.runFlow).toHaveBeenCalledWith("/c/.senda/flows/signup.flow.yaml", "/c", expect.anything());
  });

  it("opens a request step's response on click", async () => {
    render(() => <FlowPanel onClose={() => {}} />);
    fireEvent.click(await screen.findByText("signup"));
    fireEvent.click(await screen.findByText("Run flow"));
    const url = await screen.findByText("https://x/login");
    expect(screen.queryByText(/sekret/)).not.toBeInTheDocument(); // collapsed by default
    fireEvent.click(url);
    await waitFor(() => expect(screen.getByText(/sekret/)).toBeInTheDocument());
  });

  it("shows an empty hint when there are no flows", async () => {
    (api.listFlows as any).mockResolvedValueOnce([]);
    render(() => <FlowPanel onClose={() => {}} />);
    await waitFor(() => expect(screen.getByText(/No flows yet/)).toBeInTheDocument());
  });

  it("edits raw YAML and surfaces validation messages", async () => {
    render(() => <FlowPanel onClose={() => {}} />);
    fireEvent.click(await screen.findByText("signup"));
    fireEvent.click(await screen.findByText("Edit"));
    const ta = await screen.findByDisplayValue(/name: signup/);
    expect(api.readFlowRaw).toHaveBeenCalledWith("/c/.senda/flows/signup.flow.yaml");
    fireEvent.input(ta, { target: { value: "name: signup\nstart: login\nnodes: {}\nX" } });
    await waitFor(() => expect(screen.getByText(/targets missing node/)).toBeInTheDocument());
  });

  it("saves the edited flow", async () => {
    render(() => <FlowPanel onClose={() => {}} />);
    fireEvent.click(await screen.findByText("signup"));
    fireEvent.click(await screen.findByText("Edit"));
    await screen.findByDisplayValue(/name: signup/);
    fireEvent.click(screen.getByText("Save"));
    await waitFor(() =>
      expect(api.saveFlowRaw).toHaveBeenCalledWith("/c/.senda/flows/signup.flow.yaml", expect.stringContaining("name: signup")),
    );
  });

  it("deletes the selected flow after confirm", async () => {
    render(() => <FlowPanel onClose={() => {}} />);
    fireEvent.click(await screen.findByText("signup"));
    fireEvent.click(await screen.findByText("Delete"));
    await waitFor(() =>
      expect(api.deleteFlow).toHaveBeenCalledWith("/c/.senda/flows/signup.flow.yaml"),
    );
  });

  it("creates a new flow and opens it in the editor", async () => {
    render(() => <FlowPanel onClose={() => {}} />);
    await screen.findByText("signup");
    fireEvent.click(screen.getByText("New flow"));
    await waitFor(() => expect(api.createFlow).toHaveBeenCalledWith("/c", "newflow"));
  });
});
