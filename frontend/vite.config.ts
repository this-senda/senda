import { resolve } from "node:path";
import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

export default defineConfig(({ mode }) => ({
  plugins: [solid()],
  server: {
    // wails3 dev injects WAILS_VITE_PORT and points the Go shell's
    // FRONTEND_DEVSERVER_URL at it, so vite must bind that exact port. Falls
    // back to 5173 for a bare `bun run dev`.
    port: Number(process.env.WAILS_VITE_PORT) || 5173,
    strictPort: true,
    // Wails' asset proxy dials tcp4 127.0.0.1; vite's default "localhost"
    // resolves to ::1 (IPv6) on Node 17+, so the proxy gets connection
    // refused and the window stays blank. Pin to IPv4.
    host: "127.0.0.1",
  },
  build: {
    target: "esnext",
    outDir: "dist",
    emptyOutDir: true,
  },
  resolve: {
    // The Wails bindings under frontend/bindings/ are generated from Go and
    // gitignored; unit tests must run without them, so in mode "test" (used
    // by vitest, and by `vite --mode test` for browser runs without the Wails
    // toolchain) bindings imports are pointed at hand-written stubs. Dev and
    // build use the real generated modules. Note: each find must be the exact
    // import specifier — vitest's alias resolution doesn't apply regexes to
    // relative specifiers; all importers sit two levels under src/, so
    // "../../" covers them.
    alias:
      mode === "test"
        ? [
            {
              find: "../../bindings/senda/internal/model/models",
              replacement: resolve(__dirname, "src/test-stubs/models.ts"),
            },
            {
              find: "../../bindings/senda/internal/app/app",
              replacement: resolve(__dirname, "src/test-stubs/app.ts"),
            },
          ]
        : [],
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test-setup.ts"],
    // Playwright owns tests/e2e (real WebKit); keep them out of vitest/jsdom.
    exclude: ["**/node_modules/**", "**/dist/**", "tests/e2e/**"],
  },
}));
