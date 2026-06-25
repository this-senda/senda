import { test, expect } from "@playwright/test";

// Regression: the Body and Docs tabs mount a CodeMirror 6 editor whose
// .cm-editor is height:100%. When the .panes grid had no definite row height
// (implicit `auto` row), CM6 expanded to its full content height, overflowed
// the shell, and scrolled the whole window into a black void below the status
// bar. The fix: a definite `grid-template-rows: minmax(0,1fr)` on .panes plus
// `overflow:hidden` on html/body. This asserts the window can never scroll.

// True if the document root has any vertical overflow (i.e. the window scrolls).
async function windowScrolls(page: import("@playwright/test").Page) {
  return page.evaluate(() => {
    const el = document.documentElement;
    // >1 to ignore sub-pixel rounding.
    return el.scrollHeight - el.clientHeight > 1;
  });
}

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await expect(page.locator(".tree-leaf", { hasText: "create-user" })).toBeVisible();
  // Sanity: a clean shell does not scroll.
  expect(await windowScrolls(page)).toBe(false);
});

test("Body tab (CodeMirror) does not scroll the window", async ({ page }) => {
  await page.locator(".tree-leaf", { hasText: "create-user" }).click();
  await page.locator(".tabs button", { hasText: "Body" }).click();

  // CM editor mounted, and the document still does not overflow.
  await expect(page.locator(".code-editor")).toBeVisible();
  expect(await windowScrolls(page)).toBe(false);
});

test("Docs tab (CodeMirror) does not scroll the window", async ({ page }) => {
  await page.locator(".tree-leaf", { hasText: "create-user" }).click();
  await page.locator(".tabs button", { hasText: "Docs" }).click();

  await expect(page.locator(".code-editor")).toBeVisible();
  expect(await windowScrolls(page)).toBe(false);
});
