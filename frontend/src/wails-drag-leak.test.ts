import { describe, it, expect, vi } from "vitest";

// Guards the test-setup mock of @wailsio/runtime/dist/drag.js. The real module
// schedules window.setInterval(…, 50) at import; that timer fires after jsdom
// teardown and throws "window is not defined", failing the whole run. If the
// mock ever stops intercepting, this test fails (the spy sees the interval).
describe("wails runtime drag leak", () => {
  it("schedules no window.setInterval at import (stubbed)", async () => {
    vi.resetModules();
    const spy = vi.spyOn(window, "setInterval");
    // @ts-expect-error side-effect-only JS module, no type declarations
    await import("../node_modules/@wailsio/runtime/dist/drag.js");
    expect(spy).not.toHaveBeenCalled();
    spy.mockRestore();
  });
});
