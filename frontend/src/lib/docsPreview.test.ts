import { describe, expect, it } from "vitest";
import { docsSrcdoc } from "./docsPreview";

describe("docsSrcdoc", () => {
  it("embeds the rendered HTML body verbatim inside a full document", () => {
    const out = docsSrcdoc("<h1>Create user</h1><p><strong>Auth</strong></p>");
    expect(out).toContain("<!DOCTYPE html>");
    expect(out).toContain("<style>");
    expect(out).toContain("<h1>Create user</h1>");
    expect(out).toContain("<strong>Auth</strong>");
  });

  it("falls back to dark theme colors when CSS vars are unset (jsdom)", () => {
    // jsdom getComputedStyle returns no custom props → defaults apply.
    expect(docsSrcdoc("")).toContain("#111215");
  });
});
