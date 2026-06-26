// Visual capture: drives the running `vite --mode test` UI with a mocked Wails
// backend and writes documentation screenshots plus an animated GIF walkthrough.
//
// Usage:
//   bun run dev:mock                 # terminal 1 — vite --mode test on :5173
//   node tests/visual/shoot.mjs      # terminal 2 — capture
//   bun run shots                    # or: one command, starts vite itself
//
// Env:
//   SENDA_URL     dev server URL (default http://127.0.0.1:5173/)
//   SENDA_CHROME  path to a Chrome/Chromium binary (default: Playwright's own).
//                 Useful where Playwright's browser CDN is blocked — point this
//                 at a Chrome-for-Testing build instead.
//   SENDA_GIF=0   skip the GIF pass.
import { chromium } from "playwright";
import { mkdirSync, writeFileSync } from "node:fs";
import { spawn } from "node:child_process";

const URL_BASE = process.env.SENDA_URL || "http://127.0.0.1:5173/";
const OUT = new URL("./__screenshots__/", import.meta.url).pathname;
mkdirSync(OUT, { recursive: true });

const { installCaptureMock } = await import("./mock-backend.mjs");

// Start `vite --mode test` ourselves if no server is already answering, so this
// script is a single command. Returns a cleanup fn (no-op if a server was
// already running).
async function ensureServer() {
  const reachable = async () => {
    try {
      const r = await fetch(URL_BASE, { signal: AbortSignal.timeout(1000) });
      return r.ok;
    } catch {
      return false;
    }
  };
  if (await reachable()) return () => {};
  const frontendDir = new URL("../../", import.meta.url).pathname;
  const viteBin = frontendDir + "node_modules/.bin/vite";
  const child = spawn(viteBin, ["--mode", "test", "--host", "127.0.0.1"], {
    cwd: frontendDir,
    stdio: "ignore",
    env: { ...process.env, WAILS_VITE_PORT: "5173" },
  });
  child.on("error", (e) => {
    console.error(`failed to start vite (${viteBin}): ${e.message}`);
    process.exit(1);
  });
  for (let i = 0; i < 60; i++) {
    if (await reachable()) return () => child.kill();
    await new Promise((r) => setTimeout(r, 500));
  }
  child.kill();
  throw new Error(`vite did not come up at ${URL_BASE}`);
}

const stopServer = await ensureServer();

const VIEWPORT = { width: 1280, height: 820 };
const launchOpts = { args: ["--no-sandbox", "--force-color-profile=srgb"] };
if (process.env.SENDA_CHROME) launchOpts.executablePath = process.env.SENDA_CHROME;

const browser = await chromium.launch(launchOpts);

// ── helpers ────────────────────────────────────────────────────────────────
const wait = (ms) => new Promise((r) => setTimeout(r, ms));
const results = [];

async function newPage(viewport = VIEWPORT) {
  const page = await browser.newPage({ viewport, deviceScaleFactor: 1 });
  page.setDefaultTimeout(8000);
  await page.addInitScript(installCaptureMock);
  return page;
}

/** Run a named capture step; never let one failure abort the rest. */
async function shot(page, name, fn) {
  try {
    await fn();
    await page.screenshot({ path: OUT + name });
    results.push(`  ✓ ${name}`);
  } catch (e) {
    results.push(`  ✗ ${name} — ${String(e).split("\n")[0]}`);
  }
}

/** Close any open modal / palette and settle. Modals here close on a backdrop
 *  click (Escape is only wired for the command palette), so do both. */
async function dismiss(page) {
  await page.keyboard.press("Escape").catch(() => {});
  const backdrop = page.locator(".modal-backdrop").first();
  if (await backdrop.count().catch(() => 0)) {
    // click the backdrop near its corner — outside the centered modal — to fire onClose
    await backdrop.click({ position: { x: 6, y: 6 }, timeout: 2000 }).catch(() => {});
  }
  await wait(200);
}

