import { describe, expect, it } from "vitest";
import { codeStorageKey, readSavedCode, saveCode } from "@/lib/code-storage";

function memoryStorage(initial: Record<string, string> = {}) {
  const values = new Map(Object.entries(initial));
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
  };
}

describe("code storage", () => {
  it("uses a versioned per-lesson key", () => {
    expect(codeStorageKey("go", "start", "hello")).toBe("gobot:code:v1:go:start:hello");
  });
  it("uses starter when no code is saved", () => {
    expect(readSavedCode(memoryStorage(), "lesson", "starter" )).toBe("starter");
  });
  it("saves and reads a string", () => {
    const storage = memoryStorage();
    saveCode(storage, "lesson", "package main");
    expect(readSavedCode(storage, "lesson", "starter")).toBe("package main");
  });
  it.each(["{broken", "42", "null", "{}"])("falls back for corrupt value %s", (value) => {
    expect(readSavedCode(memoryStorage({ lesson: value }), "lesson", "starter")).toBe("starter");
  });
});
