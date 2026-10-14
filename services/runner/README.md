# Runner Service

Sandboxed Go code execution service for Go Bot.

## API

### POST /run

Executes user-submitted Go code inside an isolated Docker container.

**Request**

```json
{
  "files": {
    "main.go": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}"
  },
  "grader_files": {
    "lesson_test.go": "package main\n\nimport \"testing\"\n\nfunc TestGreeting(t *testing.T) {}"
  },
  "expected_tests": ["TestGreeting"],
  "mode": "test",
  "timeout_ms": 5000
}
```

| Field            | Type                | Description                                                  |
|------------------|---------------------|--------------------------------------------------------------|
| `files`          | `map[string]string` | Required student source files.                               |
| `grader_files`   | `map[string]string` | Optional grader tests; accepted only in `test` mode.         |
| `expected_tests` | `[]string`          | Optional test names expected from lesson grader tests.       |
| `mode`           | `string`            | `"run"` or `"test"`.                                        |
| `timeout_ms`     | `int`               | Hard timeout in milliseconds. Must be positive.              |

Student and grader files share a limit of 10 files and 100 KB total source.
Student file names must end in `.go`; `_test.go`, `go.mod`, `go.work`, and
`go.sum` are reserved. Names cannot be empty, dot-prefixed, contain `..` or
path separators, and student names cannot collide after lower-casing. Grader
files must end in `_test.go`, obey the same path and dot-name restrictions,
and cannot collide with a student file after lower-casing. `expected_tests`
is accepted only in test mode, allows at most 100 unique non-empty names
matching `^Test[A-Za-z0-9_]*$`.

**Response**

```json
{
  "status": "ok",
  "stdout": "hello\n",
  "stderr": "",
  "duration_ms": 312,
  "missing_tests": [],
  "tests": [
    {
      "name": "TestAdd",
      "passed": true,
      "output": "=== RUN   TestAdd\n--- PASS: TestAdd (0.00s)\n"
    }
  ]
}
```

| Field           | Type           | Description                                                |
|-----------------|----------------|------------------------------------------------------------|
| `status`        | `string`       | `"ok"`, `"compile_error"`, `"timeout"`, `"runtime_error"` |
| `stdout`        | `string`       | Program stdout, capped at 64 KB.                           |
| `stderr`        | `string`       | Program stderr, capped at 64 KB.                           |
| `duration_ms`   | `int`          | Wall-clock execution time in milliseconds.                 |
| `tests`         | `[]TestResult` | Present only when `mode` is `"test"`.                      |
| `missing_tests` | `[]string`     | Expected tests without a parsed pass or fail outcome. Always present. |

When `mode` is `"test"`, the runner builds a test binary with `go test -c`,
streams its JSON events through `go tool test2json`, and parses them into the
`tests` array. `missing_tests` contains expected test names that did not
produce a pass or fail result, in the order supplied by `expected_tests`.

The runner builds submissions before executing them. Exit code `97` is
reserved as the shell signal for a build failure and maps to
`compile_error`. A submission that calls `os.Exit(97)` is also reported as
`compile_error`; this affects only that submission's result.

### GET /healthz

Returns `{"status": "ok"}` with HTTP 200.

## Validation Rules

- `mode` must be `"run"` or `"test"`.
- At least one student file is required. Student and grader files combined
  cannot exceed 10 files or 100 KB of source.
- Student files must end in `.go`; they cannot end in `_test.go` or be named
  `go.mod`, `go.work`, or `go.sum`.
