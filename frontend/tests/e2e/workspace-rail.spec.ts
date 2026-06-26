import { test, expect } from "@playwright/test";

// Visual-geometry checks for the workspace switcher, in a real WebKit engine —
// the kind of "is the glyph actually centered" question jsdom can't answer.
test.beforeEach(async ({ page }) => {
  await page.goto("/");
  // Mock seeds /demo as lastCollection → it opens as the active workspace.
  await expect(page.locator(".ws-switch")).toBeVisible();
});

// Assert the rendered glyph's center sits on the box's center.
async function assertCentered(box: any, glyph: any, tol = 2.5) {
  const b = await box.boundingBox();
  const g = await glyph.boundingBox();
  if (!b || !g) throw new Error("missing bounding box");
  const dx = g.x + g.width / 2 - (b.x + b.width / 2);
  const dy = g.y + g.height / 2 - (b.y + b.height / 2);
  expect(Math.abs(dx), `dx=${dx.toFixed(2)}`).toBeLessThan(tol);
  expect(Math.abs(dy), `dy=${dy.toFixed(2)}`).toBeLessThan(tol);
}

test("monogram is centered in the switcher avatar", async ({ page }) => {
  const avatar = page.locator(".ws-avatar").first();
  await assertCentered(avatar, avatar.locator(".ws-avatar-mono"));
});

test("a chosen emoji is centered in the avatar", async ({ page }) => {
  const avatar = page.locator(".ws-avatar").first();
  await page.locator(".ws-switch").click({ button: "right" });
  const cell = page.locator(".ws-picker-cell").first();
  await expect(cell).toBeVisible();
  await cell.click();
  const emoji = avatar.locator(".ws-avatar-emoji");
  await expect(emoji).toBeVisible();
  await assertCentered(avatar, emoji);
});

test("the dropdown opens and shows Open collection (regression)", async ({ page }) => {
  await page.locator(".ws-switch").click();
  await expect(page.getByText("Open collection…")).toBeVisible();
});