async function seedCollection(page, { env = "dev", themeMode = "light" } = {}) {
  await page.evaluate(
    ([env, themeMode]) => {
      localStorage.setItem("senda.lastCollection", "/demo");
      localStorage.setItem("senda.activeEnv", env);
      localStorage.setItem("senda.themeMode", themeMode);
      // Pin a couple of workspaces so the titlebar rail has more than one box:
      // /demo carries a chosen emoji icon, /petstore falls back to its monogram.
      localStorage.setItem(
        "senda.pinnedCollections",
        JSON.stringify([
          { name: "demo-api", path: "/demo" },
          { name: "petstore", path: "/petstore" },
        ]),
      );
      localStorage.setItem("senda.collectionIcons", JSON.stringify({ "/demo": "🚀" }));
    },
    [env, themeMode],
  );
}

const centerTab = (page, label) =>
  page.locator("main.center .tabs button", { hasText: new RegExp(`^${label}`) }).first();

// ── still screenshots ────────────────────────────────────────────────────────
const page = await newPage();

// 01 — empty shell (no collection seeded yet).
await shot(page, "01-empty-shell.png", async () => {
  await page.goto(URL_BASE, { waitUntil: "networkidle" });
  await wait(400);
});

// 02 — collection open: tree + environment switcher.
await shot(page, "02-collection-open.png", async () => {
  await seedCollection(page);
  await page.reload({ waitUntil: "networkidle" });
  await page.getByText("create-user", { exact: true }).waitFor({ timeout: 8000 });
  await wait(300);
});

// 03 — request open in the editor (before sending).
await shot(page, "03-request-open.png", async () => {
  await page.getByText("create-user", { exact: true }).click();
  await wait(400);
});

// 04 — request sent, response viewer populated.
await shot(page, "04-request-response.png", async () => {
  await page.getByRole("button", { name: "Send", exact: true }).first().click();
  await page.getByText("201", { exact: false }).first().waitFor({ timeout: 8000 });
  await wait(400);
});

// 05 — JSON body editor.
await shot(page, "05-body-json.png", async () => {
  await centerTab(page, "Body").click();
  await wait(300);
});

// 19 — {{$faker}} autocomplete in the JSON body (still on the Body tab).
await shot(page, "19-faker-autocomplete.png", async () => {
  await page.locator(".cm-content").click();
  await page.keyboard.press("Control+End");
  // Type the token: the first completion call fires fakerTokens() (fetched async,
  // empty on that first pass). Wait for it to cache, then re-query with Ctrl+Space
  // ("$"/"." aren't word chars, so it won't auto-reopen) to get the faker entries.
  // Screenshot must happen while the popup is up — the editor's store-sync closes
  // it within ~300ms, so shot() captures immediately after waitFor (no dwell).
  await page.keyboard.type("\n{{$person.f");
  await wait(900);
  await page.keyboard.press("Control+Space");
  await page.locator(".cm-tooltip-autocomplete").waitFor({ timeout: 4000 });
});
await page.keyboard.press("Escape").catch(() => {});
// undo the scratch token so later Body-dependent shots stay clean.
await page.keyboard.press("Control+z").catch(() => {});
await page.keyboard.press("Control+z").catch(() => {});

// 06 — request headers.
await shot(page, "06-headers.png", async () => {
  await centerTab(page, "Headers").click();
  await wait(300);
});

// 07 — assertions / Tests tab.
await shot(page, "07-assertions.png", async () => {
  await centerTab(page, "Tests").click();
  await wait(300);
});

// 08 — pre/post-request scripting.
await shot(page, "08-scripting.png", async () => {
  await centerTab(page, "Script").click();
  await wait(300);
});

// 14 — auth tab (captured here while on the tab bar; numbered for README order).
await shot(page, "14-auth.png", async () => {
  await centerTab(page, "Auth").click();
  await wait(300);
});

// 15 — code generation dialog.
await shot(page, "15-code-generation.png", async () => {
  const btn = page.locator('button[title="Generate code"]').first();
  await btn.click({ timeout: 4000 });
  await wait(400);
});
await dismiss(page);

// 12 — mock server panel, running, with routes + request log (newest feature).
await shot(page, "12-mock-server.png", async () => {
  await page.locator('button[title="Collection actions"]').click();
  await page.getByText("Mock server", { exact: true }).click();
  await page.getByText("Mock Server", { exact: true }).waitFor({ timeout: 4000 });
  await page.getByRole("button", { name: "Start", exact: true }).click();
  await page.getByText("● Running", { exact: false }).waitFor({ timeout: 4000 });
  // request-log entries normally stream over a Wails event; click Refresh to
  // pull the sample log so the panel shows live traffic, not "No requests yet".
  await page.getByRole("button", { name: "Refresh", exact: true }).click().catch(() => {});
  await wait(400);
});
// stop the mock so leftover state doesn't bleed into later runs, then close.
await page.getByRole("button", { name: "Stop", exact: true }).click().catch(() => {});
await dismiss(page);

