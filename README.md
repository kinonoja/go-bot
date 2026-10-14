# Go Bot

Go Bot teaches Go through short lessons graded by a sandboxed runner.

## Structure

- `services/api/`: course API, lesson loader, Postgres store, and runner client
- `services/runner/`: sandboxed Go code runner
- `content/courses/`: ordered lessons, starter code, solutions, and grader tests
- `deploy/`: local Postgres configuration
- `apps/web/`: planned Next.js frontend

## Local API Setup

The API uses PostgreSQL, the runner, and the lesson files. From the repository root:

```bash
cp deploy/.env.example deploy/.env
docker compose -f deploy/docker-compose.yml up -d db
set -a
. deploy/.env
set +a
make migrate-up
```

After loading the same `DATABASE_URL`, roll back the latest migration with `make migrate-down`.

Start the runner in one terminal:

```bash
cd services/runner && go run ./cmd/runner
```

Start the API in another:

```bash
cd services/api && DATABASE_URL='postgres://gobot:replace-me@localhost:5432/gobot?sslmode=disable' RUNNER_URL=http://localhost:8080 CONTENT_DIR=../../content/courses DEV_AUTH=true go run ./cmd/api
```

The API listens on port 8081 by default. `DEV_AUTH=true` enables development authentication using `X-Dev-User-Email`; production authentication is not implemented. The runner should only be reachable from the API service.

Example requests:

```bash
curl http://localhost:8081/healthz
curl http://localhost:8081/api/courses
curl http://localhost:8081/api/courses/go-fundamentals/lessons/getting-started/hello-world
curl -H 'X-Dev-User-Email: learner@example.test' http://localhost:8081/api/me/progress
curl -X POST http://localhost:8081/api/courses/go-fundamentals/lessons/getting-started/hello-world/submit -H 'Content-Type: application/json' -H 'X-Dev-User-Email: learner@example.test' -d '{"files":{"main.go":"package main\nfunc main() {}"}}'
```

## Checks

```bash
(cd services/api && GOCACHE=/tmp/gocache-build go test ./...)
(cd services/api && GOCACHE=/tmp/gocache-build go vet ./...)
(cd services/runner && go test ./...)
(cd services/runner && go vet ./...)
```
