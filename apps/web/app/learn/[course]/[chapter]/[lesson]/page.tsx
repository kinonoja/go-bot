import { notFound } from "next/navigation";
import { LessonWorkspace } from "@/components/lesson-workspace";
import { apiFetch } from "@/lib/api-server";
import { getCourses, getLesson } from "@/lib/api-client";

type Params = { course: string; chapter: string; lesson: string };

export default async function LearnPage({ params }: { params: Promise<Params> }) {
  const { course, chapter, lesson } = await params;
  const [data, courses] = await Promise.all([
    getLesson(await apiFetch(`courses/${course}/lessons/${chapter}/${lesson}`)),
    getCourses(await apiFetch("courses")),
  ]);
  const all = courses.flatMap((item) => item.chapters.flatMap((part) => part.lessons.map((entry) => ({ item, part, entry }))));
  const currentIndex = all.findIndex(({ item, part, entry }) => item.slug === course && part.slug === chapter && entry.slug === lesson);
  if (currentIndex < 0) notFound();
  const previousEntry = all[currentIndex - 1];
  const nextEntry = all[currentIndex + 1];
  const makeNeighbor = (entry: typeof previousEntry) => entry ? {
    href: `/learn/${entry.item.slug}/${entry.part.slug}/${entry.entry.slug}`,
    title: entry.entry.title,
  } : null;
  const catalogLesson = all[currentIndex].entry;
  return <LessonWorkspace
    course={course}
    chapter={chapter}
    lesson={lesson}
    data={{ ...data, boss: catalogLesson.boss }}
    previous={makeNeighbor(previousEntry)}
    next={makeNeighbor(nextEntry)}
  />;
}
