import { defineConfig, devices } from "@playwright/test";

// E2E smoke run against a real WebKit engine — the closest thing to the
// WebKitGTK runtime the desktop app actually ships, without a native build.
// It catches the class of bugs jsdom can't: drag-and-drop that never starts,
// prompt() handling, real focus/selection behaviour. It does NOT reproduce
// GTK input-layer quirks (e.g. the ISO_Left_Tab keysym) — Playwright
// synthesizes DOM events directly, bypassing GDK. Those need the built binary
// under Xvfb; for the keysym path we rely on src/lib/keymap.test.ts instead.
//
// The app runs against the in-memory dev mock (`vite --mode test` →
// installDevMock), so assertions are interaction-level (a drag starts, a tab
// renames) rather than real disk writes.
export default defineConfig({
  testDir: "./tests/e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL: "http://localhost:5173",
    trace: "on-first-retry",
  },
  projects: [{ name: "webkit", use: { ...devices["Desktop Safari"] } }],
  webServer: {
    // CI runs bun directly; run-podman.sh overrides with a node-launched vite
    // (the playwright image has no bun).
    command: process.env.PW_WEB_CMD ?? "bun run dev:mock",
    url: "http://localhost:5173",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
