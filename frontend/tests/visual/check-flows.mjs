// Opens the Flows panel in the dev mock and screenshots it, to verify padding
// and the node-graph preview render. Chromium; starts `vite --mode test` if down.
import { chromium } from "playwright";
import { spawn } from "node:child_process";

const URL_BASE = process.env.SENDA_URL || "http://127.0.0.1:5173/";
const reachable = async () => {
  try {
    return (await fetch(URL_BASE, { signal: AbortSignal.timeout(1000) })).ok;
  } catch {
    return false;
  }
};
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

const stop = await ensureServer();
const browser = await chromium.launch({ args: ["--no-sandbox", "--force-color-profile=srgb"] });
const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
try {
  await page.goto(URL_BASE);
  await page.locator(".coll-overflow").waitFor({ state: "visible", timeout: 15000 });
  await page.locator(".coll-overflow").click();
  await page.locator(".flow-open").click();
  await page.locator(".flow-panel").waitFor({ state: "visible", timeout: 5000 });
  // first flow auto-selects on open — graph should appear with no click.
  await page.locator(".flow-graph").waitFor({ state: "visible", timeout: 5000 });
  // run it, then open the first request step's response.
  await page.locator(".flow-run").click();
  await page.locator(".flow-steps .flow-step.clickable").first().waitFor({ state: "visible", timeout: 5000 });
  await page.locator(".flow-steps .flow-step.clickable").first().click();
  await page.locator(".flow-step-body").waitFor({ state: "visible", timeout: 5000 });
  await page.locator(".flow-panel").screenshot({ path: "/tmp/senda-flows.png" });
  console.log("OK wrote /tmp/senda-flows.png");
} finally {
  await browser.close();
  stop();
}
