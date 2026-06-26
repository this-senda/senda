import { test, expect } from "@playwright/test";

// Smoke coverage for the three bugs that jsdom unit tests couldn't see, run in
// a real WebKit engine. Backend is the in-memory dev mock, so we assert the
// UI interaction happened (the part that was actually broken), not disk state.

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  // The mock seeds a collection; wait for the tree to render.
  await expect(page.locator(".tree-leaf", { hasText: "comments" })).toBeVisible();
});

test("a new scratch request can be saved (save-as prompt)", async ({ page }) => {
  // Fresh scratch tab, titled "New request".
  await page.locator(".tab-new").click();
  const activeTitle = page.locator(".tab.active .tab-title");
  await expect(activeTitle).toHaveText("New request");

  // Dirty it (method change flips the dirty flag), which reveals Save. The verb
  // picker is a custom dropdown (#43), not a native <select>: open it, pick PUT.
  await page.locator("button.method-inline").click();
  await page.locator(".method-menu .method-opt", { hasText: "PUT" }).click();
  await expect(page.locator(".url-icon-btn.dirty")).toBeVisible();

  // Save → in-app prompt (custom <Dialog/>, not native window.prompt) → write.
  // Fill the dialog input and confirm; the tab adopts the new name and goes clean.
  await page.keyboard.press("Control+s");
  await page.locator(".dlg-input").fill("smoke-req");
  await page.locator(".dlg-ok").click();
  await expect(activeTitle).toHaveText("smoke-req");
  await expect(page.locator(".tab.active .tab-dot.on")).toHaveCount(0);
});

test("dragging a request starts a drag and highlights the drop folder", async ({ page }) => {
  const leaf = page.locator(".tree-leaf", { hasText: "comments" });
  const folder = page.locator(".tree-folder", { hasText: "auth" });

  const from = await leaf.boundingBox();
  const to = await folder.boundingBox();
  if (!from || !to) throw new Error("row not found");

  // Pointer-based drag: press on the request, move past the threshold onto the
  // folder. If the drag never started (the old HTML5/WebKitGTK bug) this would
  // select text and the folder would never get .drop-hover.
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
  await page.mouse.down();
  await page.mouse.move(to.x + to.width / 2, to.y + to.height / 2, { steps: 8 });
  await expect(folder).toHaveClass(/drop-hover/);
  await page.mouse.up();
});

test("collection settings: proxy + mTLS network config edits, browse-fills and persists", async ({ page }) => {
  // Open the collection overflow menu → Collection settings (the root modal —
  // proxy/TLS lives here, NOT in per-folder settings).
  await page.locator(".coll-overflow").click();
  await page.locator(".ctx-item", { hasText: "Collection settings" }).click();

  // Network section renders (regression guard: it was first put in the wrong
  // modal where it was unreachable).
  await expect(page.locator(".modal-section-label", { hasText: "Network" })).toBeVisible();

  // Type a proxy + a {{var}} cert path (free-text must stay editable).
  await page.getByPlaceholder(/Proxy URL/).fill("http://corp:8080");
  await page.getByPlaceholder(/Client cert file/).fill("{{certVar}}");

  // Browse fills the key field from the native picker (mock returns a path).
  await page
    .locator(".net-file", { has: page.getByPlaceholder(/Client key file/) })
    .locator("button", { hasText: "Browse" })
    .click();
  await expect(page.getByPlaceholder(/Client key file/)).toHaveValue("/picked/client-cert.pem");

  // Toggle insecure, then save.
  await page.locator(".net-insecure input[type=checkbox]").check();
  await page.locator(".modal-foot .btn:not(.ghost)").click();
  await expect(page.locator(".modal-title")).toHaveCount(0);

  // Reopen → values persisted through the store (proves the save wired the new
  // fields, and that they reload into the form).
  await page.locator(".coll-overflow").click();
  await page.locator(".ctx-item", { hasText: "Collection settings" }).click();
  await expect(page.getByPlaceholder(/Proxy URL/)).toHaveValue("http://corp:8080");
  await expect(page.getByPlaceholder(/Client cert file/)).toHaveValue("{{certVar}}");
  await expect(page.getByPlaceholder(/Client key file/)).toHaveValue("/picked/client-cert.pem");
  await expect(page.locator(".net-insecure input[type=checkbox]")).toBeChecked();
});

test("Ctrl+Shift+Tab switches to the previous tab", async ({ page }) => {
  // Open two requests → two tabs, second one active.
  await page.locator(".tree-leaf", { hasText: "comments" }).click();
  await page.locator(".tree-leaf", { hasText: "create-user" }).click();
  await expect(page.locator(".tab.active .tab-title")).toHaveText("create-user");

  await page.keyboard.press("Control+Shift+Tab");
  await expect(page.locator(".tab.active .tab-title")).toHaveText("comments");
});