// 13 — environments editor.
await shot(page, "13-environments.png", async () => {
  await page.locator('button[title="Edit environments"]').click();
  await wait(500);
});
await dismiss(page);

// 16 — import dialog.
await shot(page, "16-import.png", async () => {
  await page.locator('button[title="Collection actions"]').click();
  await page.getByText("Import collection", { exact: true }).click();
  await wait(500);
});
await dismiss(page);

// 17 — history panel.
await shot(page, "17-history.png", async () => {
  await page.locator('button[title="History"]').click();
  await wait(500);
});
await dismiss(page);

// 18 — workspace rail: right-click a box to reveal the icon picker.
await shot(page, "18-workspace-rail.png", async () => {
  await page.locator(".ws-box").first().click({ button: "right" });
  await page.locator(".ws-picker").waitFor({ timeout: 4000 });
  await wait(300);
});
// the picker closes on its own backdrop, not the modal backdrop dismiss() uses.
await page.locator(".menu-backdrop").first().click({ position: { x: 6, y: 6 } }).catch(() => {});
await wait(200);

// 09 — command palette.
await shot(page, "09-command-palette.png", async () => {
  await page.keyboard.press("Control+k");
  await page.locator(".palette-input, input.palette-input").first().waitFor({ timeout: 4000 });
  await wait(300);
});
await dismiss(page);

// 10 — appearance modal.
await shot(page, "10-appearance-modal.png", async () => {
  await page.locator('button[title="Appearance"]').click();
  await wait(500);
});

// 11 — full app in the Catppuccin Mocha (dark) theme. The app reads the theme
// from localStorage on mount, so set it there and reload for a deterministic
// result (driving the theme picker by click is flaky once the list scrolls).
await shot(page, "11-theme-catppuccin-mocha.png", async () => {
  await dismiss(page); // close the appearance modal from shot 10
  await page.evaluate(() => {
    localStorage.setItem("senda.themeMode", "dark");
    localStorage.setItem("senda.themeDark", "catppuccin-mocha");
  });
  await page.reload({ waitUntil: "networkidle" });
  // after reload the request tab is restored, so scope to the sidebar tree leaf
  // to avoid matching both the tab and the tree node.
  const leaf = page.locator("aside .tree-leaf", { hasText: "create-user" }).first();
  await leaf.waitFor({ timeout: 8000 });
  await leaf.click();
  await page.getByRole("button", { name: "Send", exact: true }).first().click();
  await page.getByText("201", { exact: false }).first().waitFor({ timeout: 8000 });
  await wait(400);
});

await page.close();

// ── GIF walkthrough ──────────────────────────────────────────────────────────
if (process.env.SENDA_GIF !== "0") {
  try {
    await captureGif();
    results.push("  ✓ walkthrough.gif");
  } catch (e) {
    results.push(`  ✗ walkthrough.gif — ${String(e).split("\n")[0]}`);
  }
}

await browser.close();
stopServer();
console.log("\nCapture complete:\n" + results.join("\n") + "\n");

