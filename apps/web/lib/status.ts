export function statusHeadline(status: string, passed = false, hasTests = false, httpStatus?: number): string {
  if (httpStatus === 429 || httpStatus === 503) return "Go Bot is busy, try again in a moment";
  if (httpStatus === 502) return "The code runner is unavailable";
  if (status === "compile_error") return "Your code did not compile";
  if (passed) return "All tests passed";
  if (hasTests) return "Some tests failed";
  if (status === "runner_error") return "The code runner could not run this submission";
  return "Submission received";
}
