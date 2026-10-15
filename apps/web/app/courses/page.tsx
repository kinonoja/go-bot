import Link from "next/link";
import { Badge, Card, DifficultyBadge, PageTitle, XpBar } from "@/components/ui";
import { apiFetch } from "@/lib/api-server";
import { getCourses, getProgress } from "@/lib/api-client";
import { rankForXp } from "@/lib/ranks";

export default async function CoursesPage() {
  const courses = await getCourses(await apiFetch("courses"));
  let progress: Awaited<ReturnType<typeof getProgress>> | null = null;
  try { progress = await getProgress(await apiFetch("me/progress")); } catch (error) {
    if (!(error instanceof Error) || !("status" in error) || error.status !== 401) throw error;
  }
  const completed = new Set(progress?.completed_lessons.map((lesson) => `${lesson.course_slug}/${lesson.chapter_slug}/${lesson.lesson_slug}`));
  const rank = rankForXp(progress?.total_xp ?? 0);
  return <main className="shell page-wrap">
    <PageTitle>Course library</PageTitle>
    <p className="muted">Pick a lesson and build your Go skills one small win at a time.</p>
    {progress ? <Card className="progress-card">
      <div><strong>{rank.name}</strong><span className="muted"> · Level {rank.level} · {progress.total_xp} XP</span></div>
      <XpBar total={progress.total_xp} min={rank.minXp} next={rank.nextXp} />
      {rank.nextXp !== null && <span className="muted">{rank.nextXp - progress.total_xp} XP to {rank.name === "Concurrency Knight" ? "Go Master" : "the next rank"}</span>}
    </Card> : <Card className="progress-card"><strong>Track your progress</strong><span className="muted">Continue as a learner in the header to save completions and earn XP.</span></Card>}
    <div className="course-list">{courses.map((course) => <Card className="course-card" key={course.slug}>
      <h2>{course.title}</h2><p className="muted">{course.description}</p>
      {course.chapters.map((chapter) => <section className="chapter" key={chapter.slug}>
        <h3>{chapter.title}</h3><p className="muted">{chapter.description}</p>
        {chapter.lessons.map((lesson) => {
          const done = completed.has(`${course.slug}/${chapter.slug}/${lesson.slug}`);
          return <Link className="lesson-row" key={lesson.slug} href={`/learn/${course.slug}/${chapter.slug}/${lesson.slug}`}>
            <span className="lesson-name">{done && <span className="tick" aria-label="Completed">✓ </span>}{lesson.title}</span>
            <span className="lesson-meta"><DifficultyBadge difficulty={lesson.difficulty} />{lesson.boss && <Badge className="boss-badge">Boss</Badge>}<span className="muted">{lesson.xp} XP</span></span>
          </Link>;
        })}
      </section>)}
    </Card>)}</div>
  </main>;
}
