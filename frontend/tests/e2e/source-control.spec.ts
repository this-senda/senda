import { test, expect } from "@playwright/test";

// Read-only git comparison modal. Backend is the in-memory mock (GitStatus/
// GitDiff seeded in mock-backend.mjs), so this asserts the comparison UX wires
// up: changed list with status badges, external-changes bucket, and the
// semantic per-field diff appearing when a row is selected.

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await expect(page.locator(".tree-leaf", { hasText: "comments" })).toBeVisible();
});

test("source control lists changes and shows a semantic diff on select", async ({ page }) => {
  await page.locator(".scm-open").click();

  // changed requests render with status badges; untracked dotfile buckets under
  // the External section.
  const rows = page.locator(".scm-row");
  await expect(rows).toHaveCount(3);
  await expect(page.locator(".scm-row", { hasText: "create-user" }).locator(".scm-badge")).toHaveText("modified");
  await expect(page.locator(".scm-section-head", { hasText: "External file changes" })).toBeVisible();

  // nothing selected yet → placeholder
  await expect(page.locator(".scm-diff-empty")).toHaveText("Select a change to view diff");

  // select the modified request → per-field diff with old/new blocks
  await page.locator(".scm-row", { hasText: "create-user" }).click();
  await expect(page.locator(".scm-field-label", { hasText: "Method" })).toBeVisible();
  await expect(page.locator(".scm-field", { hasText: "Method" }).locator(".scm-old")).toHaveText("GET");
  await expect(page.locator(".scm-field", { hasText: "Method" }).locator(".scm-new")).toHaveText("POST");

  // a non-request file falls back to a raw text diff
  await page.locator(".scm-row", { hasText: ".gitignore" }).click();
  await expect(page.locator(".scm-raw")).toContainText("node_modules");
});