- Student and grader names cannot be empty or dot-prefixed, contain `..`, or
  contain `/` or `\`. Student names must be unique after lower-casing.
- `grader_files` is accepted only in test mode. Each grader name must end in
  `_test.go` and cannot collide with a student name after lower-casing.
- `expected_tests` is accepted only in test mode. It allows at most 100
  unique, non-empty names matching `^Test[A-Za-z0-9_]*$`.
- `timeout_ms` must be positive.
- When the concurrency semaphore is full, returns HTTP 429.

## Sandbox Restrictions

Each submission runs in a throwaway Docker container built from
`sandbox/Dockerfile` (`golang:1.23-alpine`, non-root `sandbox` user,
`CGO_ENABLED=0`, `GOFLAGS=""`, and `GOPROXY=off`).

The image builds a warm Go cache in `/opt/gocache` using the same Go
environment as submissions. Its warm-up imports `bufio`, `bytes`, `cmp`,
`container/heap`, `container/list`, `context`, `crypto/sha256`,
`encoding/base64`, `encoding/binary`, `encoding/csv`, `encoding/hex`,
`encoding/json`, `errors`, `flag`, `fmt`, `hash/fnv`, `html/template`, `io`,
`io/fs`, `iter`, `log`, `maps`, `math`, `math/big`, `math/rand`, `net`,
`net/http`, `net/url`, `os`, `os/exec`, `os/signal`, `path/filepath`,
`reflect`, `regexp`, `runtime`, `slices`, `sort`, `strconv`, `strings`, `sync`,
`sync/atomic`, `text/tabwriter`, `text/template`, `time`, `unicode`, and
`unicode/utf8`. Test-only warm-up imports are `net/http/httptest`, `testing`,
`testing/iotest`, and `testing/quick`.

Each container copies that cache into its own writable `/tmp/gocache` tmpfs
before running `go run` or `go test`. The cache is copied per container
instead of mounted as a shared writable volume so one submission cannot
change cached build artifacts used by another submission. Packages not
covered by the warm-up imports or their dependency tree are compiled in each
new container and can make runs slow. To warm another package, add a blank
import for it to `sandbox/warm/main.go` (or `sandbox/warm/main_test.go` if it
is only needed by tests), then rebuild the image with
`docker build -t gobot-sandbox:latest services/runner/sandbox/`.
Container startup and cache copying receive a 10-second allowance beyond the
requested execution timeout.

Docker run flags:

| Flag                       | Purpose                                |
|----------------------------|----------------------------------------|
| `--network none`           | No network access                      |
| `--memory 512m`            | Memory limit                           |
| `--cpus 0.5`               | CPU limit                              |
| `--pids-limit 64`          | Process limit (mitigates fork bombs)   |
| `--read-only`              | Read-only root filesystem              |
| `--user sandbox`           | Run as the unprivileged sandbox user   |
| `--tmpfs /tmp:rw,exec,size=384m` | Writable executable temp/cache area |
| `--cap-drop ALL`           | Drop all Linux capabilities            |
| `--security-opt no-new-privileges` | Prevent privilege escalation   |

User source files are mounted read-only at `/workspace`. A synthetic `go.mod`
is generated for each submission. The workspace directory is mode `0755` and
files are mode `0644` so the sandbox user can read them through the read-only
mount.

If the timeout is reached, the container is killed and removed. Stdout and
stderr are each capped at 64 KB; output beyond the cap is truncated with a
`... output truncated ...` marker.

## Threat Model

**Goal**: Allow untrusted Go code to compile and run without affecting the
host or other submissions.

### Threats and Mitigations

| Threat                    | Mitigation                                                     |
|---------------------------|----------------------------------------------------------------|
| Network exfiltration      | `--network none` blocks all network access.                    |
| File system tampering     | `--read-only` root FS; source mounted read-only.               |
| Resource exhaustion (CPU) | `--cpus 0.5` plus hard timeout with container kill.            |
| Resource exhaustion (RAM) | `--memory 512m` enforced by the container runtime OOM killer.  |
| Fork bomb                 | `--pids-limit 64` caps total processes in the container.       |
| Privilege escalation      | Non-root user, `--cap-drop ALL`, `no-new-privileges`.          |
| Downloading dependencies  | `GOPROXY=off` prevents module fetches; combined with no network.|
| Output flooding           | Stdout and stderr capped at 64 KB each.                        |
| Path traversal in files   | File names validated to reject separators and `..`.            |
| Denial of service         | Concurrency semaphore limits parallel executions; excess → 429.|
| Container leaks           | The runner removes containers on exit and kills them on timeout.|

## Known limitations

Student code and grader tests run in one process, so a determined student can
fake passing output or otherwise interfere with the grader. This is accepted
because grading integrity only affects that student's own progress, while the
sandbox protects the host either way.

The runner must only be reachable from the API service and must never be
exposed to browsers. `grader_files` must come from trusted lesson content and
never from user input.

### Residual Risks

- The Docker daemon runs as root on the host. A container escape exploit in
  the kernel or runc could compromise the host. Running in a VM or using
  gVisor/Kata Containers would add an additional isolation layer.
- Side-channel attacks (timing, cache) are not mitigated by container
  isolation alone.
- Disk I/O through the 384 MB tmpfs is bounded by the container memory limit
  but could still cause temporary pressure on the host.

## Running

```bash
docker build -t gobot-sandbox:latest services/runner/sandbox/

# Start the runner
go run ./cmd/runner --port 8080 --max-concurrent 5

# Run unit tests
go test ./...

# Run integration tests (requires Docker)
go test -tags integration ./internal/integration/

# Sample request
curl -s http://localhost:8080/run \
  -H 'Content-Type: application/json' \
  -d '{"files":{"main.go":"package main\nimport \"fmt\"\nfunc main(){fmt.Println(\"hello\")}"},"mode":"run","timeout_ms":5000}' | jq .
```
