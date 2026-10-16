Read AGENTS.md first. This file adds context and current status.

# Go Bot: Claude Code context

## What this is

Go Bot is a Go-only coding tutor with graded lessons, XP, ranks, and a planned AI tutor using the Claude API. The repository describes the learning product and phases; its dark visual theme is implemented in the web app. The origin remote is `https://github.com/kinonoja/go-bot.git`.

[Unverified] This is a one-developer portfolio project implemented with an AI assistant. A premium dark UI is a product goal beyond the dark theme visible in the current code.

## Architecture in 10 lines

1. `services/runner/` is a Go HTTP service; it defaults to port 8080 and executes submissions in Docker.
2. `services/api/` is a Go HTTP service backed by PostgreSQL; it defaults to port 8081.
3. `apps/web/` is a Next.js app; its package scripts use port 3000.
4. The web app forwards browser `/api/*` requests to the API origin, defaulting to `http://localhost:8081`.
5. Ordered course and lesson content lives under `content/courses/`.
6. Lesson grader source lives in each lesson's `tests/main_test.go`; solutions live in `solution/main.go`.
7. The browser submits student files to the Next.js API proxy, which forwards them to the Go API.
8. The API loads trusted grader files and expected test names from lesson content, then sends these with student files to the runner.
9. The runner validates the request, writes a temporary workspace, runs it in a constrained Docker container, and returns parsed test results.
10. The API records the submission, awards completion XP, and returns result data to the browser; grader source and solutions are not included in lesson response structs.

The course catalog currently exposes `expected_tests` names in its public lesson view. Keep grader source and solutions private, and review whether exposing grader test names is acceptable.

## Runner contract

`POST /run` accepts a JSON object with `files` (required student filename-to-source map), optional `grader_files` (trusted `_test.go` sources, test mode only), optional `expected_tests` (test names, test mode only), `mode` (`run` or `test`), and positive `timeout_ms`. Validation also limits combined sources to 10 files and 100 KB.

The response has `status` (`ok`, `compile_error`, `timeout`, or `runtime_error`), `stdout`, `stderr`, `duration_ms`, optional `tests` (`name`, `passed`, `output`), and `missing_tests` (always present). Exit code 97 is reserved by the fixed build command as the build-failure signal and maps to `compile_error`; student code exiting 97 also maps to that status.

The image prebuilds Go packages into `/opt/gocache`. Each submission copies that cache to its own writable `/tmp/gocache`; do not mount a shared writable cache. The sandbox runs with `--network none`, `--memory 512m`, `--cpus 0.5`, `--pids-limit 64`, `--read-only`, `--cap-drop ALL`, `--security-opt no-new-privileges`, `--user sandbox`, and executable `/tmp` tmpfs. Student and grader code currently share one Go test process, so student code can interfere with its own grader.

## Security rules

- Keep the runner reachable only from the API service; it currently listens on all interfaces by default, so deployment networking must restrict access.
- Never return grader source files or solutions through public endpoints.
- `DEV_AUTH` is for local development only and must never be true in a deployed environment. Real authentication is not implemented.
- Never interpolate submitted values into a shell command. Keep shell command strings fixed and pass other values as process arguments.
- Never commit secrets, local environment files, credentials, or production data.
- Do not add external network requests to web runtime flows. Verify editor and font assets are served locally before claiming offline operation.

## Environment and gotchas

The developer has confirmed that work belongs in WSL Ubuntu's Linux checkout at `/home/kingsolomon/go-bot`, never in `/mnt/c/Users/USER/go-bot`; the latter is a stale backup and must not be edited. After reboot, Docker Engine may need `sudo service docker start`, and the developer's Linux account must belong to the `docker` group.

The checkout inspected for this guide currently uses `next/font/google` in `apps/web/app/layout.tsx`; it does not contain the claimed Geist package font import or a Monaco asset-copy script. `apps/web/README.md` is only a placeholder. Therefore offline font/editor behavior is not verified for this checkout. The web app uses Next.js development mode, which compiles pages when first visited.

## How to run

Run commands from `/home/kingsolomon/go-bot` unless a command changes directory. Docker and PostgreSQL run inside WSL.

Start PostgreSQL and apply migrations:

```bash
cp deploy/.env.example deploy/.env
docker compose -f deploy/docker-compose.yml up -d db
set -a
. deploy/.env
set +a
make migrate-up
```

Build and start the runner in its own terminal:

```bash
docker build -t gobot-sandbox:latest services/runner/sandbox/
cd services/runner && go run ./cmd/runner
```

Start the API in another terminal with development auth enabled locally:

```bash
cd services/api && GOCACHE=/tmp/gocache-build DATABASE_URL='postgres://gobot:replace-me@localhost:5432/gobot?sslmode=disable' RUNNER_URL=http://localhost:8080 CONTENT_DIR=../../content/courses DEV_AUTH=true go run ./cmd/api
```

