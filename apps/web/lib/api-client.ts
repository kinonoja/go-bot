import { coursesSchema, lessonSchema, progressSchema, submitSchema } from "@/lib/api-types";

async function parsed<T>(response: Response, schema: { parse: (value: unknown) => T }): Promise<T> {
  if (!response.ok) throw Object.assign(new Error(`API request failed with ${response.status}`), { status: response.status });
  return schema.parse(await response.json());
}

export const getCourses = (response: Response) => parsed(response, coursesSchema);
export const getLesson = (response: Response) => parsed(response, lessonSchema);
export const getProgress = (response: Response) => parsed(response, progressSchema);
export const getSubmission = (response: Response) => parsed(response, submitSchema);
