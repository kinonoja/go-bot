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
  "mode": "run",
  "timeout_ms": 5000
}
```

| Field        | Type              | Description                                      |
|--------------|-------------------|--------------------------------------------------|
| `files`      | `map[string]string` | File name to source code. Max 10 files.         |
| `mode`       | `string`          | `"run"` or `"test"`.                             |
| `timeout_ms` | `int`             | Hard timeout in milliseconds. Must be positive.  |

**Response**

```json
{
  "status": "ok",
  "stdout": "hello\n",
  "stderr": "",
  "duration_ms": 312,
  "tests": [
    {
      "name": "TestAdd",
      "passed": true,
      "output": "=== RUN   TestAdd\n--- PASS: TestAdd (0.00s)\n"
    }
  ]
}
```

| Field         | Type           | Description                                              |
|---------------|----------------|----------------------------------------------------------|
| `status`      | `string`       | `"ok"`, `"compile_error"`, `"timeout"`, `"runtime_error"` |
| `stdout`      | `string`       | Program stdout, capped at 64 KB.                         |
| `stderr`      | `string`       | Program stderr, capped at 64 KB.                         |
| `duration_ms` | `int`          | Wall-clock execution time in milliseconds.               |
| `tests`       | `[]TestResult` | Present only when `mode` is `"test"`.                    |

When `mode` is `"test"`, the runner executes `go test -json ./...` and parses
the JSON event stream into the `tests` array.

### GET /healthz

Returns `{"status": "ok"}` with HTTP 200.

## Validation Rules

- `mode` must be `"run"` or `"test"`.
- At least one file required, at most 10.
- File names must not contain `/`, `\`, or `..`.
- File names must not be absolute paths.
- Total payload size must not exceed 100 KB.
- `timeout_ms` must be positive.
- When the concurrency semaphore is full, returns HTTP 429.

## Sandbox Restrictions

Each submission runs in a throwaway Docker container built from
`sandbox/Dockerfile` (`golang:1.23-alpine`, non-root `sandbox` user,
`CGO_ENABLED=0`, `GOFLAGS=""`, and `GOPROXY=off`).

The image builds a warm Go cache in `/opt/gocache` using the same Go
environment as submissions. Each container copies that cache into its own
writable `/tmp/gocache` tmpfs before running `go run` or `go test`. The cache
is copied per container instead of mounted as a shared writable volume so one
submission cannot change cached build artifacts used by another submission.
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
