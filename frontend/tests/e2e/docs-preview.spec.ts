import { test, expect } from "@playwright/test";

// Docs tab Edit/Preview toggle. The mock's create-user request carries markdown
// docs (heading + bold); list-users has none. Drives the real WebKit engine
// against the in-memory mock (App.RenderMarkdown stubbed in mock-backend.mjs).

test("Docs preview renders markdown in a sandboxed iframe", async ({ page }) => {
  await page.goto("/");
  await page.locator(".tree-leaf", { hasText: "create-user" }).click();

  await page.locator(".tabs button", { hasText: "Docs" }).click();
  // Edit is the default view: the source editor is visible, no preview iframe.
  await expect(page.locator("iframe.docs-preview")).toHaveCount(0);

  await page.locator(".docs-toolbar button", { hasText: "Preview" }).click();
  const frame = page.frameLocator("iframe.docs-preview");
  await expect(frame.locator("h1")).toHaveText("Create user");
  await expect(frame.locator("strong").first()).toBeVisible();

  // Toggle back to Edit removes the iframe and shows the source again.
  await page.locator(".docs-toolbar button", { hasText: "Edit" }).click();
  await expect(page.locator("iframe.docs-preview")).toHaveCount(0);
});

test("Docs preview shows a placeholder when there are no docs", async ({ page }) => {
  await page.goto("/");
  await page.locator(".tree-leaf", { hasText: "comments" }).click();

  await page.locator(".tabs button", { hasText: "Docs" }).click();
  await page.locator(".docs-toolbar button", { hasText: "Preview" }).click();

  await expect(page.locator("iframe.docs-preview")).toHaveCount(0);
  await expect(page.locator(".docs-hint", { hasText: "Nothing to preview" })).toBeVisible();
});
