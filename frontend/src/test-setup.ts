import { afterAll, vi } from "vitest";
import "@testing-library/jest-dom";

// Belt-and-suspenders for the drag.js leak below: the module mock is racy (it
// keys on one resolved path and can be slipped under some suite orderings),
// letting the real ~5s setInterval poll survive into jsdom teardown and throw
// "window is not defined", failing the whole run nondeterministically. Track
// every interval and clear them when a file's tests finish — afterAll runs
// before environment teardown, so no timer outlives the jsdom window.
const realSetInterval = globalThis.setInterval;
const liveIntervals = new Set<ReturnType<typeof setInterval>>();
globalThis.setInterval = ((fn: TimerHandler, ms?: number, ...args: unknown[]) => {
  const id = realSetInterval(fn as () => void, ms, ...args);
  liveIntervals.add(id);
  return id;
}) as typeof setInterval;
afterAll(() => {
  for (const id of liveIntervals) clearInterval(id);
  liveIntervals.clear();
});

// @wailsio/runtime/dist/drag.js is a side-effect-only module (no exports) that
// schedules a window.setInterval at import; under jsdom that timer can fire
// after the environment is torn down, throwing "window is not defined" as an
// unhandled error that fails the whole run. Anything pulling the runtime
// (store.ts → Events, the generated bindings → Call) imports it transitively.
// Mock it to an empty module via a relative filesystem path: the package's
// exports field doesn't expose the ./dist/drag.js subpath (a bare specifier
// won't resolve), but a path resolves to the same module id as the internal
// "./drag.js" import. Nothing depends on its (nonexistent) exports.
vi.mock("../node_modules/@wailsio/runtime/dist/drag.js", () => ({}));

// jsdom may not provide crypto.randomUUID (store.ts uses it for tab ids).
if (typeof globalThis.crypto?.randomUUID !== "function") {
  Object.defineProperty(globalThis.crypto, "randomUUID", {
    value: () =>
      "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
        const r = (Math.random() * 16) | 0;
        return (c === "x" ? r : (r & 0x3) | 0x8).toString(16);
      }),
    configurable: true,
  });
}
