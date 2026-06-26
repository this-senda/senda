// Geometry check for the workspace rail: measures whether the monogram and a
// chosen emoji are actually centered in their box. Chromium (WebKit needs
// system deps this host lacks). Starts `vite --mode test` if not already up.
import { chromium } from "playwright";
import { spawn } from "node:child_process";

const URL_BASE = process.env.SENDA_URL || "http://127.0.0.1:5173/";

async function reachable() {
  try {
    const r = await fetch(URL_BASE, { signal: AbortSignal.timeout(1000) });
    return r.ok;
  } catch {
    return false;
  }
}
async function ensureServer() {
  if (await reachable()) return () => {};
  const frontendDir = new URL("../../", import.meta.url).pathname;
  const child = spawn(frontendDir + "node_modules/.bin/vite", ["--mode", "test", "--host", "127.0.0.1"], {
    cwd: frontendDir,
    stdio: "ignore",
    env: { ...process.env, WAILS_VITE_PORT: "5173" },
  });
  for (let i = 0; i < 60; i++) {
    if (await reachable()) return () => child.kill();
    await new Promise((r) => setTimeout(r, 500));
  }
  child.kill();
  throw new Error("vite did not come up");
}

const center = (b) => ({ x: b.x + b.width / 2, y: b.y + b.height / 2 });
async function offset(box, glyph) {
  const b = await box.boundingBox();
  const g = await glyph.boundingBox();
  return { dx: center(g).x - center(b).x, dy: center(g).y - center(b).y, b, g };
}

const stop = await ensureServer();
const browser = await chromium.launch({ args: ["--no-sandbox", "--force-color-profile=srgb"] });
const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
let failed = false;
try {
  await page.goto(URL_BASE);
  const box = page.locator(".ws-avatar").first();
  await box.waitFor({ state: "visible", timeout: 15000 });

  const mono = await offset(box, box.locator(".ws-avatar-mono"));
  console.log(`monogram: dx=${mono.dx.toFixed(2)} dy=${mono.dy.toFixed(2)}  box=${JSON.stringify(mono.b)}`);

  await page.locator(".ws-switch").click({ button: "right" });
  await page.locator(".ws-picker-cell").first().click();
  const emojiLoc = box.locator(".ws-avatar-emoji");
  await emojiLoc.waitFor({ state: "visible", timeout: 5000 });
  const emo = await offset(box, emojiLoc);
  console.log(`emoji:    dx=${emo.dx.toFixed(2)} dy=${emo.dy.toFixed(2)}`);

  await page.screenshot({ path: "/tmp/senda-rail.png", clip: { x: 0, y: 0, width: 1280, height: 56 } });

  const TOL = 2.5;
  for (const [name, o] of [["monogram", mono], ["emoji", emo]]) {
    if (Math.abs(o.dx) > TOL || Math.abs(o.dy) > TOL) {
      console.log(`FAIL ${name} off-center (tol ${TOL})`);
      failed = true;
    } else {
      console.log(`OK   ${name} centered`);
    }
  }
} finally {
  await browser.close();
  stop();
}
process.exit(failed ? 1 : 0);
