# Go Bot

A platform that teaches the Go programming language with graded coding lessons.

## Structure

```
gobot/
  apps/web/            – frontend (coming soon)
  services/api/        – lesson API
  services/runner/     – sandboxed code runner
  content/courses/     – lesson content
  deploy/              – deployment configs
```

## Quick Start

```bash
make test          # run all unit tests
make lint          # run staticcheck
make run-runner    # start the runner service
```