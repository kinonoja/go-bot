export const codeStorageKey = (course: string, chapter: string, lesson: string) => `gobot:code:v1:${course}:${chapter}:${lesson}`;

export function readSavedCode(storage: Pick<Storage, "getItem">, key: string, starter: string): string {
  try {
    const value = storage.getItem(key);
    if (value === null) return starter;
    const parsed: unknown = JSON.parse(value);
    return typeof parsed === "string" ? parsed : starter;
  } catch {
    return starter;
  }
}

export function saveCode(storage: Pick<Storage, "setItem">, key: string, code: string): void {
  storage.setItem(key, JSON.stringify(code));
}
