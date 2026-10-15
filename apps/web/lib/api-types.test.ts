import { describe, expect, it } from "vitest";
import { coursesSchema, lessonSchema, progressSchema, submitSchema } from "@/lib/api-types";

const coursePayload = [{ slug: "go", title: "Go Basics", description: "Learn Go", chapters: [{ slug: "start", order: 1, title: "Start", description: "Begin here", lessons: [{ slug: "hello", order: 1, title: "Hello", xp: 20, difficulty: "easy", boss: false, body: "# Hello", starter_files: { "main.go": "package main" }, expected_tests: ["TestHello"] }] }] }];

describe("API response schemas", () => {
  it("accepts realistic course, lesson, submit, and progress responses", () => {
    expect(coursesSchema.parse(coursePayload)).toHaveLength(1);
    expect(lessonSchema.parse({ slug: "hello", title: "Hello", body: "# Hello", starter_files: { "main.go": "package main" }, xp: 20, difficulty: "easy" }).slug).toBe("hello");
    expect(submitSchema.parse({ passed: true, status: "ok", tests: [{ name: "TestHello", passed: true }], missing_tests: [], xp_awarded: 22, newly_completed: true, duration_ms: 35 }).passed).toBe(true);
    expect(progressSchema.parse({ completed_lessons: [{ course_slug: "go", chapter_slug: "start", lesson_slug: "hello", completed_at: "2026-01-01T00:00:00Z", xp_awarded: 22 }], total_xp: 22 }).total_xp).toBe(22);
  });
  it("rejects mismatched API payloads", () => {
    expect(() => submitSchema.parse({ passed: true, tests: null })).toThrow();
    expect(() => coursesSchema.parse([{ slug: 9 }])).toThrow();
  });
});
