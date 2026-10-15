import { z } from "zod";

export const publicLessonSchema = z.object({
  slug: z.string(), order: z.number().int(), title: z.string(), xp: z.number().int(),
  difficulty: z.enum(["easy", "medium", "hard"]), boss: z.boolean(), body: z.string(),
  starter_files: z.record(z.string(), z.string()), expected_tests: z.array(z.string()),
});
export const publicChapterSchema = z.object({ slug: z.string(), order: z.number().int(), title: z.string(), description: z.string(), lessons: z.array(publicLessonSchema) });
export const courseSchema = z.object({ slug: z.string(), title: z.string(), description: z.string(), chapters: z.array(publicChapterSchema) });
export const coursesSchema = z.array(courseSchema);
export const lessonSchema = z.object({ slug: z.string(), title: z.string(), body: z.string(), starter_files: z.record(z.string(), z.string()), xp: z.number().int(), difficulty: z.enum(["easy", "medium", "hard"]) });
export const submitSchema = z.object({
  passed: z.boolean(), status: z.string(), tests: z.array(z.object({ name: z.string(), passed: z.boolean(), output: z.string().optional() })),
  missing_tests: z.array(z.string()), stderr: z.string().optional(), xp_awarded: z.number().int(),
  newly_completed: z.boolean(), duration_ms: z.number(),
});
export const progressSchema = z.object({ completed_lessons: z.array(z.object({ course_slug: z.string(), chapter_slug: z.string(), lesson_slug: z.string(), completed_at: z.string(), xp_awarded: z.number().int() })), total_xp: z.number().int() });

export type Course = z.infer<typeof courseSchema>;
export type Lesson = z.infer<typeof lessonSchema>;
export type SubmitResponse = z.infer<typeof submitSchema>;
export type Progress = z.infer<typeof progressSchema>;
