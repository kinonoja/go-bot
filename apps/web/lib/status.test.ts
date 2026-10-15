import { describe, expect, it } from "vitest";
import { statusHeadline } from "@/lib/status";

describe("statusHeadline", () => {
  it.each([
    { status: "compile_error", passed: false, hasTests: true, httpStatus: undefined, headline: "Your code did not compile" },
    { status: "ok", passed: true, hasTests: true, httpStatus: undefined, headline: "All tests passed" },
    { status: "runtime_error", passed: false, hasTests: true, httpStatus: undefined, headline: "Some tests failed" },
    { status: "", passed: false, hasTests: false, httpStatus: 429, headline: "Go Bot is busy, try again in a moment" },
    { status: "", passed: false, hasTests: false, httpStatus: 503, headline: "Go Bot is busy, try again in a moment" },
    { status: "", passed: false, hasTests: false, httpStatus: 502, headline: "The code runner is unavailable" },
  ])("maps status $status", ({ status, passed, hasTests, httpStatus, headline }) => {
    expect(statusHeadline(status, passed, hasTests, httpStatus)).toBe(headline);
  });
});
