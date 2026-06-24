// Namespaced {{$category.name}} faker tokens, fetched once from the Go backend
// (gofakeit's catalog) so the editor autocomplete never drifts from what the
// pipeline can actually resolve at send time. The accessor is synchronous
// (returns [] until the first fetch lands, then the cached list) because
// CodeMirror completion sources run synchronously.
import { api } from "./api";
import type { Token } from "../../bindings/senda/internal/fake/models";

let cache: Token[] = [];
let started = false;

export function fakerTokens(): Token[] {
  if (!started) {
    started = true;
    api
      .fakerTokens()
      .then((t) => {
        cache = t ?? [];
      })
      .catch(() => {});
  }
  return cache;
}