Start the web app in another terminal:

```bash
cd apps/web && DEV_AUTH=true API_URL=http://localhost:8081 npm run dev
```

The browser's development login stores a `dev_email` cookie. A valid cookie is needed for the web proxy to forward the development user identity and submit code successfully. Do not use development auth in a deployed environment.

Check lessons against the runner:

```bash
cd services/api && GOCACHE=/tmp/gocache-build go run ./cmd/lessoncheck -content ../../content/courses -runner-url http://localhost:8080
```

Run Go unit tests:

```bash
cd services/runner && GOCACHE=/tmp/gocache-build go test ./...
cd services/api && GOCACHE=/tmp/gocache-build go test ./...
```

Run Docker-backed runner integration tests after building the image and starting Docker:

```bash
cd services/runner && go test -count=1 -tags integration ./internal/integration/
```

Run PostgreSQL store integration tests with the local database migrated and `DATABASE_URL` loaded from `deploy/.env`:

```bash
cd services/api && set -a && . ../../deploy/.env && set +a && go test -tags integration ./internal/store/
```

Run web checks:

```bash
cd apps/web && npm install
cd apps/web && npm run lint
cd apps/web && npm run typecheck
cd apps/web && npm test
```

## Conventions

Use Go 1.23 module/workspace targets and prefer the standard library. Add table-driven tests for packages. Write no code comments. Make one commit per logical change with a clear message. Existing tags are `runner-v1`, `runner-v2`, `runner-v3`, and `lessons-v1`. Keep prompts and changes small. Report real command output and distinguish verified results from assumptions.

## Current status and issues

The runner service, validation, Docker execution, result parsing, and runner tests are present. The API has a lesson loader, one Go Fundamentals chapter with four lessons, PostgreSQL persistence and migrations, submission grading, XP awards, and progress reads. The Next.js app has a home page, course catalog, lesson workspace, development login, and API proxy. Production authentication, leaderboard/streak features, and the AI tutor are not implemented.

The root layout already mounts a shared header with the Go Bot home link, Courses link, theme toggle, and DEV_AUTH-gated continue-as control, including on lesson pages. Do not describe that header as missing in this checkout.

Confirmed issues and limits in the inspected checkout:

- `XpBar` is exported from `apps/web/components/ui.tsx` and imports `motion` from `framer-motion`; server components import this shared UI module. [Unverified] A runtime `createMotionComponent` crash is plausible but was not verified by a production build or browser run.
- The lesson submission UI has friendly messages for 429, 502, and 503, but `statusHeadline` has no 401 case; a 401 currently displays the generic “Submission received” heading.
- `PublicLesson` includes `expected_tests`, so the public course catalog exposes grader test names. Grader source files and solutions are omitted from public lesson response structs.
- The runner binds to `:8080`; enforce private networking so only the API can reach it.
- `next/font/google` is used and no tracked local font or Monaco copy script exists in this checkout. Offline asset behavior needs implementation or verification.
- No real authentication exists; API authenticated-user handling rejects requests when `DEV_AUTH` is false.
- [Unverified] A read-only runner security audit is not represented in the inspected repository; audit completion is unverified.
- [Unverified] A recent `npm audit` report mentioning `postcss` through Next.js and `dompurify` through Monaco was supplied by the developer, but it has not been rechecked in this checkout.
- [Unverified] Submission run time of about 5–7 seconds and a future warm container pool were supplied as developer-reported context; timing was not measured here and no pool is implemented.
- No deployment procedure is documented in this checkout.

Verification performed while preparing this guide: all 34 web unit tests passed. API unit tests reached test packages but failed when the sandbox denied `httptest` local sockets. Runner unit packages passed, but `go test ./...` exited with a read-only Go cache trim error. Web lint failed after scanning generated `.next` output and `next-env.d.ts`; web typecheck could not write `tsconfig.tsbuildinfo` because the checkout is read-only. Docker integration tests, PostgreSQL integration tests, `npm audit`, production build, and browser flow were not run.

## Next tasks in order

1. Check the `XpBar` client/server boundary and retain the existing shared header behavior.
2. Verify the browser flow end to end, including the 401 message, dev cookie, and submission results.
3. Add real authentication with argon2id password hashing, secure session cookies, CSRF protection, and login rate limiting.
4. Add leaderboard and streaks.
5. Add the Claude API tutor with tiered hints.
6. Expand the course to twenty real lessons.
7. Complete a read-only security audit of the runner.
8. Prepare and verify deployment, including private runner networking and production authentication configuration.

## Working with the developer

The developer is a junior engineer working in Ubuntu. Give short, exact commands with their working directories, and explain what output to paste back. Be direct about what a command did and did not verify. Never claim something works unless command output demonstrates it.
