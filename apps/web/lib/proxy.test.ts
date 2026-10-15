import { describe, expect, it } from "vitest";
import { proxyHeaders, validDevEmail } from "@/lib/proxy";

describe("proxy header rules", () => {
  it("forwards content type and no browser headers", () => {
    const headers = proxyHeaders("application/json", false, "learner@example.test");
    expect([...headers.entries()]).toEqual([["content-type", "application/json"]]);
  });
  it.each([
    { email: "learner@example.test", valid: true },
    { email: "missing-at.example.test", valid: false },
    { email: "learner@invalid", valid: false },
    { email: "a".repeat(90) + "@example.test", valid: false },
    { email: undefined, valid: false },
  ])("validates dev email $email", ({ email, valid }) => {
    expect(validDevEmail(email)).toBe(valid);
    const headers = proxyHeaders("application/json", true, email);
    expect(headers.has("x-dev-user-email")).toBe(valid);
  });
  it("adds no auth header when dev auth is disabled", () => {
    expect(proxyHeaders(null, false, "learner@example.test").has("x-dev-user-email")).toBe(false);
  });
});