// ── GIF implementation ───────────────────────────────────────────────────────
// A screen-recording-style walkthrough: an animated cursor glides between
// targets, clicks pulse, text types in character by character, and transitions
// (response render, theme switch) are captured frame-by-frame. Frames are
// delta-encoded (unchanged pixels become transparent) so the extra motion stays
// small on disk.
async function captureGif() {
  const gifenc = await import("gifenc");
  const { GIFEncoder, quantize, applyPalette } = gifenc.default ?? gifenc;
  const { PNG } = await import("pngjs");

  const W = 960, H = 600;
  const gp = await browser.newPage({ viewport: { width: W, height: H }, deviceScaleFactor: 1 });
  gp.setDefaultTimeout(8000);
  await gp.addInitScript(installCaptureMock);

  // recorded frames: { buf, delay }
  const frames = [];
  const FPS_DELAY = 55; // ms per motion frame (~18 fps)
  let cx = W - 70, cy = H - 60; // cursor start, lower-right

  // Inject a fake pointer (headless has no visible cursor) that we move in step
  // with the real mouse so hover/click states still fire.
  const injectCursor = () =>
    gp.evaluate(([x, y]) => {
      let el = document.getElementById("__shot_cursor");
      if (!el) {
        el = document.createElement("div");
        el.id = "__shot_cursor";
        el.style.cssText =
          "position:fixed;left:0;top:0;width:22px;height:22px;z-index:2147483647;" +
          "pointer-events:none;will-change:transform;filter:drop-shadow(0 1px 1px rgba(0,0,0,.4));";
        el.innerHTML =
          "<svg width='22' height='22' viewBox='0 0 24 24' fill='none'>" +
          "<path d='M5 3l14 7-6 1.6L9.7 19z' fill='white' stroke='black' stroke-width='1.3' stroke-linejoin='round'/></svg>";
        document.body.appendChild(el);
      }
      el.style.transform = `translate(${x}px,${y}px)`;
    }, [cx, cy]);

  const setCursor = (x, y, scale = 1) =>
    gp.evaluate(
      ([x, y, s]) => {
        const el = document.getElementById("__shot_cursor");
        if (el) el.style.transform = `translate(${x}px,${y}px) scale(${s})`;
      },
      [x, y, scale],
    );

  const snap = async (delay = FPS_DELAY) => frames.push({ buf: await gp.screenshot({ type: "png" }), delay });
  // Hold the current frame for a beat (single frame, long delay — cheap).
  const dwell = async (ms = 750) => snap(ms);
  const easeInOut = (t) => (t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2);

  // Glide the cursor (and real mouse) from its current spot to (x,y).
  const glide = async (x, y, steps = 14) => {
    const x0 = cx, y0 = cy;
    for (let i = 1; i <= steps; i++) {
      const t = easeInOut(i / steps);
      cx = x0 + (x - x0) * t;
      cy = y0 + (y - y0) * t;
      await Promise.all([setCursor(cx, cy), gp.mouse.move(cx, cy)]);
      await snap();
    }
    cx = x; cy = y;
  };
  const centerOf = async (locator) => {
    const b = await locator.boundingBox();
    return b ? { x: b.x + b.width / 2, y: b.y + b.height / 2 } : null;
  };
  // Move to an element, press down (cursor dips), click, release.
  const clickEl = async (locator, { settle = 250 } = {}) => {
    const c = await centerOf(locator);
    if (!c) return false;
    await glide(c.x, c.y);
    await setCursor(cx, cy, 0.85); await snap(40); // press
    await gp.mouse.click(c.x, c.y);
    await setCursor(cx, cy, 1); await snap(40);     // release
    if (settle) await wait(settle);
    return true;
  };
  // Capture `n` frames over a live animation (response render, theme cross-fade).
  const film = async (n = 6, gap = 45) => {
    for (let i = 0; i < n; i++) { await snap(gap); await wait(gap); }
  };
  // Type into the focused element one character at a time, filming keystrokes.
  const typeOut = async (text) => {
    for (const ch of text) {
      await gp.keyboard.type(ch);
      await snap(70);
    }
  };

  // ── scenario ───────────────────────────────────────────────────────────────
  await gp.goto(URL_BASE, { waitUntil: "networkidle" });
  await seedCollection(gp);
  await gp.reload({ waitUntil: "networkidle" });
  await gp.getByText("create-user", { exact: true }).waitFor({ timeout: 8000 });
  await injectCursor();
  await dwell(800); // collection open

  // workspace rail: right-click the active box and pick a new emoji icon
  const wsBox = gp.locator(".ws-box").first();
  const wsC = await centerOf(wsBox);
  if (wsC) {
    await glide(wsC.x, wsC.y);
    await gp.mouse.click(wsC.x, wsC.y, { button: "right" });
    await gp.locator(".ws-picker").waitFor({ timeout: 4000 }).catch(() => {});
    await film(3);
    const cell = gp.locator(".ws-picker-cell").nth(4); // a different emoji
    if (await cell.count()) { await clickEl(cell, { settle: 120 }); await film(4); }
    await gp.locator(".menu-backdrop").first().click({ position: { x: 6, y: 6 } }).catch(() => {});
    await wait(150);
  }

  // open a request
  await clickEl(gp.locator("aside .tree-leaf", { hasText: "create-user" }).first());
  await dwell(600);

  // send it and watch the response come in
  await clickEl(gp.getByRole("button", { name: "Send", exact: true }).first(), { settle: 0 });
  await film(7); // response renders
  await gp.getByText("201", { exact: false }).first().waitFor({ timeout: 8000 });
  await dwell(850);

  // peek at the assertions
  await clickEl(gp.locator("main.center .tabs button", { hasText: /^Tests/ }).first());
  await dwell(850);

  // {{$faker}} autocomplete: open Body, type a token, film the dropdown
  await clickEl(gp.locator("main.center .tabs button", { hasText: /^Body/ }).first());
  await dwell(400);
  await clickEl(gp.locator(".cm-content").first(), { settle: 120 });
  await gp.keyboard.press("Control+End");
  await gp.keyboard.type("\n");
  await typeOut("{{$person.f"); // first pass kicks off the faker fetch
  await wait(900);             // let it cache
  await gp.keyboard.press("Control+Space"); // re-query -> faker entries
  await gp.locator(".cm-tooltip-autocomplete").waitFor({ timeout: 4000 }).catch(() => {});
  await film(4); // hold on the dropdown before it auto-closes
  await gp.keyboard.press("Escape");
  await wait(150);

  // command palette: open and type to filter
  await gp.keyboard.press("Control+k");
  await gp.locator(".palette-input, input.palette-input").first().waitFor({ timeout: 4000 });
  await film(3, 60);
  await typeOut("comments");
  await dwell(900);
  await gp.keyboard.press("Escape");
  await wait(150);

  // theme switch — film the cross-fade
  await clickEl(gp.locator('button[title="Appearance"]').first());
  await film(4);
  const darkBtn = gp.locator("button.appearance-mode-btn").nth(1);
  if (await darkBtn.count()) { await clickEl(darkBtn, { settle: 120 }); await film(5); }
  const mocha = gp.locator("button.appearance-item", { hasText: /^Catppuccin Mocha$/ }).first();
  if (await mocha.count()) {
    await mocha.scrollIntoViewIfNeeded();
    await clickEl(mocha, { settle: 120 });
    await film(6); // theme cross-fade
  }
  await gp.locator(".modal-backdrop").first().click({ position: { x: 6, y: 6 } }).catch(() => {});
  await wait(200);
  await glide(W / 2, H / 2, 10);
  await dwell(1200); // final hold on the themed app

  // ── encode (delta frames: unchanged pixels → transparent) ───────────────────
  const enc = GIFEncoder();
  let prev = null, dims = null;
  for (let f = 0; f < frames.length; f++) {
    const png = PNG.sync.read(frames[f].buf);
    dims = { width: png.width, height: png.height };
    const data = new Uint8Array(png.data.buffer, png.data.byteOffset, png.data.length);
    const palette = quantize(data, 255); // reserve one slot for transparency
    const index = applyPalette(data, palette);
    const transparentIndex = palette.length; // ≤ 255
    if (prev) {
      let changed = false;
      for (let p = 0; p < index.length; p++) {
        const o = p * 4;
        if (data[o] === prev[o] && data[o + 1] === prev[o + 1] && data[o + 2] === prev[o + 2]) {
          index[p] = transparentIndex; // unchanged → previous pixel shows through
        } else changed = true;
      }
      if (!changed) index[0] = transparentIndex; // pure dwell: hold the frame
      enc.writeFrame(index, png.width, png.height, {
        palette: [...palette, [0, 0, 0]],
        delay: frames[f].delay,
        transparent: true,
        transparentIndex,
        dispose: 1,
      });
    } else {
      enc.writeFrame(index, png.width, png.height, { palette, delay: frames[f].delay, dispose: 1 });
    }
    prev = data;
  }
  enc.finish();
  writeFileSync(OUT + "walkthrough.gif", Buffer.from(enc.bytes()));
  // Optional: dump a few raw frames as PNG for visual inspection of the motion.
  if (process.env.SENDA_GIF_DUMP) {
    for (const i of [12, 28, 60, 90, frames.length - 1]) {
      if (frames[i]) writeFileSync(`/tmp/gifframe-${i}.png`, frames[i].buf);
    }
  }
  await gp.close();
  if (dims) results.push(`    (gif ${frames.length} frames @ ${dims.width}×${dims.height}, delta-encoded)`);
}
