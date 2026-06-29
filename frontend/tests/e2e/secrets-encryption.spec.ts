import { test, expect } from "@playwright/test";

// Secret-encryption controls in the Secrets modal. Backend is the in-memory mock
// (EncryptionStatus/EnableEncryption in devMock.ts), so this asserts the toggle
// wires up: enabling flips to the unlocked state and reveals the Export-key
// action. Opens via the collection overflow menu → "Manage secrets".

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await expect(page.locator(".tree-leaf", { hasText: "comments" })).toBeVisible();
});

test("toggling at-rest encryption flips to the unlocked state", async ({ page }) => {
  await page.locator(".coll-overflow").click();
  await page.locator(".secrets-open").click();

  // strip present, encryption off by default
  const toggle = page.locator(".secrets-enc-toggle input");
  await expect(toggle).not.toBeChecked();
  await expect(page.locator(".secrets-enc", { hasText: "Encrypt secret files at rest" })).toBeVisible();

  // enable → unlocked status + Export key action appear
  await toggle.check();
  await expect(page.locator(".secrets-enc-status", { hasText: "unlocked" })).toBeVisible();
  await expect(page.locator(".secrets-enc button", { hasText: "Export key" })).toBeVisible();

  // export reveals the base64 key strip
  await page.locator(".secrets-enc button", { hasText: "Export key" }).click();
  await expect(page.locator(".secrets-enc-key")).not.toBeEmpty();
});
