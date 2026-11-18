import { afterEach, expect, it, vi } from "vitest";
import { readVisibilityDefault, saveVisibilityDefault } from "./visibilityDefaults";

afterEach(() => vi.unstubAllGlobals());

it("keeps defaults scoped to the account and content type", () => {
  const storage = new Map<string, string>();
  vi.stubGlobal("localStorage", {
    getItem: (key: string) => storage.get(key) ?? null,
    setItem: (key: string, value: string) => storage.set(key, value),
  });

  saveVisibilityDefault(1, "video", "private");

  expect(readVisibilityDefault(1, "video")).toBe("private");
  expect(readVisibilityDefault(2, "video")).toBe("public");
  expect(readVisibilityDefault(1, "post")).toBe("public");

  saveVisibilityDefault(1, "video", "public");
  expect(readVisibilityDefault(1, "video")).toBe("public");
});
