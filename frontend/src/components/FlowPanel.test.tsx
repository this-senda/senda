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
          result: { name: "login", path: "login.yaml", method: "GET", url: "https://x/login", status: 200, ok: true, assertPass: 0, assertFail: 0, durationMs: 1, sizeBytes: 0 },
        },
        { nodeId: "check", type: "branch", branch: "true" },
      ]),
    ),
  },
}));
vi.mock("@wailsio/runtime", () => ({ Events: { On: () => () => {} } }));

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

  it("shows an empty hint when there are no flows", async () => {
    (api.listFlows as any).mockResolvedValueOnce([]);
    render(() => <FlowPanel onClose={() => {}} />);
    await waitFor(() => expect(screen.getByText(/No flows yet/)).toBeInTheDocument());
  });
});
