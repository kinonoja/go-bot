# Go Bot

## Project goal

Go Bot teaches the Go programming language through graded coding lessons. The platform is Go only and aims for a Boot.dev-style learning experience.

## Monorepo layout

- `apps/web/`: web frontend; currently a placeholder.
- `services/api/`: lesson API module with the content loader and lessoncheck command; API endpoints remain a placeholder.
- `services/runner/`: HTTP service that validates and runs submitted Go programs in Docker, with unit and integration tests.
- `content/courses/`: ordered Go course, chapter, and lesson content.
- `deploy/`: planned deployment configuration.
- `go.work`: Go workspace for the API and runner modules.

## Lesson content layout

Course content lives under `content/courses/<course-slug>/`. Each course has
a `course.yaml` with `title` and `description`. Chapter directories use
`<NN-chapter-slug>/` and contain `chapter.yaml` with `title` and
`description`. Lesson directories use `<NN-lesson-slug>/` and contain:

```text
lesson.md
starter/main.go
solution/main.go
tests/main_test.go
```

The two-digit numeric prefixes start at `01`, are contiguous, and define
chapter and lesson order. Slugs use lowercase letters, digits, and hyphens.
Lesson Markdown starts with YAML frontmatter for `title`, `xp` (10 through
500), `difficulty` (`easy`, `medium`, or `hard`), and `expected_tests` (one or
more Go test function names). `boss: true` marks a boss lesson and requires at
least 100 XP. Expected test names must each name a function in
`tests/main_test.go`.

The API lesson loader is in `services/api/internal/lessons`. Public lesson
views omit solution and grader files. The internal lesson checker reads the
starter, solution, and grader sources separately.

## Working rules

- Use Go 1.23, prefer the standard library, and keep dependencies minimal.
- Do not write code comments.
- Every package gets table-driven tests.
- Commit after each numbered step with a clear commit message.
- Stay within the task for the current step; do not start later phases without instruction.

## WSL and Docker

Development runs inside WSL Ubuntu at `~/go-bot`. Docker Engine runs inside WSL and may need to be started with `sudo service docker start` before building the sandbox image or running integration tests.

## Commands

Run these from the repository root unless the command says otherwise.

Build the sandbox image:

```bash
docker build -t gobot-sandbox:latest services/runner/sandbox/
```

The sandbox image prebuilds a Go cache at `/opt/gocache` with `CGO_ENABLED=0`,
empty `GOFLAGS`, and `GOPROXY=off`. Each container copies that cache into its
own writable tmpfs at `/tmp/gocache`. Do not share a writable cache volume
between submissions: one submission could change cache contents used by
another.

Run unit tests:

```bash
cd services/runner && go test ./...
```

Run integration tests from `services/runner` (requires Docker and the sandbox image):

```bash
go test -count=1 -tags integration ./internal/integration/
```

Run API unit tests:

```bash
cd services/api && GOCACHE=/tmp/gocache-build go test ./...
```

Check every lesson against the runner at `http://localhost:8080`:

```bash
cd services/api && GOCACHE=/tmp/gocache-build go run ./cmd/lessoncheck -content ../../content/courses -runner-url http://localhost:8080
```

## Planned phases

1. Phase 1 runner service
2. Phase 2 API (Postgres, auth, lessons, submissions)
3. Phase 3 web frontend (Next.js, Monaco, terminal panel)
4. Phase 4 XP, ranks, leaderboard
5. Phase 5 AI tutor using the Claude API
6. Phase 6 polish and deploy

## Current status

Phase 1 runner service is implemented with `POST /run`, `GET /healthz`, request validation, Docker sandbox execution, test output parsing, unit tests, and Docker-backed integration tests. Phase 2 has started with the validated course content loader, a sample Go fundamentals course, and a lessoncheck command. API endpoints, authentication, persistence, and the web app remain unimplemented. Update this section as phases progress.
