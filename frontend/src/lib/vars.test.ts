import { describe, expect, it } from "vitest";
import { varRe, triggerAt, varStatus, splitVars, isSecret } from "./vars";

// Captures every {{...}} token's inner text via the shared grammar.
const tokens = (s: string) => [...s.matchAll(varRe())].map((m) => m[1]);

describe("varRe grammar", () => {
  it("matches plain vars, trimming inner whitespace in capture context", () => {
    expect(tokens("{{base}}/x/{{ id }}")).toEqual(["base", "id"]);
  });

  it("matches faker tokens, namespaced and with params", () => {
    expect(tokens('{"a":"{{$email}}"}')).toEqual(["$email"]);
    expect(tokens("{{$person.firstname}}")).toEqual(["$person.firstname"]);
    expect(tokens("{{$number.int(min=1,max=9)}}")).toEqual(["$number.int(min=1,max=9)"]);
  });

  it("does not match an unopened/closed-only brace", () => {
    expect(tokens("plain text, no braces")).toEqual([]);
  });
});

describe("triggerAt", () => {
  it("returns the prefix inside an open {{", () => {
    expect(triggerAt("{{ba", 4)).toEqual({ start: 2, prefix: "ba" });
  });

  it("offers the empty prefix right after {{", () => {
    expect(triggerAt("{{", 2)).toEqual({ start: 2, prefix: "" });
  });

  it("recognizes a $ faker prefix", () => {
    expect(triggerAt("{{$em", 5)).toEqual({ start: 2, prefix: "$em" });
  });

  it("stops once params are being typed (paren is not a name char)", () => {
    // Autocomplete should close while typing inside (...).
    expect(triggerAt("{{$number.int(min=", 18)).toBeNull();
  });

  it("returns null after the placeholder is closed", () => {
    const v = "{{base}}x";
    expect(triggerAt(v, v.length)).toBeNull();
  });
});

describe("varStatus", () => {
  const scope = new Map([["base", "v"]]);
  it("found for an in-scope var", () => expect(varStatus(scope, "base")).toBe("found"));
  it("found for any $ faker token (resolved at send)", () =>
    expect(varStatus(scope, "$person.firstname")).toBe("found"));
  it("secret for credential-looking names", () =>
    expect(varStatus(scope, "api_token")).toBe("secret"));
  it("missing otherwise", () => expect(varStatus(scope, "nope")).toBe("missing"));
});

describe("splitVars", () => {
  it("splits plain runs from tokens and tags faker tokens as found", () => {
    const segs = splitVars("hi {{$uuid}}!", new Map());
    expect(segs.map((s) => s.text)).toEqual(["hi ", "{{$uuid}}", "!"]);
    expect(segs[1].status).toBe("found");
  });
});

describe("isSecret", () => {
  it("flags credential keywords, case-insensitively", () => {
    expect(isSecret("API_KEY")).toBe(true);
    expect(isSecret("username")).toBe(false);
  });
});
