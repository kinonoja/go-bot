"use client";

import dynamic from "next/dynamic";
import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Lesson, SubmitResponse } from "@/lib/api-types";
import { codeStorageKey, readSavedCode, saveCode } from "@/lib/code-storage";
import { statusHeadline } from "@/lib/status";
import { Badge, Button, Card, DifficultyBadge, XpBar } from "@/components/ui";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { ZodError } from "zod";

const Editor = dynamic(() => import("@monaco-editor/react"), { ssr: false, loading: () => <div className="editor-host" aria-label="Loading code editor" /> });

type Neighbor = { href: string; title: string } | null;

export function LessonWorkspace({ course, chapter, lesson, data, previous, next }: { course: string; chapter: string; lesson: string; data: Lesson & { boss?: boolean }; previous: Neighbor; next: Neighbor }) {
  const starter = data.starter_files["main.go"] || "package main\n\nfunc main() {\n\t\n}\n";
  const storageKey = useMemo(() => codeStorageKey(course, chapter, lesson), [course, chapter, lesson]);
  const [code, setCode] = useState(starter);
  const [tab, setTab] = useState("lesson");
  const [result, setResult] = useState<SubmitResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [httpStatus, setHttpStatus] = useState<number | undefined>();
  const [schemaError, setSchemaError] = useState(false);
  const [split, setSplit] = useState(42);
  const [xpAnimate, setXpAnimate] = useState(false);
  const dragRef = useRef(false);

  useEffect(() => { setCode(readSavedCode(localStorage, storageKey, starter)); }, [storageKey, starter]);
  useEffect(() => { saveCode(localStorage, storageKey, code); }, [storageKey, code]);

  const run = useCallback(async () => {
    if (loading) return;
    setLoading(true); setResult(null); setHttpStatus(undefined);
    try {
      const response = await fetch(`/api/courses/${course}/lessons/${chapter}/${lesson}/submit`, {
        method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ files: { "main.go": code } }),
      });
      setHttpStatus(response.status);
      if (!response.ok) return;
      const { getSubmission } = await import("@/lib/api-client");
      const submission = await getSubmission(response);
      setResult(submission);
      if (submission.newly_completed) setXpAnimate(true);
      setTab("results");
    } catch (error) {
      if (error instanceof ZodError) setSchemaError(true);
      else setHttpStatus(502);
    } finally { setLoading(false); }
  }, [chapter, code, course, lesson, loading]);

  useEffect(() => {
    function shortcut(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key === "Enter") { event.preventDefault(); void run(); }
    }
    window.addEventListener("keydown", shortcut);
    return () => window.removeEventListener("keydown", shortcut);
  }, [run]);

  useEffect(() => {
    function move(event: PointerEvent) {
      if (!dragRef.current) return;
      setSplit(Math.min(68, Math.max(28, event.clientX / window.innerWidth * 100)));
    }
    function end() { dragRef.current = false; }
    window.addEventListener("pointermove", move); window.addEventListener("pointerup", end);
    return () => { window.removeEventListener("pointermove", move); window.removeEventListener("pointerup", end); };
  }, []);

  function reset() {
    if (window.confirm("Reset your code to the starter? Your saved edits will be replaced.")) setCode(starter);
  }
  const headline = result ? statusHeadline(result.status, result.passed, result.tests.length > 0) : httpStatus ? statusHeadline("", false, false, httpStatus) : null;

  if (schemaError) throw new Error("The submission response did not match the expected format");

  return <main className="learn-shell" data-tab={tab}>
    <div className="mobile-tabs" role="tablist" aria-label="Lesson workspace">
      {[["lesson", "Lesson"], ["code", "Code"], ["results", "Results"]].map(([key, label]) => <Button key={key} variant={tab === key ? "primary" : "secondary"} role="tab" aria-selected={tab === key} onClick={() => setTab(key)}>{label}</Button>)}
    </div>
    <section className="lesson-pane" style={{ flexBasis: `${split}%` }}>
      <div className="lesson-header-meta"><DifficultyBadge difficulty={data.difficulty} />{data.boss && <Badge className="boss-badge">Boss</Badge>}<span>{data.xp} XP reward</span></div>
      <h1 className="lesson-title">{data.title}</h1>
      <article className="markdown"><ReactMarkdown remarkPlugins={[remarkGfm]}>{data.body}</ReactMarkdown></article>
      <nav className="lesson-nav" aria-label="Lesson navigation">{previous ? <Link href={previous.href}>← {previous.title}</Link> : <span />}{next && <Link href={next.href}>{next.title} →</Link>}</nav>
    </section>
    <div className="splitter" role="separator" aria-label="Resize lesson and editor panes" aria-orientation="vertical" tabIndex={0} onPointerDown={() => { dragRef.current = true; }} onKeyDown={(event) => { if (event.key === "ArrowLeft") setSplit((value) => Math.max(28, value - 3)); if (event.key === "ArrowRight") setSplit((value) => Math.min(68, value + 3)); }} />
    <div className="work-pane">
      <section className="editor-pane" aria-label="Go code editor">
        <div className="pane-heading"><h2>main.go</h2><div className="run-controls"><Button variant="ghost" onClick={reset}>Reset</Button><Button onClick={() => void run()} disabled={loading}>{loading ? "Running…" : "Run tests"}</Button></div></div>
        {loading && <div className="run-note" role="status">Running your code. This can take several seconds.</div>}
        <div className="editor-host"><Editor language="go" theme="vs-dark" value={code} onChange={(value) => setCode(value ?? "")} options={{ minimap: { enabled: false }, fontFamily: "var(--font-mono)", fontSize: 14, padding: { top: 16 }, automaticLayout: true, tabSize: 4, scrollBeyondLastLine: false }} /></div>
      </section>
      <section className="results-pane" aria-label="Test results" aria-live="polite" aria-atomic="false">
        <div className="pane-heading"><h2>Results</h2>{result && <span className="muted">{result.duration_ms} ms</span>}</div>
        <div className="results-content">
          {!result && !httpStatus && <p className="muted">Run your code to see test results here.</p>}
          {headline && <h3>{headline}</h3>}
          {httpStatus === 429 || httpStatus === 503 ? <p className="notice notice-warning">Please wait about 2 seconds before submitting again.</p> : null}
          {httpStatus === 502 && <p className="notice">Please try again later.</p>}
          {result?.stderr && <><p className="muted">The compiler reported:</p><pre className="code-output">{result.stderr}</pre></>}
          {result?.tests.map((test, index) => <div key={`${test.name}-${index}`} className={`test-result ${test.passed ? "pass" : "fail"}`}><span aria-hidden="true">{test.passed ? "✓" : "×"}</span><strong>{test.name}</strong>{!test.passed && test.output && <pre className="code-output result-output">{test.output}</pre>}</div>)}
          {result && result.missing_tests.length > 0 && <p className="notice notice-warning">Some checks did not run: {result.missing_tests.join(", ")}</p>}
          {result?.newly_completed && <Card className="success-card"><h3>Lesson complete</h3><p className="pass">+{result.xp_awarded} XP</p><XpBar total={result.xp_awarded} min={0} next={result.xp_awarded} animate={xpAnimate} />{next && <Link className="button button-primary" href={next.href}>Next lesson: {next.title}</Link>}</Card>}
          {result?.passed && !result.newly_completed && <p className="notice">You already completed this lesson. No XP was added.</p>}
        </div>
      </section>
    </div>
  </main>;
}
