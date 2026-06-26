import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@solidjs/testing-library";

// Mock the Go binding boundary. switchCollection/openCollectionDialog flow
// through refreshCollection -> openCollection + listEnvironments + activity.
vi.mock("../lib/api", () => ({
  api: {
    openCollection: vi.fn((p: string) => Promise.resolve({ name: "opened", path: p })),
    listEnvironments: vi.fn(() => Promise.resolve([])),
    collectionActivity: vi.fn(() => Promise.resolve({})),
    pickDirectory: vi.fn(() => Promise.resolve("/new/coll")),
    pickZipCollection: vi.fn(() => Promise.resolve("")),
  },
}));

import { api } from "../lib/api";
import WorkspaceRail, { monogram } from "./WorkspaceRail";
import { collectionIcon, ensurePinned, pinned, setCollection, unpin } from "../lib/store";

beforeEach(() => {
  localStorage.clear();
  vi.clearAllMocks();
  setCollection(null);
  for (const p of [...pinned()]) unpin(p.path); // drain the module-level signal
});

describe("monogram", () => {
  it("takes initials of two words, else first two letters", () => {
    expect(monogram("train-travel-api")).toBe("TT");
    expect(monogram("petstore")).toBe("PE");
    expect(monogram("My API")).toBe("MA");
  });
});

describe("WorkspaceRail", () => {
  it("shows the active collection as the switcher pill", () => {
    ensurePinned("train-travel-api", "/a");
    setCollection({ name: "train-travel-api", path: "/a" } as any);
    render(() => <WorkspaceRail />);
    expect(screen.getByText("TT")).toBeInTheDocument(); // avatar monogram
    expect(screen.getByText("train-travel-api")).toBeInTheDocument(); // pill name
  });

  it("opening the dropdown and clicking another workspace switches", async () => {
    ensurePinned("train-travel-api", "/a");
    ensurePinned("petstore", "/b");
    setCollection({ name: "train-travel-api", path: "/a" } as any);
    render(() => <WorkspaceRail />);

    fireEvent.click(screen.getByTitle("Switch workspace"));
    fireEvent.click(screen.getByText("petstore"));
    await waitFor(() => expect(api.openCollection).toHaveBeenCalledWith("/b"));
  });

  // Regression guard: the dropdown must open and actually trigger the folder
  // picker (a CSS overflow:hidden once clipped this menu invisible).
  it("the dropdown opens and 'Open collection…' invokes the picker", async () => {
    render(() => <WorkspaceRail />);
    fireEvent.click(screen.getByTitle("Switch workspace"));
    fireEvent.click(screen.getByText("Open collection…"));
    await waitFor(() => expect(api.pickDirectory).toHaveBeenCalled());
    await waitFor(() => expect(api.openCollection).toHaveBeenCalledWith("/new/coll"));
  });

  it("right-click sets an emoji icon that replaces the monogram", () => {
    ensurePinned("petstore", "/p");
    setCollection({ name: "petstore", path: "/p" } as any);
    render(() => <WorkspaceRail />);
    fireEvent.contextMenu(screen.getByText("PE")); // right-click the pill avatar
    fireEvent.click(screen.getByText("🚆"));
    expect(collectionIcon("/p")).toBe("🚆");
    expect(screen.queryByText("PE")).not.toBeInTheDocument();
    expect(screen.getByText("🚆")).toBeInTheDocument();
  });
});
