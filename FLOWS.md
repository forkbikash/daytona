# Daytona System Flows Documentation

This document provides detailed code-level traces of all major system flows in Daytona, from entry point through all layers to completion.

---

## Table of Contents

1. [Sandbox Creation Flow](#1-sandbox-creation-flow)
2. [Sandbox Start/Stop Flow](#2-sandbox-startstop-flow)
3. [Command Execution Flow](#3-command-execution-flow)
4. [File Upload/Download Flow](#4-file-uploaddownload-flow)
5. [SSH Access Flow](#5-ssh-access-flow)
6. [Preview URL Flow](#6-preview-url-flow)
7. [Snapshot Creation Flow](#7-snapshot-creation-flow)
8. [Code Interpreter Flow](#8-code-interpreter-flow)
9. [Git Operations Flow](#9-git-operations-flow)
10. [Computer Use Flow](#10-computer-use-flow)
11. [Volume Management Flow](#11-volume-management-flow)
12. [Authentication Flows](#12-authentication-flows)

---

## 1. Sandbox Creation Flow

### Overview
Creates a new sandbox from a snapshot or Dockerfile, provisions it on a runner, and starts the daemon.

### Flow Diagram

```
SDK: Daytona.create()
    │
    ▼
API Server: POST /sandbox
    │
    ├─► SandboxController.createSandbox()
    │       │
    │       ▼
    │   SandboxService.createFromSnapshot() / createFromBuildInfo()
    │       │
    │       ├─► Validate organization quotas
    │       ├─► Check warm pool for pre-created sandbox
    │       ├─► Select runner via RunnerService.getRandomAvailableRunner()
    │       ├─► Create Sandbox entity in database
    │       └─► Emit SandboxCreatedEvent
    │
    ▼
SandboxManager (async via event/cron)
    │
    ├─► syncInstanceState()
    │       │
    │       ▼
    │   SandboxStartAction.run()
    │       │
    │       ├─► Pull snapshot to runner (if needed)
    │       └─► RunnerAdapter.createSandbox()
    │
    ▼
Runner: Job Execution
    │
    ├─► DockerClient.Create()
    │       │
    │       ├─► PullImage() - Download snapshot from registry
    │       ├─► getContainerConfigs() - CPU, memory, volumes
    │       ├─► ContainerCreate() - Create Docker container
    │       └─► Start() - Start container
    │
    ▼
Daemon (inside container)
    │
    ├─► Start Toolbox server (port 2280)
    ├─► Start SSH server (port 22)
    ├─► Start Terminal server (port 22222)
    └─► Execute entrypoint command
```

### Code Path

| Step | Component | File | Function | Line |
|------|-----------|------|----------|------|
| 1 | SDK | `libs/sdk-python/src/daytona/_sync/daytona.py` | `Daytona.create()` | 311 |
| 2 | SDK | `libs/sdk-python/src/daytona/_sync/daytona.py` | `_sandbox_api.create_sandbox()` | 396 |
| 3 | API | `apps/api/src/sandbox/controllers/sandbox.controller.ts` | `createSandbox()` | 277 |
| 4 | API | `apps/api/src/sandbox/services/sandbox.service.ts` | `createFromSnapshot()` | 318 |
| 5 | API | `apps/api/src/sandbox/services/runner.service.ts` | `getRandomAvailableRunner()` | 622 |
| 6 | API | `apps/api/src/sandbox/managers/sandbox.manager.ts` | `syncInstanceState()` | 378 |
| 7 | API | `apps/api/src/sandbox/managers/sandbox-actions/sandbox-start.action.ts` | `run()` | 49 |
| 8 | API | `apps/api/src/sandbox/runner-adapter/runnerAdapter.v2.ts` | `createSandbox()` | 115 |
| 9 | Runner | `apps/runner/pkg/runner/v2/executor/sandbox.go` | `createSandbox()` | 16 |
| 10 | Runner | `apps/runner/pkg/docker/create.go` | `Create()` | 25 |
| 11 | Runner | `apps/runner/pkg/docker/start.go` | `Start()` | 21 |
| 12 | Daemon | `apps/daemon/cmd/daemon/main.go` | `main()` | 27 |

### State Transitions

```
PENDING → CREATING → RESTORING → STARTED
    │         │          │
    ▼         ▼          ▼
  ERROR    ERROR      ERROR
```

### Key Implementation Details

**Runner Selection Algorithm:**
- Uses TOPSIS (Technique for Order of Preference by Similarity to Ideal Solution)
- Metrics: CPU usage, memory usage, disk usage, allocation ratios
- Returns random runner from top-scoring candidates

**Warm Pool Optimization:**
- Pre-created sandboxes for instant deployment
- Checked before creating new sandbox
- Significantly reduces creation latency

### Layer Responsibilities

**SDK Layer** (`libs/sdk-python/src/daytona/_sync/daytona.py`)
- **Input validation:** Validates timeout (must be non-negative), auto_stop_interval, auto_archive_interval (all must be >= 0)
- **Parameter transformation:** Converts `CreateSandboxFromSnapshotParams` or `CreateSandboxFromImageParams` to `CreateSandbox` DTO. Transforms declarative `Image` objects to build context hashes via `SnapshotService.process_image_context()`. Maps volumes to `SandboxVolume` DTOs. Converts resources (CPU, memory, disk, GPU) to sandbox creation fields.
- **Build log streaming:** If state is `PENDING_BUILD` and `on_snapshot_create_logs` callback provided, streams logs asynchronously while polling. Polls every 1 second until sandbox leaves PENDING_BUILD state.
- **State polling:** Polls sandbox state at **100ms intervals** via `refresh_data()` until state == `started` or error states (`error`, `build_failed`) reached.
- **Timeout enforcement:** Default 60 seconds via `@with_timeout` decorator (ThreadPoolExecutor-based). Timeout of 0 means no timeout.
- **Error wrapping:** `@intercept_errors` decorator converts `OpenApiException` → `DaytonaError`, 404 → `DaytonaNotFoundError`, 429 → `DaytonaRateLimitError`.

**API Layer** (`apps/api/src/sandbox/`)
- **Controller** (`controllers/sandbox.controller.ts`): Validates `CreateSandboxDto` via class-validator decorators. Rejects if both `buildInfo` AND `snapshot` specified. Rejects resource specs when using snapshot. Permission check: `WRITE_SANDBOXES` decorator. Audit logging enabled with `MASKED_AUDIT_VALUE` for env vars. Waits up to 30s for STARTED state via Redis event channel subscription.
- **Service** (`services/sandbox.service.ts`): `createFromSnapshot()` validates snapshot exists, checks organization quotas (per-sandbox CPU/memory/disk maxes, region-level quota enforcement via `OrganizationUsageService`), increments pending usage counters, checks warm pool for pre-created sandbox, selects runner via TOPSIS algorithm through `RunnerService.getRandomAvailableRunner()`, creates Sandbox entity in database with state=PULLING_SNAPSHOT and desiredState=STARTED, emits `SandboxCreatedEvent`. Rolls back pending usage on failure. `createFromBuildInfo()` additionally creates BuildInfo entity, generates snapshotRef hash, creates snapshot via `SnapshotService`, sets state=PENDING_BUILD.
- **Runner Service** (`services/runner.service.ts`): `findAvailableRunners()` filters runners by state=READY, unschedulable=false, availabilityScore >= threshold, optional snapshot/region/class filters. TOPSIS scoring uses 7-dimensional weighted metrics (CPU usage, memory usage, disk usage, CPU/memory/disk allocation ratios, started sandbox count) with penalty multipliers for overloaded metrics using exponential decay.
- **SandboxManager** (`managers/sandbox.manager.ts`): Listens for `SandboxCreatedEvent` via `@OnAsyncEvent`. Calls `syncInstanceState()` which acquires per-sandbox lock (`sandbox:{id}:state-change`, 30s TTL), routes to `SandboxStartAction.run()`. Timeout: aborts if running > 10 seconds.
- **SandboxStartAction** (`managers/sandbox-actions/sandbox-start.action.ts`): Handles multi-phase startup — PULLING_SNAPSHOT (selects runner, creates SnapshotRunner record, waits for snapshot pull), BUILDING_SNAPSHOT (polls with 60-min timeout), UNKNOWN→CREATING→STARTED transitions. Calls `RunnerAdapter.createSandbox()` through adapter factory (V0=direct HTTP, V2=job queue).
- **RunnerAdapter**: V0 (`runnerAdapter.v0.ts`) makes direct HTTP POST with Axios (3 retries, exponential backoff, 1-hour timeout). V2 (`runnerAdapter.v2.ts`) creates Job record in database (type=CREATE_SANDBOX, status=PENDING) for runner to poll.

**Runner Layer** (`apps/runner/`)
- **V2 Job Processing**: Poller (`pkg/runner/v2/poller/poller.go`) long-polls API for pending jobs. Executor (`pkg/runner/v2/executor/sandbox.go`) dispatches to `createSandbox()` handler, parses `CreateSandboxDTO` from job payload, calls `docker.Create()`, tracks metrics, reports status back to API.
- **Docker Create** (`pkg/docker/create.go`): Checks if sandbox already exists (idempotency). Pulls image from registry via `PullImage()`. Validates image architecture is x86_64. Calls `getVolumesMountPathBinds()` for S3/FUSE volume mounting via mount-s3. Builds container config: hostname=sandboxId, privileged=true, CPU quota (units of 100000), memory limit (bytes), storage quota (xfs), env vars (DAYTONA_SANDBOX_ID, SNAPSHOT, OS_USER + custom). Creates container via Docker API `ContainerCreate()`. Immediately calls `Start()`.
- **Docker Start** (`pkg/docker/start.go`): Sets state to "Starting". Calls `ContainerStart()`. Waits for container running state (10ms polling, configurable timeout). Extracts container IP. Starts daemon process inside container via `startDaytonaDaemon()` if entrypoint is not already daemon. Waits for daemon HTTP server to be accessible (`waitForDaemonRunning`, configurable timeout). Sets network limiter if metadata flag set. Sets state to "Started".
- **Volume Mounting** (`pkg/docker/volumes_mountpaths.go`): For each volume, acquires per-volume mutex, checks if already FUSE-mounted via `isDirectoryMounted()`, creates mount directory (0755), runs `mount-s3` command with `--allow-other --allow-delete --allow-overwrite --prefix`, waits up to 5 seconds (50 polls × 100ms) for FUSE mount readiness via `waitForMountReady()`.

**Daemon Layer** (`apps/daemon/`)
- **Startup** (`cmd/daemon/main.go`): Loads config from environment. Sets working directory. Starts three concurrent services: Toolbox HTTP server (port 2280), Terminal server (port 22222), SSH server (port 22220). Executes entrypoint command in background goroutine. Graceful shutdown: SIGTERM with configurable grace period, then SIGKILL.
- **Toolbox Server** (`pkg/toolbox/toolbox.go`): Gin HTTP framework with Swagger UI. Registers all route groups: `/files`, `/process`, `/git`, `/lsp`, `/computeruse`, `/port`, `/proxy`. Middleware: error recovery, request logging, error formatting.

---

## 2. Sandbox Start/Stop Flow

### Overview
Transitions sandbox between started and stopped states.

### Start Flow Diagram

```
SDK: sandbox.start()
    │
    ▼
API: POST /sandbox/{id}/start
    │
    ├─► SandboxController.startSandbox()
    │       │
    │       ▼
    │   SandboxService.start()
    │       │
    │       ├─► Validate state is STOPPED/ARCHIVED
    │       ├─► Validate organization quotas
    │       ├─► Set desiredState = STARTED
    │       └─► Emit SandboxEvents.STARTED
    │
    ▼
SandboxManager (event handler)
    │
    ├─► syncInstanceState()
    │       │
    │       ▼
    │   SandboxStartAction.run()
    │       │
    │       └─► RunnerAdapter.startSandbox()
    │
    ▼
Runner: Job Execution
    │
    └─► DockerClient.Start()
            │
            ├─► ContainerStart()
            ├─► waitForContainerRunning()
            ├─► waitForDaemonRunning()
            └─► Set state = STARTED
```

### Stop Flow Diagram

```
SDK: sandbox.stop()
    │
    ▼
API: POST /sandbox/{id}/stop
    │
    ├─► SandboxController.stopSandbox()
    │       │
    │       ▼
    │   SandboxService.stop()
    │       │
    │       ├─► Set desiredState = STOPPED
    │       └─► Emit SandboxEvents.STOPPED
    │
    ▼
SandboxManager (event handler)
    │
    ├─► syncInstanceState()
    │       │
    │       ▼
    │   SandboxStopAction.run()
    │       │
    │       └─► RunnerAdapter.stopSandbox()
    │
    ▼
Runner: Job Execution
    │
    └─► DockerClient.Stop()
            │
            ├─► stopContainerWithRetry() (SIGKILL)
            ├─► ContainerWait(NotRunning)
            └─► Set state = STOPPED
```

### Code Path

| Step | Component | File | Function |
|------|-----------|------|----------|
| 1 | API | `apps/api/src/sandbox/controllers/sandbox.controller.ts` | `startSandbox()` / `stopSandbox()` |
| 2 | API | `apps/api/src/sandbox/services/sandbox.service.ts` | `start()` / `stop()` |
| 3 | API | `apps/api/src/sandbox/managers/sandbox.manager.ts` | `syncInstanceState()` |
| 4 | API | `apps/api/src/sandbox/managers/sandbox-actions/` | `SandboxStartAction` / `SandboxStopAction` |
| 5 | Runner | `apps/runner/pkg/docker/start.go` | `Start()` |
| 6 | Runner | `apps/runner/pkg/docker/stop.go` | `Stop()` |

### Auto-Stop Mechanism

**Cron Job:** `autostopCheck()` runs every 10 seconds

```typescript
// Finds STARTED sandboxes where:
// - lastActivityAt < NOW() - autoStopInterval
// - autoStopInterval != 0
// Sets desiredState = STOPPED
```

### Layer Responsibilities

**SDK Layer** (`libs/sdk-python/src/daytona/_sync/sandbox.py`)
- **start():** Calls API to start sandbox. Polls state every **100ms** until state == `started` or error states (`error`, `build_failed`). Default timeout: 60 seconds via `@with_timeout`. Treats unexpected terminal states as errors.
- **stop():** Calls API to stop sandbox with timeout parameter. Polls state every **100ms** until state in `[stopped, destroyed]`. Treats `destroyed` as valid final state for ephemeral sandboxes. Handles `ValidationError` gracefully during polling (continues waiting). Default timeout: 60 seconds.
- **Error wrapping:** Same `@intercept_errors` and `@with_timeout` decorators as create.

**API Layer** (`apps/api/src/sandbox/`)
- **Controller** (`controllers/sandbox.controller.ts`): `startSandbox()` calls service then waits up to 30s for STARTED state via Redis event subscription. `stopSandbox()` calls service and returns immediately.
- **Service** (`services/sandbox.service.ts`): `start()` validates current state transition (STOPPED/ARCHIVED/ERROR → STARTED). For ARCHIVED: triggers restore from backup. For STOPPED: sets desiredState=STARTED, pending=true. Emits `SandboxStartedEvent`. `stop()` validates only STARTED → STOPPED transition. Sets desiredState=STOPPED, pending=true. Emits `SandboxStoppedEvent`.
- **SandboxManager** (`managers/sandbox.manager.ts`): Event-driven via `@OnAsyncEvent`. Routes to `SandboxStartAction` or `SandboxStopAction`. Also runs `autostopCheck()` cron every **10 seconds** — acquires Redis lock `auto-stop-check-worker-selected` (60s TTL), finds STARTED sandboxes where `lastActivityAt < NOW() - autoStopInterval`, sets desiredState=STOPPED (or DESTROYED if autoDeleteInterval==0). Processes up to 100 per batch.
- **SandboxStartAction**: Handles STOPPED state by selecting runner, calling `runnerAdapter.startSandbox()`. For ARCHIVED: first restores from backup. Returns SYNC_AGAIN for continued polling.
- **SandboxStopAction**: For STARTED state — gets runner, validates READY, calls `runnerAdapter.stopSandbox()`, updates to STOPPING, returns SYNC_AGAIN. For STOPPING — polls runner for sandbox info, transitions to STOPPED on completion.

**Runner Layer** (`apps/runner/`)
- **Start** (`pkg/docker/start.go`): Sets state to "Starting". Cancels any in-progress backup. Inspects container — if already running, skips to daemon wait. Otherwise calls `ContainerStart()`, waits for running state (10ms polling), extracts container IP, starts daemon if needed, waits for daemon accessibility. Sets network limiter. Sets state to "Started".
- **Stop** (`pkg/docker/stop.go`): Sets state to "Stopping". Cancels in-progress backup. Calls `stopContainerWithRetry()` — retries up to `DEFAULT_MAX_RETRIES` with exponential backoff, sends SIGKILL with 2-second timeout, falls back to `ContainerKill()`. Waits for container to reach "NotRunning" via `ContainerWait()`. Sets state to "Stopped".

---

## 3. Command Execution Flow

### Overview
Executes shell commands inside a running sandbox and returns output.

### Flow Diagram

```
SDK: sandbox.process.exec(command)
    │
    ▼
ToolboxApiClient (lazy URL loading)
    │
    ├─► Get toolbox proxy URL from API
    │       GET /sandbox/{id}/toolbox-proxy-url
    │
    ▼
HTTP POST to Proxy
    │
    ├─► Proxy: GetProxyTarget()
    │       │
    │       ├─► Parse sandbox ID from host/path
    │       ├─► Authenticate request
    │       ├─► Get runner info from cache/API
    │       └─► Build target: {runner}/sandboxes/{id}/toolbox/proxy/2280
    │
    ▼
Daemon (Toolbox): POST /process/execute
    │
    ├─► ExecuteCommand()
    │       │
    │       ├─► Parse command
    │       ├─► exec.Command(command)
    │       ├─► Set timeout (default 360s)
    │       ├─► cmd.CombinedOutput()
    │       └─► Return {exitCode, result}
    │
    ▼
Response flows back through Proxy → SDK
```

### Code Path

| Step | Component | File | Function |
|------|-----------|------|----------|
| 1 | SDK | `libs/sdk-python/src/daytona/_sync/process.py` | `exec()` |
| 2 | SDK | `libs/sdk-python/src/daytona/internal/toolbox_api_client_proxy.py` | `call_api()` |
| 3 | Proxy | `apps/proxy/pkg/proxy/get_sandbox_target.go` | `GetProxyTarget()` |
| 4 | Proxy | `apps/proxy/pkg/proxy/auth.go` | `Authenticate()` |
| 5 | Daemon | `apps/daemon/pkg/toolbox/process/execute.go` | `ExecuteCommand()` |

### Request/Response Format

**Request:**
```json
{
  "command": "echo 'hello'",
  "cwd": "/workspace",
  "timeout": 60
}
```

**Response:**
```json
{
  "exitCode": 0,
  "result": "hello\n"
}
```

### Session-Based Execution

For long-running commands, use sessions:

```
SDK: sandbox.process.create_session(id)
     sandbox.process.execute_session_command(id, command)
     sandbox.process.get_session_command_logs(id, cmdId)
```

### Layer Responsibilities

**SDK Layer** (`libs/sdk-python/src/daytona/_sync/process.py`)
- **exec():** Encodes command in **base64** to prevent shell injection: wraps as `echo '<base64>' | base64 -d | sh`. Encodes environment variables similarly: `export KEY=$(echo '<base64>' | base64 -d)`. Final wrapper: `sh -c "<command>"`. Parses stdout lines for artifact markers (`dtn_artifact_k39fd2:`) to extract JSON artifacts (charts). Returns `ExecutionArtifacts` with stdout and list of Chart objects.
- **Toolbox URL resolution:** Uses `ToolboxApiClientProxyLazyBaseUrl` which defers URL loading until first API call. Calls `GET /sandbox/{id}/toolbox-proxy-url` once, caches result with thread-safe `Future`-based per-region caching (double-check locking pattern). Prepends base URL to all subsequent relative paths.
- **Session management:** `create_session()` creates session via API. `execute_session_command()` calls API then demultiplexes output using binary prefix markers (0x01=stdout, 0x02=stderr) via `demux_log()` utility. Returns `SessionExecuteResponse` with separated stdout/stderr and exit code. `get_session_command_logs_async()` converts HTTP URL to WebSocket URL via regex, streams logs with demultiplexing.

**Proxy Layer** (`apps/proxy/pkg/proxy/`)
- **Target resolution** (`get_sandbox_target.go`): `GetProxyTarget()` parses host header (format: `{port}-{sandboxId}.{proxyDomain}`) or toolbox subpath (`/toolbox/{sandboxId}/{path}`). For non-public sandboxes or toolbox/terminal ports, requires authentication. Resolves sandbox to runner via `getSandboxRunnerInfo()` (cached **2 minutes** in Redis or in-memory). Builds target URL: `{runnerApiUrl}/sandboxes/{sandboxId}/toolbox/proxy/{port}{path}`. Adds headers: `X-Daytona-Authorization: Bearer {apiKey}`, `X-Forwarded-Host: {originalHost}`.
- **Authentication chain** (`auth.go`): Tries 6 methods in order: (1) Bearer token from `Authorization` header, (2) `X-Daytona-Preview-Token` header, (3) `DAYTONA_SANDBOX_AUTH_KEY` query param, (4) encrypted secure cookie `daytona-sandbox-auth-{sandboxId}`, (5) signed preview URL token via API lookup, (6) OIDC redirect with PKCE. Auth results cached **2 minutes**. Removes auth headers/params before forwarding.
- **Reverse proxy:** Uses `httputil.ReverseProxy.ServeHTTP()` — transparently streams request/response bodies (multipart, WebSocket upgrades, chunked encoding). Transport: HTTP/1.1 with 30s KeepAlive, 100 max idle connections.
- **Activity tracking** (`get_sandbox_target.go`): Updates sandbox `lastActivityAt` via API. Cached to prevent spam: **50-second poll interval** with 45-second cache expiry. Starts polling goroutine for active connections, stops when connection closes.

**Daemon Layer** (`apps/daemon/pkg/toolbox/process/`)
- **ExecuteCommand** (`execute.go`): Parses command with custom quote-aware parser (handles `"` and `'` in commands). Splits into `[]string` for `exec.Command()`. Sets working directory via `cmd.Dir`. Captures combined stdout+stderr via `cmd.CombinedOutput()`. Default timeout: **360 seconds** via `time.AfterFunc()` — kills process group on expiry. Returns `{exitCode, result}` — HTTP 408 on timeout, exit code from `exec.ExitError` on failure, -1 if process fails to start.
- **Session execution** (`session/`): Creates persistent shell process (`/bin/sh`) with stdin pipe. Commands executed via FIFO-based shell scripts that prefix stdout/stderr with binary markers (0x01/0x02) and write exit code to separate file. Synchronous mode polls for exit code file at 50ms intervals. Async mode returns command ID immediately (HTTP 202). Session termination: SIGTERM to process tree, 5-second grace, then SIGKILL. Uses `gopsutil/process` for recursive child process enumeration.

---

## 4. File Upload/Download Flow

### Overview
Upload and download files to/from sandbox filesystem.

### Upload Flow Diagram

```
SDK: sandbox.fs.upload_file(source, destination)
    │
    ▼
HTTP POST (Multipart) to Proxy
    │
    ├─► POST /files/bulk-upload
    │       Content-Type: multipart/form-data
    │       Fields:
    │         files[0].path = destination
    │         files[0].file = (binary content)
    │
    ▼
Daemon: UploadFiles()
    │
    ├─► Parse multipart reader
    ├─► For each file:
    │       │
    │       ├─► os.MkdirAll(parent, 0755)
    │       ├─► os.Create(destination)
    │       └─► io.Copy(file, part)
    │
    ▼
Return HTTP 200
```

### Download Flow Diagram

```
SDK: sandbox.fs.download_file(remote_path)
    │
    ▼
HTTP POST to Proxy
    │
    ├─► POST /files/bulk-download
    │       Body: {"paths": ["/path/to/file"]}
    │
    ▼
Daemon: DownloadFiles()
    │
    ├─► Set boundary: "DAYTONA-FILE-BOUNDARY"
    ├─► For each path:
    │       │
    │       ├─► If file exists:
    │       │       └─► Write file part (multipart)
    │       └─► If not exists:
    │               └─► Write error part
    │
    ▼
Response: multipart/form-data
    │
    ▼
SDK: Parse multipart response
    │
    ├─► "file" parts → Write to disk or BytesIO
    └─► "error" parts → Collect errors
```

### Code Path

| Step | Component | File | Function |
|------|-----------|------|----------|
| 1 | SDK | `libs/sdk-python/src/daytona/_sync/filesystem.py` | `upload_file()` / `download_file()` |
| 2 | Daemon | `apps/daemon/pkg/toolbox/fs/upload_files.go` | `UploadFiles()` |
| 3 | Daemon | `apps/daemon/pkg/toolbox/fs/download_files.go` | `DownloadFiles()` |

### Streaming Details

- **Upload chunks:** 64 KiB (HTTPX default)
- **Download chunks:** 64 KiB
- **Multipart boundary:** `DAYTONA-FILE-BOUNDARY`

### Layer Responsibilities

**SDK Layer** (`libs/sdk-python/src/daytona/_sync/filesystem.py`)
- **upload_file()/upload_files():** Accepts bytes/bytearray (wraps in `io.BytesIO`) or file paths (opens with `open(path, 'rb')`). Uses `ExitStack` for file handle lifecycle management. Constructs multipart form data with keys `files[i].path` (destination) and `files[i].file` (binary content). Sends via httpx streaming client. Default timeout: **30 minutes** (1800 seconds).
- **download_file()/download_files():** Sends POST with JSON body `{paths: [...]}`. Parses multipart/form-data response using `PushMultipartParser` with **64 KiB** chunk streaming. Detects field name to distinguish `file` parts from `error` parts. If destination path provided: creates parent directories via `os.makedirs()`, writes to disk. If no destination: returns bytes via `io.BytesIO`. Individual file errors stored in response object (doesn't throw).
- **Other operations:** `create_folder()`, `delete_file()`, `list_files()`, `get_file_info()`, `move_files()`, `find_files()`, `search_files()`, `replace_in_files()`, `set_file_permissions()` — all simple API passthroughs with `@intercept_errors` wrapping. Paths resolved against sandbox working directory.

**Proxy Layer** (`apps/proxy/pkg/proxy/`)
- **Transparent streaming:** `httputil.ReverseProxy.ServeHTTP()` transparently streams multipart request bodies (uploads) and multipart response bodies (downloads) without buffering entire content. No special multipart handling — uses default HTTP client behavior.
- **Authentication & routing:** Same as Command Execution Flow — parses host, authenticates, resolves runner, builds target URL, adds auth headers.

**Daemon Layer** (`apps/daemon/pkg/toolbox/fs/`)
- **UploadFiles** (`upload_files.go`): Parses multipart reader from request. Extracts file indices from form field names (`files[N].path`, `files[N].file`). For each file: creates parent directories with `os.MkdirAll(0755)`, creates destination file with `os.Create()`, copies content with `io.Copy()`. Returns aggregated errors list.
- **DownloadFiles** (`download_files.go`): Sets response boundary to `DAYTONA-FILE-BOUNDARY`. Uses context-aware writer (`ctxWriter`) that wraps part writer and cancels on request context Done. For each requested path: checks existence — if exists, writes as `file` part using `writeFilePart()` with filename header; if not exists, writes `error` part using `writeErrorPart()`.
- **CreateFolder** (`create_folder.go`): `os.MkdirAll()` with 0755 permissions.
- **DeleteFile** (`delete_file.go`): `os.RemoveAll()` (recursive).
- **GetFileInfo** (`get_file_info.go`): Returns name, size, mode, modified time, is_directory, permissions.
- **ListFiles** (`list_files.go`): `os.ReadDir()` for directory listing, collects `FileInfo` structures.
- **MoveFile** (`move_file.go`): `os.Rename()`.
- **SearchFiles** (`search_files.go`): `filepath.Walk()` + `filepath.Match()` for glob patterns.
- **FindInFiles** (`find_in_files.go`): `filepath.Walk()` traversal. Binary file detection: reads first 512 bytes, skips if contains null byte. Line-by-line `bufio.Scanner()` search. Returns matches with file, line number, content.
- **ReplaceInFiles** (`replace_in_files.go`): Same traversal and binary detection as FindInFiles. `strings.ReplaceAll()` for text replacement. Writes updated content back to file.
- **SetFilePermissions** (`set_file_permissions.go`): `os.Chmod()` with octal mode.

---

## 5. SSH Access Flow

### Overview
Establishes SSH tunnel access to sandbox via SSH Gateway.

### Flow Diagram

```
SDK: sandbox.create_ssh_access(expires_in_minutes)
    │
    ▼
API: POST /sandbox/{id}/ssh-access
    │
    ├─► SandboxService.createSshAccess()
    │       │
    │       ├─► Generate token: customNanoid(32)
    │       ├─► Set expiration
    │       └─► Save to database
    │
    ▼
Return: {token, sshCommand}
    │
    ▼
SSH Client connects to SSH Gateway (port 2222)
    │
    ├─► Username = token
    │
    ▼
SSH Gateway: handleConnection()
    │
    ├─► Extract token from username
    ├─► API: ValidateSshAccess(token)
    │       └─► Check token exists and not expired
    ├─► API: GetRunnerBySandboxId(sandboxId)
    ├─► Validate sandbox is STARTED
    └─► Forward to Runner SSH (port 2220)
            │
            ▼
Runner SSH Gateway: handleChannel()
    │
    ├─► Verify public key
    └─► Forward to Daemon SSH (port 22220)
            │
            ▼
Daemon SSH Server
    │
    ├─► Password auth: "sandbox-ssh"
    ├─► Spawn shell with PTY
    └─► Interactive session
```

### Code Path

| Step | Component | File | Function |
|------|-----------|------|----------|
| 1 | SDK | `libs/sdk-python/src/daytona/_sync/sandbox.py` | `create_ssh_access()` |
| 2 | API | `apps/api/src/sandbox/services/sandbox.service.ts` | `createSshAccess()` |
| 3 | SSH Gateway | `apps/ssh-gateway/main.go` | `handleConnection()` |
| 4 | Runner | `apps/runner/pkg/sshgateway/service.go` | `handleChannel()` |
| 5 | Daemon | `apps/daemon/pkg/ssh/server.go` | `Start()` |

### Port Mapping

| Service | Port | Purpose |
|---------|------|---------|
| SSH Gateway | 2222 | External entry point |
| Runner SSH | 2220 | Internal forwarding |
| Daemon SSH | 22220 | Sandbox SSH server |

### Layer Responsibilities

**SDK Layer** (`libs/sdk-python/src/daytona/_sync/sandbox.py`)
- **create_ssh_access():** Simple passthrough to API with optional `expires_in_minutes` parameter (default 60). Returns `SshAccessDto` containing token, expiresAt, and sshGatewayUrl. No complex transformation or polling.

**CLI Layer** (`apps/cli/cmd/sandbox/ssh.go`, `apps/cli/cmd/common/ssh.go`)
- **ParseSSHCommand():** Parses SSH command from `SshAccessDto.sshCommand`. Extracts user, host, port, and any extra SSH options.
- **ExecuteSSH():** Spawns native `ssh` process via `os/exec` with parsed arguments. Connects stdin/stdout/stderr to terminal. Waits for process to exit.

**API Layer** (`apps/api/src/sandbox/`)
- **createSshAccess** (`services/sandbox.service.ts`): Looks up sandbox. Revokes any existing SSH access. Generates 32-character token via `customNanoid(32)` (alphanumeric, no `-` or `_`). Sets expiry: `new Date(Date.now() + expiresInMinutes * 60 * 1000)`. Saves `SshAccess` entity with sandboxId, token, expiresAt. Looks up region for `sshGatewayUrl`. Returns `SshAccessDto`.
- **validateSshAccess** (`services/sandbox.service.ts`): Finds `SshAccess` by token with sandbox relation. Validates `expiresAt > now`. Returns `SshAccessValidationDto` with boolean `valid` and `sandboxId`. Called by SSH Gateway on every connection.
- **revokeSshAccess**: Deletes SshAccess records by sandboxId (optionally filtered by specific token).

**SSH Gateway Layer** (`apps/ssh-gateway/main.go`)
- **Start():** Listens on TCP port 2222 (default). Creates SSH ServerConfig with `NoClientAuth=true` (custom auth in handler). Adds host key. Accepts connections in infinite loop, spawns goroutine per connection.
- **handleConnection():** (1) Performs SSH handshake. (2) Extracts token from SSH username field (`serverConn.User()`). (3) Validates token via API call `SandboxAPI.ValidateSshAccess(token)` — returns `{valid, sandboxId}`. (4) Looks up runner domain via `RunnersAPI.GetRunnerBySandboxId(sandboxId)`. (5) Checks sandbox state is STARTED via `SandboxAPI.GetSandbox()`. (6) Spawns global requests handler (discards). (7) Enters channel loop.
- **handleChannel():** Accepts client SSH channel. Connects to runner via `connectToRunner()`. Opens matching channel on runner connection (preserves channel type and extra data). Starts **4 goroutines**: client→runner request forwarding, runner→client request forwarding, client→runner data streaming (`io.Copy`), runner→client data streaming. All SSH channel types (session, direct-tcpip, subsystem) forwarded transparently including PTY allocation, window changes, exec, shell.
- **connectToRunner():** Dials runner SSH at `{runnerDomain}:2220` using public key authentication with gateway's private key. Username = sandboxId. Timeout: 30 seconds. `InsecureIgnoreHostKey` (internal network).
- **Keep-alive:** Background goroutine calls `SandboxAPI.UpdateLastActivity(sandboxId)` immediately, then every **45 seconds** during active SSH connection. Prevents auto-stop while user is connected.

**Runner SSH Gateway** (`apps/runner/pkg/sshgateway/service.go`)
- **Start():** Listens on runner SSH port (2220). SSH server config uses public key authentication (verifies gateway's public key). Accepts connections, handles channels.
- **handleChannel():** Accepts runner-side channel. Connects to sandbox container via `connectToSandbox()`. Bidirectional request and data forwarding (same pattern as SSH Gateway).
- **connectToSandbox():** Inspects Docker container to get sandbox IP. Dials SSH at `{containerIP}:22220`. Uses password authentication: `"sandbox-ssh"`. Username: `"daytona"`. Timeout: 30 seconds.

**Daemon SSH Server** (`apps/daemon/pkg/ssh/server.go`)
- **Start():** Listens on port 22220 (configurable via `config.SSH_PORT`). Uses `gliderlabs/ssh` library. Password auth: accepts `"sandbox-ssh"`. Public key: accepts all keys. SSH agent forwarding supported.
- **Session handling:** PTY mode: spawns TTY via `common.SpawnTTY()`, handles terminal resize via window-change channel. Non-PTY mode: executes `/bin/sh -c <command>`, pipes stdin/stdout/stderr. Maps SSH signals (SIGTERM, SIGKILL, etc.) to OS signals.
- **Port forwarding:** Supports local (`-L`), reverse (`-R`), and direct TCP/IP forwarding. SFTP subsystem via `pkg/sftp` library.

---

## 6. Preview URL Flow

### Overview
Generates preview URLs for accessing services running inside sandbox.

### Flow Diagram

```
SDK: sandbox.get_preview_link(port)
    │
    ▼
API: GET /sandbox/{id}/ports/{port}/preview-url
    │
    ├─► SandboxService.getPortPreviewUrl()
    │       │
    │       └─► Build URL: {protocol}://{port}-{sandboxId}.{proxyDomain}
    │
    ▼
Return: {sandboxId, url, token}
```

### Signed Preview URL Flow

```
SDK: sandbox.get_signed_preview_url(port, expires)
    │
    ▼
API: GET /sandbox/{id}/ports/{port}/signed-preview-url
    │
    ├─► Generate random 16-char token
    ├─► Redis: SET sandbox:signed-preview-url-token:{port}:{token} = sandboxId
    │       TTL: expiresInSeconds
    └─► Build URL: {protocol}://{port}-{token}.{proxyDomain}
```

### Browser Access Flow

```
Browser: https://3000-abc123.proxy.daytona.io
    │
    ▼
Proxy: Router catches request
    │
    ├─► parseHost() → port=3000, token=abc123
    ├─► Check if sandbox is public
    │       │
    │       ▼ (if private)
    │   Authenticate()
    │       │
    │       ├─► Try Bearer token
    │       ├─► Try X-Daytona-Preview-Token header
    │       ├─► Try query param
    │       ├─► Try secure cookie
    │       ├─► Try signed token resolution
    │       │       └─► API: GetSandboxIdFromSignedPreviewUrlToken()
    │       └─► Redirect to OIDC (if all fail)
    │
    ├─► getSandboxRunnerInfo() → get runner URL
    │
    ├─► Build target: {runner}/sandboxes/{id}/toolbox/proxy/{port}
    │
    └─► Reverse proxy to runner
            │
            ▼
        Service in sandbox responds
```

### Code Path

| Step | Component | File | Function |
|------|-----------|------|----------|
| 1 | SDK | `libs/sdk-typescript/src/Sandbox.ts` | `getPreviewLink()` |
| 2 | API | `apps/api/src/sandbox/services/sandbox.service.ts` | `getPortPreviewUrl()` |
| 3 | Proxy | `apps/proxy/pkg/proxy/proxy.go` | Router handler |
| 4 | Proxy | `apps/proxy/pkg/proxy/get_sandbox_target.go` | `GetProxyTarget()` |
| 5 | Proxy | `apps/proxy/pkg/proxy/auth.go` | `Authenticate()` |

### URL Format

- **Unsigned:** `https://{port}-{sandboxId}.{proxyDomain}`
- **Signed:** `https://{port}-{signedToken}.{proxyDomain}`

### Layer Responsibilities

**SDK Layer** (`libs/sdk-typescript/src/Sandbox.ts`, `libs/sdk-python/src/daytona/_sync/sandbox.py`)
- **getPreviewLink() / get_preview_link():** Simple API passthrough. Calls `GET /sandbox/{id}/ports/{port}/preview-url`. Returns `PortPreviewUrlDto` containing sandboxId, url, and token (sandbox's authToken for embedding in requests).
- **getSignedPreviewUrl() / get_signed_preview_url():** Calls `GET /sandbox/{id}/ports/{port}/signed-preview-url` with `expiresInSeconds` parameter. Returns `SignedPortPreviewUrlDto` with time-limited token embedded in URL.

**API Layer** (`apps/api/src/sandbox/services/sandbox.service.ts`)
- **getPortPreviewUrl():** Validates port 1-65535. Looks up sandbox by ID/name (1-second TypeORM cache). Gets `proxy.domain` and `proxy.protocol` from config. Constructs URL: `{protocol}://{port}-{sandboxId}.{proxyDomain}`. Region override: if sandbox's region has `proxyUrl` configured, replaces domain portion. Returns `{sandboxId, url, token}` where token = sandbox.authToken.
- **getSignedPortPreviewUrl():** Validates port 1-65535, expiry 1s–24h. Generates 16-character random token via `nanoid` (alphanumeric, no `-` or `_`). Stores in Redis: `SETEX sandbox:signed-preview-url-token:{port}:{token} {TTL} {sandboxId}`. Constructs URL with signed token: `{protocol}://{port}-{token}.{proxyDomain}`. Region override same as unsigned.
- **expireSignedPreviewUrlToken():** Deletes Redis key to immediately revoke signed URL access.

**Proxy Layer** (`apps/proxy/pkg/proxy/`)
- **Host parsing** (`proxy.go`): `parseHost()` extracts port and sandboxId/signedToken from subdomain format `{port}-{id}.{proxyDomain}`.
- **Public access check** (`get_sandbox_target.go`): `getSandboxPublic()` checks if sandbox allows unauthenticated access. Cached for **1 hour**.
- **Authentication** (`auth.go`): For private sandboxes, runs full 6-method auth chain: Bearer → Header → Query → Cookie → Signed token → OIDC redirect. For signed tokens: calls API `GetSandboxIdFromSignedPreviewUrlToken(token, port)` to resolve token→sandboxId. On success, sets encrypted secure cookie `daytona-sandbox-auth-{sandboxId}` (HttpOnly, Secure, SameSite=None, 3600s TTL) so subsequent requests skip auth.
- **OIDC callback** (`auth_callback.go`): Full OAuth2 PKCE flow — decodes state from base64 JSON `{returnTo, sandboxId, state}`, retrieves code_verifier from encrypted cookie, exchanges auth code for token, validates sandbox access, sets auth cookie, redirects to original URL.
- **Reverse proxy** (`proxy.go`): Builds target: `{runnerApiUrl}/sandboxes/{sandboxId}/toolbox/proxy/{port}{path}`. Proxies request with auth headers. Service inside sandbox container responds directly to browser.

---

## 7. Snapshot Creation Flow

### Overview
Creates a container image snapshot from a Dockerfile or existing image.

### Flow Diagram (Dockerfile Build)

```
CLI: daytona snapshot create --dockerfile Dockerfile
    │
    ▼
Upload build context to MinIO
    │
    ├─► Parse Dockerfile for COPY/ADD commands
    ├─► For each context file:
    │       └─► MinIO: Upload with SHA256 hash
    │
    ▼
API: POST /snapshots
    │
    ├─► SnapshotService.createFromBuildInfo()
    │       │
    │       ├─► Generate snapshotRef from Dockerfile hash
    │       ├─► Create BuildInfo entity
    │       ├─► Create Snapshot entity (state=PENDING)
    │       └─► Emit SnapshotCreatedEvent
    │
    ▼
SnapshotManager (async)
    │
    ├─► handleSnapshotStatePending()
    │       │
    │       ├─► Select runner
    │       ├─► Set state = BUILDING
    │       └─► processBuildOnRunner()
    │               │
    │               └─► RunnerAdapter.buildSnapshot()
    │
    ▼
Runner: Job Execution
    │
    └─► DockerClient.BuildImage()
            │
            ├─► Create tar archive with:
            │       ├─► Dockerfile
            │       └─► Context files from MinIO
            ├─► Docker: ImageBuild()
            ├─► Stream build logs to file
            ├─► Docker: ImagePush() to registry
            └─► Return image info
    │
    ▼
SnapshotManager: handleCheckInitialRunnerSnapshot()
    │
    ├─► Verify image exists on runner
    ├─► Get image digest
    ├─► Update state = ACTIVE
    └─► Propagate to other runners (async)
```

### Code Path

| Step | Component | File | Function |
|------|-----------|------|----------|
| 1 | CLI | `apps/cli/cmd/snapshot/create.go` | `CreateCmd.RunE` |
| 2 | CLI | `apps/cli/cmd/common/build.go` | `getContextHashes()` |
| 3 | API | `apps/api/src/sandbox/services/snapshot.service.ts` | `createFromBuildInfo()` |
| 4 | API | `apps/api/src/sandbox/managers/snapshot.manager.ts` | `handleSnapshotStatePending()` |
| 5 | Runner | `apps/runner/pkg/docker/image_build.go` | `BuildImage()` |
| 6 | Runner | `apps/runner/pkg/docker/image_push.go` | `PushImage()` |

### State Transitions

```
PENDING → BUILDING → ACTIVE
    │         │
    ▼         ▼
  ERROR   BUILD_FAILED
```

### Layer Responsibilities

**CLI Layer** (`apps/cli/cmd/snapshot/create.go`)
- **CreateCmd.RunE:** Parses Dockerfile path. Scans Dockerfile for COPY/ADD commands to identify context files. Computes SHA256 hash for each context file. Uploads context files to MinIO object storage via `getContextHashes()`. Constructs `CreateSnapshotDto` with buildInfo and context hashes. Calls API.

**SDK Layer** (`libs/sdk-python/src/daytona/_sync/snapshot.py`)
- **create():** If image is declarative (not a string): calls `process_image_context()` which fetches push credentials from object storage API and uploads each context file to S3. Builds `CreateSnapshot` DTO with name, image/image_name, build_info (context_hashes, Dockerfile), resources (CPU, GPU, memory, disk), region_id. Polls state every **1 second** until terminal state: `[ACTIVE, ERROR, BUILD_FAILED]`. If `on_logs` callback provided: spawns `threading.Thread` for concurrent build log streaming via `process_streaming_response()`. Raises `DaytonaError` if final state is ERROR/BUILD_FAILED with `error_reason`.

**API Layer** (`apps/api/src/sandbox/`)
- **SnapshotService** (`services/snapshot.service.ts`): `createFromPull()` validates image name (regex, requires tag not `:latest`, or `@sha256:hash`), validates snapshot name, checks organization not suspended, checks snapshot count quota, creates Snapshot entity with state=PENDING, creates SnapshotRegion mapping, emits `SnapshotCreatedEvent`. `createFromBuildInfo()` additionally creates BuildInfo entity with Dockerfile, context hashes, snapshotRef hash.
- **SnapshotManager** (`managers/snapshot.manager.ts`): `handleSnapshotStatePending()` selects runner, sets state=BUILDING, calls `processBuildOnRunner()` → `RunnerAdapter.buildSnapshot()`. `syncRunnerSnapshotStates()` cron (every 10s) polls runner for build progress. `handleCheckInitialRunnerSnapshot()` verifies image exists on runner, gets digest, updates state=ACTIVE, propagates to other runners asynchronously. `syncRunnerSnapshots()` cron (every 5s) propagates snapshots across all runners (100 per batch, pagination via Redis skip pointer).

**Runner Layer** (`apps/runner/`)
- **BuildImage** (`pkg/docker/image_build.go`): Validates image format. Creates tar archive build context from Dockerfile and context files fetched from object storage by hash. Sets up registry authentication for source registries (special Docker Hub handling). Calls Docker `ImageBuild()` with tags, Dockerfile, force_remove=true, pull_parent=true, platform=linux/amd64. Streams build output to log file and optional logWriter via `DisplayJSONMessagesStream`. Returns error if build fails.
- **PushImage** (`pkg/docker/image_push.go`): Calls Docker `ImagePush()` with registry auth from RegistryDTO. Streams JSON response to debug log writer.

**Snapshot Manager Layer** (`apps/snapshot-manager/`)
- **Docker Registry Wrapper:** Wraps `distribution/distribution` v3 as OCI/Docker registry server on port 5000. Storage backends: filesystem (local directory) or S3 (with optional encryption). Optional htpasswd-based basic authentication. Health check at `/healthz`. All Docker registry v2 APIs routed through distribution handlers. Configuration via `SNAPSHOT_MANAGER_*` environment variables. Supports multi-instance deployments via shared HTTP secret.

---

## 8. Code Interpreter Flow

### Overview
Executes Python code with persistent state across multiple executions.

### Flow Diagram

```
SDK: sandbox.code_interpreter.run_code(code, context)
    │
    ▼
WebSocket Connection
    │
    ├─► Upgrade: GET /process/interpreter/execute → WS
    │
    ▼
Send JSON message: {code, contextId, envs, timeout}
    │
    ▼
Daemon: Execute() handler
    │
    ├─► Get or create context
    ├─► Enqueue execution job
    │
    ▼
Context: processQueue()
    │
    ├─► executeCode()
    │       │
    │       ├─► Send command to Python worker (JSON line)
    │       ├─► Wait for completion (poll every 50ms)
    │       └─► Handle timeout (SIGINT → SIGKILL)
    │
    ▼
Python Worker Process (repl_worker.py)
    │
    ├─► Read command from stdin (JSON line)
    ├─► exec(code, persistent_globals)
    ├─► Capture stdout/stderr
    │       └─► Stream as JSON chunks to stdout
    ├─► Catch exceptions
    │       └─► Emit error chunk with traceback
    └─► Emit control chunk ("completed")
    │
    ▼
Daemon: workerReadLoop()
    │
    ├─► Parse JSON chunks
    ├─► handleChunk()
    │       │
    │       ├─► "stdout"/"stderr" → Stream to WebSocket
    │       ├─► "error" → Update command status
    │       └─► "control" → Mark completion
    │
    ▼
WebSocket messages to SDK
    │
    ├─► on_stdout callback
    ├─► on_stderr callback
    └─► on_error callback
    │
    ▼
Return: ExecutionResult{stdout, stderr, error}
```

### Code Path

| Step | Component | File | Function |
|------|-----------|------|----------|
| 1 | SDK | `libs/sdk-python/src/daytona/_sync/code_interpreter.py` | `run_code()` |
| 2 | Daemon | `apps/daemon/pkg/toolbox/process/interpreter/controller.go` | `Execute()` |
| 3 | Daemon | `apps/daemon/pkg/toolbox/process/interpreter/repl_client.go` | `executeCode()` |
| 4 | Worker | `apps/daemon/pkg/toolbox/process/interpreter/repl_worker.py` | `execute_code()` |

### Wire Protocol

**SDK → Daemon (WebSocket text):**
```json
{"code": "print('hello')", "contextId": null, "envs": null, "timeout": 600}
```

**Worker ↔ Daemon (JSON lines over stdin/stdout):**
```json
{"type": "stdout", "text": "hello\n"}
{"type": "stderr", "text": "warning message"}
{"type": "error", "name": "ValueError", "value": "...", "traceback": "..."}
{"type": "control", "text": "completed"}
```

**Daemon → SDK (WebSocket text):**
```json
{"type": "stdout", "text": "hello\n"}
```

### Context Isolation

- Each context has its own Python worker process
- Variables/imports persist within context
- Default context for shared state
- `create_context()` for isolated execution

### Layer Responsibilities

**SDK Layer** (`libs/sdk-python/src/daytona/_sync/code_interpreter.py`)
- **run_code():** Converts HTTP URL to WebSocket URL via regex (`^http → ws`). Connects via `websockets.sync.client.connect()`. Sends JSON request: `{code, contextId, envs, timeout}` (default timeout: **10 minutes**). Processes WebSocket messages in loop until `ConnectionClosedOK`. Aggregates output in `ExecutionResult`. Chunk types: `stdout` → appends to result.stdout + calls `on_stdout` callback, `stderr` → appends to result.stderr + calls `on_stderr` callback, `error` → sets `ExecutionError` with name/value/traceback + calls `on_error` callback. Close code 4008 → raises `DaytonaTimeoutError`. Other close codes → raises `DaytonaError`.
- **create_context():** Creates isolated Python execution context via API with optional working directory. Returns `InterpreterContext` with id and metadata.
- **delete_context():** Deletes context by ID, cleans up associated Python worker process.

**Proxy Layer** (`apps/proxy/pkg/proxy/`)
- **WebSocket upgrade:** Gin router catches `GET /process/interpreter/execute` request. Reverse proxy (`httputil.ReverseProxy`) automatically handles HTTP → WebSocket upgrade transparently. `ConnectionMonitor` wraps hijacked TCP connection to detect close events and stop activity polling goroutines.
- **Transparent streaming:** All WebSocket frames forwarded bidirectionally without inspection.

**Daemon Layer** (`apps/daemon/pkg/toolbox/process/interpreter/`)
- **Controller** (`controller.go`): `Execute()` handler upgrades HTTP to WebSocket. Parses incoming JSON message for `ExecuteRequest{code, contextId, timeout, envs}`. Validates code is required. Gets or creates default context if no contextId specified. Calls `iCtx.enqueueAndExecute()`.
- **Manager** (`manager.go`): Global context registry. `CreateContext()` generates UUID, creates new context. `GetOrCreateDefaultContext()` lazy-initializes default context. Pre-warms default context in background on startup. `DeleteContext()` removes from map and shuts down context's Python worker process.
- **REPL Client** (`repl_client.go`): `enqueueAndExecute()` pushes job to FIFO queue (128 slots). `processQueue()` goroutine dequeues sequentially — guarantees single-threaded execution within a context. `executeCode()` creates `CommandExecution` tracking object with UUID, sends `WorkerCommand{ID, Code, Envs}` as JSON line to Python worker stdin. Polls every **50ms** for completion. Timeout handling: sends SIGINT on timeout, **2-second grace period**, then SIGKILL if not terminated.
- **WebSocket** (`websocket.go`): `attachWebSocket()` connects client to context — creates `wsClient` with send channel (1024 buffer). `clientWriter()` goroutine reads from channel, writes to WebSocket with **10-second write deadline**. `emitOutput()` sends output messages non-blocking — closes connection for slow consumers. Close handshake: 5-second drain timeout, proper RFC 6455 close frame.
- **Worker Read Loop**: `workerReadLoop()` reads worker stdout line-by-line (64KB buffer). `handleChunk()` dispatches: `stdout`/`stderr` → emits to WebSocket client, `error` → sets command status to ERROR, `control` (`completed`/`interrupted`) → marks command status OK/INTERRUPTED.
- **Python Worker** (`repl_worker.py`): Persistent `globals` dict (with Python builtins) across executions. Compiles code to bytecode, executes with `exec(compiled, self.globals)`. Redirects stdout/stderr to custom `_StreamEmitter` (buffers until newline or 1024 bytes, emits as JSON chunks). Cleans traceback to show only user code (filename == `<string>`). Handles `KeyboardInterrupt` → emits control `interrupted`. Environment variable snapshot/restore around each execution. JSON line protocol over stdin/stdout.

---

## 9. Git Operations Flow

### Overview
Git clone, commit, and push operations inside sandbox.

### Clone Flow

```
SDK: sandbox.git.clone(url, path, branch, username, password)
    │
    ▼
HTTP POST to Proxy: /git/clone
    │
    Body: {url, path, branch, username, password}
    │
    ▼
Daemon: CloneRepository()
    │
    ├─► Create git.Service with WorkDir
    ├─► Build BasicAuth (if credentials provided)
    └─► gitService.CloneRepository()
            │
            ▼
        go-git: PlainClone()
            │
            ├─► Clone with options:
            │       ├─► URL
            │       ├─► SingleBranch: true
            │       ├─► Auth: BasicAuth
            │       └─► ReferenceName: branch
            │
            └─► If commit_id specified:
                    └─► Checkout specific commit
```

### Commit Flow

```
SDK: sandbox.git.commit(path, message, author, email)
    │
    ▼
HTTP POST to Proxy: /git/commit
    │
    ▼
Daemon: CommitChanges()
    │
    └─► gitService.Commit()
            │
            ├─► git.PlainOpen(path)
            ├─► repo.Worktree()
            └─► worktree.Commit(message, options)
    │
    ▼
Return: {hash: "abc123..."}
```

### Push Flow

```
SDK: sandbox.git.push(path, username, password)
    │
    ▼
HTTP POST to Proxy: /git/push
    │
    ▼
Daemon: PushChanges()
    │
    └─► gitService.Push()
            │
            ├─► git.PlainOpen(path)
            ├─► repo.Head() → get current branch
            └─► repo.Push(PushOptions{
                    Auth: BasicAuth,
                    RefSpecs: [currentBranch:currentBranch]
                })
```

### Code Path

| Step | Component | File | Function |
|------|-----------|------|----------|
| 1 | SDK | `libs/sdk-python/src/daytona/_sync/git.py` | `clone()` / `commit()` / `push()` |
| 2 | Daemon | `apps/daemon/pkg/toolbox/git/clone_repository.go` | `CloneRepository()` |
| 3 | Daemon | `apps/daemon/pkg/toolbox/git/commit.go` | `CommitChanges()` |
| 4 | Daemon | `apps/daemon/pkg/toolbox/git/push.go` | `PushChanges()` |
| 5 | Git Service | `apps/daemon/pkg/git/clone.go` | `CloneRepository()` |
| 6 | Git Service | `apps/daemon/pkg/git/commit.go` | `Commit()` |
| 7 | Git Service | `apps/daemon/pkg/git/push.go` | `Push()` |

### Library Used

`github.com/go-git/go-git/v5` - Pure Go git implementation

### Layer Responsibilities

**SDK Layer** (`libs/sdk-python/src/daytona/_sync/git.py`)
- All operations are simple API passthroughs with minimal transformation and `@intercept_errors` wrapping.
- **clone():** Builds `GitCloneRequest` with url, branch, path, commit_id, username, password. Sends to toolbox API.
- **commit():** Builds `GitCommitRequest` with path, message, author, email. Returns `GitCommitResponse` with hash (SHA).
- **push():** Builds `GitRepoRequest` with path, username, password.
- **pull():** Builds `GitRepoRequest` with path, username, password.
- **status():** Returns `GitStatus` with current_branch, file_status, ahead, behind, branch_published.
- **add():** Builds `GitAddRequest` with files list.
- **branches():** Returns `ListBranchResponse`.
- **checkout_branch() / create_branch() / delete_branch():** Build respective request DTOs.
- No complex transformation, validation, or polling — all direct API delegations.

**Proxy Layer** (`apps/proxy/pkg/proxy/`)
- Same transparent reverse proxy behavior as other toolbox flows. Authenticates, resolves runner, forwards request with auth headers.

**Daemon Layer** (`apps/daemon/pkg/toolbox/git/`, `apps/daemon/pkg/git/`)
- **Toolbox handlers** (`toolbox/git/`): HTTP route handlers that parse requests and delegate to the git service layer. `CloneRepository()` creates `git.Service` with WorkDir, builds `BasicAuth` if credentials provided. `CommitChanges()` delegates to `gitService.Commit()`. `PushChanges()` delegates to `gitService.Push()`.
- **Git service** (`pkg/git/`): Uses `github.com/go-git/go-git/v5` (pure Go git implementation). `CloneRepository()` calls `git.PlainClone()` with SingleBranch=true, optional BasicAuth, optional ReferenceName. If commit_id specified: checks out specific commit after clone. `Commit()` calls `git.PlainOpen(path)` → `repo.Worktree()` → `worktree.Commit(message, options)`. `Push()` calls `git.PlainOpen(path)` → `repo.Head()` to get current branch → `repo.Push(PushOptions{Auth, RefSpecs: [currentBranch:currentBranch]})`. `Pull()`, `Add()`, `Checkout()`, `CreateBranch()`, `DeleteBranch()` all delegate to go-git library methods.

---

## 10. Computer Use Flow

### Overview
Desktop automation with VNC, screenshots, mouse, and keyboard control.

### VNC Desktop Startup

```
SDK: sandbox.computer_use.start()
    │
    ▼
HTTP POST to Proxy: /computeruse/start
    │
    ▼
Daemon: StartComputerUse()
    │
    └─► ComputerUse.Start()
            │
            ├─► Start processes in order (2s gap between each):
            │       │
            │       ├─► Priority 100: Xvfb (X Virtual Framebuffer)
            │       │       Command: /usr/bin/Xvfb :0 -screen 0 1024x768x24
            │       │
            │       ├─► Priority 200: xfce4 (Desktop)
            │       │       Command: /usr/bin/startxfce4
            │       │
            │       ├─► Priority 300: x11vnc (VNC Server)
            │       │       Command: /usr/bin/x11vnc -display :0 -forever -rfbport 5901
            │       │
            │       └─► Priority 400: novnc (Web VNC)
            │               Command: websockify --vnc localhost:5901 --listen 6080
            │
            └─► Verify all processes running
```

### Screenshot Flow

```
SDK: sandbox.computer_use.screenshot.take_full_screen()
    │
    ▼
HTTP GET to Proxy: /computeruse/screenshot
    │
    ▼
Daemon: TakeScreenshot()
    │
    └─► ComputerUse.TakeScreenshot()
            │
            ├─► screenshot.CaptureRect(displayBounds)
            ├─► Convert to RGBA
            ├─► Draw cursor (if showCursor=true)
            │       └─► robotgo.Location() → get cursor position
            ├─► Encode to PNG
            └─► Return base64 string
```

### Mouse/Keyboard Operations

```
SDK: sandbox.computer_use.mouse.click(x, y)
    │
    ▼
HTTP POST to Proxy: /computeruse/mouse/click
    │
    Body: {x, y, button: "left", double: false}
    │
    ▼
Daemon: Click()
    │
    └─► ComputerUse.Click()
            │
            ├─► robotgo.Move(x, y)
            ├─► time.Sleep(100ms)
            └─► robotgo.Click(button, double)
```

### Code Path

| Step | Component | File | Function |
|------|-----------|------|----------|
| 1 | SDK | `libs/sdk-python/src/daytona/_sync/computer_use.py` | `start()` / `screenshot.*` / `mouse.*` |
| 2 | Daemon | `apps/daemon/pkg/toolbox/computeruse/handler.go` | HTTP handlers |
| 3 | Plugin | `libs/computer-use/pkg/computeruse/computeruse.go` | `Start()` |
| 4 | Plugin | `libs/computer-use/pkg/computeruse/screenshot.go` | `TakeScreenshot()` |
| 5 | Plugin | `libs/computer-use/pkg/computeruse/mouse.go` | `Click()` / `Move()` |
| 6 | Plugin | `libs/computer-use/pkg/computeruse/keyboard.go` | `TypeText()` / `PressKey()` |

### Libraries Used

- `github.com/go-vgo/robotgo` - Mouse/keyboard control
- `github.com/kbinani/screenshot` - Display capture
- `github.com/hashicorp/go-plugin` - Plugin system

### Layer Responsibilities

**SDK Layer** (`libs/sdk-python/src/daytona/_sync/computer_use.py`)
- All operations are simple API passthroughs with `@intercept_errors` wrapping.
- **Process management:** `start()` starts VNC desktop processes. `stop()` stops all processes. `get_status()`, `get_process_status(name)`, `restart_process(name)`, `get_process_logs(name)`, `get_process_errors(name)` — all direct API delegations.
- **Mouse operations:** `get_position()` returns `{x, y}`. `move(x, y)` returns position after move. `click(x, y, button, double)` supports left/right/middle + double-click. `drag(start_x, start_y, end_x, end_y, button)`. `scroll(x, y, direction, amount)` — direction = "up"/"down".
- **Keyboard operations:** `type(text, delay)` with millisecond delay between characters. `press(key, modifiers)` — modifiers = ["ctrl", "alt", "meta", "shift"]. `hotkey(keys)` — format "ctrl+c", "alt+tab".
- **Screenshot operations:** `take_full_screen(show_cursor)` returns base64 encoded image. `take_region(region, show_cursor)` with x/y/width/height. `take_compressed(options)` with format/quality/scale. `take_compressed_region(region, options)`.
- **Display operations:** `get_info()` returns display dimensions. `get_windows()` returns open window list with id/title.

**Proxy Layer** (`apps/proxy/pkg/proxy/`)
- Same transparent reverse proxy behavior as other toolbox flows.

**Daemon Layer** (`apps/daemon/pkg/toolbox/computeruse/handler.go`)
- **Handler:** HTTP route handlers wrapping `IComputerUse` interface. If plugin unavailable, all endpoints return "disabled" response. Registers routes under `/computeruse` group with sub-groups for mouse, keyboard, screenshot, display, and process management.
- **Plugin loading:** Attempts to load external plugin binary at (1) `/usr/local/lib/daytona-computer-use` (production) or (2) `~/.daytona/daytona-computer-use` (development fallback). Uses `hashicorp/go-plugin` for RPC communication between daemon and plugin process.

**Computer Use Plugin** (`libs/computer-use/pkg/computeruse/`)
- **Start** (`computeruse.go`): Starts 4 processes in priority order with 2-second gap between each: (1) **Xvfb** (X Virtual Framebuffer): `/usr/bin/Xvfb :0 -screen 0 1024x768x24`, (2) **xfce4** (Desktop): `/usr/bin/startxfce4`, (3) **x11vnc** (VNC Server): `/usr/bin/x11vnc -display :0 -forever -rfbport 5901`, (4) **novnc** (Web VNC): `websockify --vnc localhost:5901 --listen 6080`. Verifies all processes running.
- **Screenshot** (`screenshot.go`): `TakeScreenshot()` calls `screenshot.CaptureRect(displayBounds)`. Converts to RGBA. Draws cursor overlay if `showCursor=true` via `robotgo.Location()`. Encodes to PNG. Returns base64 string.
- **Mouse** (`mouse.go`): `Click()` calls `robotgo.Move(x, y)`, sleeps 100ms, then `robotgo.Click(button, double)`. `Move()` calls `robotgo.Move(x, y)`. `Drag()` moves to start, mouse down, moves to end, mouse up. `Scroll()` calls `robotgo.Scroll()` with direction and amount.
- **Keyboard** (`keyboard.go`): `TypeText()` calls `robotgo.TypeStr()` with optional delay. `PressKey()` calls `robotgo.KeyTap()` with modifiers. `Hotkey()` parses "ctrl+c" format and calls `robotgo.KeyTap()`.

---

## 11. Volume Management Flow

### Overview
Persistent S3-backed storage mounted to sandboxes.

### Volume Creation Flow

```
SDK: daytona.volume.create(name)
    │
    ▼
API: POST /volumes
    │
    ├─► VolumeController.createVolume()
    │       │
    │       ▼
    │   VolumeService.create()
    │       │
    │       ├─► Validate organization quotas
    │       ├─► Generate UUID
    │       ├─► Check for duplicate names
    │       ├─► Create Volume entity (state=PENDING_CREATE)
    │       └─► Save to database
    │
    ▼
VolumeManager (cron, every 5s)
    │
    ├─► processPendingVolumes()
    │       │
    │       ▼
    │   handlePendingCreate()
    │       │
    │       ├─► S3: CreateBucket("daytona-volume-{volumeId}")
    │       ├─► S3: PutBucketTagging(VolumeId, OrganizationId)
    │       └─► Update state = READY
```

### Volume Mount Flow (Sandbox Creation)

```
Runner: DockerClient.Create()
    │
    ├─► getVolumesMountPathBinds(volumes)
    │       │
    │       ▼
    │   For each volume:
    │       │
    │       ├─► Construct volume ID: "daytona-volume-{volumeId}"
    │       ├─► Get mount path: /mnt/daytona-volume-{volumeId}
    │       ├─► Check if already mounted
    │       │
    │       └─► Mount via FUSE:
    │               │
    │               ├─► os.MkdirAll(path, 0755)
    │               └─► exec: mount-s3 {volumeId} {path}
    │                       --allow-other
    │                       --allow-delete
    │                       --allow-overwrite
    │                       --prefix {subpath}/ (if subpath provided)
    │
    ├─► getContainerHostConfig()
    │       │
    │       └─► Add bind mounts: /mnt/daytona-volume-{id}/:{mountPath}/
    │
    └─► ContainerCreate() with bind mounts
```

### Code Path

| Step | Component | File | Function |
|------|-----------|------|----------|
| 1 | SDK | `libs/api-client-*/api/volumes-api.*` | `createVolume()` |
| 2 | API | `apps/api/src/sandbox/controllers/volume.controller.ts` | `createVolume()` |
| 3 | API | `apps/api/src/sandbox/services/volume.service.ts` | `create()` |
| 4 | API | `apps/api/src/sandbox/managers/volume.manager.ts` | `handlePendingCreate()` |
| 5 | Runner | `apps/runner/pkg/docker/volumes_mountpaths.go` | `getVolumesMountPathBinds()` |

### State Transitions

```
PENDING_CREATE → CREATING → READY → PENDING_DELETE → DELETING → DELETED
      │              │                    │              │
      ▼              ▼                    ▼              ▼
    ERROR         ERROR               ERROR          ERROR
```

### Layer Responsibilities

**SDK Layer** (`libs/api-client-*/api/volumes-api.*`)
- **create():** API passthrough. Calls `POST /volumes` with volume name and optional configuration. Returns volume entity.
- **list():** Paginated volume listing.
- **delete():** Calls `DELETE /volumes/{id}`.
- No complex transformation — uses auto-generated API client.

**API Layer** (`apps/api/src/sandbox/`)
- **VolumeController** (`controllers/volume.controller.ts`): `createVolume()` validates `CreateVolumeDto`, checks permissions (`WRITE_VOLUMES` decorator). `deleteVolume()` checks `DELETE_VOLUMES`. `listVolumes()` supports pagination.
- **VolumeService** (`services/volume.service.ts`): `create()` validates organization quotas (volume count limits), generates UUID, checks for duplicate names, creates Volume entity with state=PENDING_CREATE, saves to database. `delete()` sets state=PENDING_DELETE.
- **VolumeManager** (`managers/volume.manager.ts`): `processPendingVolumes()` cron runs every **5 seconds**. Acquires Redis lock `process-pending-volumes` (30s TTL). Queries volumes in PENDING_CREATE or PENDING_DELETE state. Per-volume lock: `volume-state-{volumeId}` (30s TTL). `handlePendingCreate()`: transitions PENDING_CREATE → CREATING, refreshes lock, executes S3 `CreateBucketCommand` to create bucket named `daytona-volume-{volumeId}`, adds tags (OrganizationId, VolumeId) via `PutBucketTaggingCommand`, refreshes lock, updates state to READY. On error: sets state=ERROR with reason. `handlePendingDelete()`: transitions to DELETING, deletes S3 bucket, updates state to DELETED.

**Runner Layer** (`apps/runner/pkg/docker/`)
- **Volume mounting during sandbox creation** (`volumes_mountpaths.go`): `getVolumesMountPathBinds()` runs for each volume in `CreateSandboxDTO`. Per-volume mutex prevents concurrent mount attempts. Constructs volume key from volume ID + subpath MD5 hash for uniqueness. Mount path: `/mnt/daytona-volume-{volumeId}` (production) or `/tmp/daytona-volume-{volumeId}` (dev). Checks if already FUSE-mounted via `isDirectoryMounted()`. If not mounted: creates directory (0755), builds `mount-s3` command with `--allow-other --allow-delete --allow-overwrite --file-mode 0666 --dir-mode 0777`, sets AWS environment variables (endpoint, access key, secret key, region), executes mount. Waits up to **5 seconds** for FUSE mount readiness (50 polls × 100ms), verifying via `mountpoint` command + `stat` + `ReadDir`. Returns bind mount strings (e.g., `/mnt/daytona-volume-{id}/:{containerMountPath}/`) for Docker container creation.
- **Container config** (`create.go`): `getContainerHostConfig()` adds volume bind mounts from `getVolumesMountPathBinds()` to container's `HostConfig.Binds`.

---

## 12. Authentication Flows

### Overview
JWT (OIDC) and API Key authentication mechanisms.

### JWT/OIDC Flow (Dashboard)

```
Browser: Visit dashboard
    │
    ▼
ConfigProvider: Fetch /api/config
    │
    └─► Returns OIDC configuration:
            {issuer, clientId, audience}
    │
    ▼
AuthProvider (react-oidc-context)
    │
    ├─► Check if authenticated
    │
    ▼ (if not authenticated)
signinRedirect()
    │
    └─► Redirect to OIDC provider
            │
            ▼
        User logs in at OIDC provider
            │
            ▼
        Redirect back with auth code
            │
            ▼
        Exchange code for tokens
            │
            ▼
        Tokens stored in userStore
    │
    ▼
ApiProvider: Create ApiClient with access_token
    │
    └─► All API requests include: Authorization: Bearer {jwt}
```

### API Key Flow (SDK/CLI)

```
SDK: Daytona(api_key="dtn_...")
    │
    ▼
All API requests include:
    Authorization: Bearer dtn_...
    │
    ▼
API Server: CombinedAuthGuard
    │
    ├─► Try JWT strategy (fails)
    │
    └─► Try API Key strategy
            │
            ▼
        ApiKeyStrategy.validate()
            │
            ├─► Check system API keys (ssh-gateway, proxy)
            │
            ├─► Check cache: api-key:validation:{hash}
            │       │
            │       ▼ (cache miss)
            │   ApiKeyService.getApiKeyByValue()
            │       │
            │       └─► SELECT FROM api_key WHERE keyHash = SHA256(token)
            │
            ├─► Check expiration
            │
            ├─► Cache result
            │
            ├─► Get user info (cached)
            │
            └─► Return AuthContext:
                    {userId, role, email, apiKey, organizationId}
```

### JWT Validation

```
API Server: JwtStrategy.validate()
    │
    ├─► Extract token from Authorization header
    │
    ├─► Fetch JWKS from {issuer}/.well-known/jwks.json
    │       (cached by jwks-rsa library)
    │
    ├─► Verify signature (RS256)
    │
    ├─► Verify claims:
    │       ├─► audience
    │       ├─► issuer
    │       └─► expiration
    │
    ├─► Find or create user
    │
    └─► Return AuthContext
```

### Code Path

| Step | Component | File | Function |
|------|-----------|------|----------|
| 1 | Dashboard | `apps/dashboard/src/providers/ConfigProvider.tsx` | OIDC config |
| 2 | Dashboard | `apps/dashboard/src/providers/ApiProvider.tsx` | Token injection |
| 3 | API | `apps/api/src/auth/combined-auth.guard.ts` | `canActivate()` |
| 4 | API | `apps/api/src/auth/jwt.strategy.ts` | `validate()` |
| 5 | API | `apps/api/src/auth/api-key.strategy.ts` | `validate()` |

### Guard Hierarchy

```
CombinedAuthGuard (JWT or API Key)
    │
    ▼
OrganizationAccessGuard
    │
    ├─► Verify organization membership
    └─► Cache organization/user data
    │
    ▼
OrganizationActionGuard
    │
    └─► Check required role (OWNER, MEMBER)
    │
    ▼
OrganizationResourceActionGuard
    │
    └─► Check specific permissions (READ_SANDBOXES, DELETE_VOLUMES, etc.)
```

### Cache Keys

| Cache | Key Pattern | TTL |
|-------|-------------|-----|
| API Key | `api-key:validation:{SHA256}` | Configurable |
| User | `api-key:user:{userId}` | Configurable |
| Organization | `organization:{orgId}` | 10s |
| Org User | `organization-user:{orgId}:{userId}` | 10s |
| JWKS | Internal (jwks-rsa) | Managed |

### Layer Responsibilities

**Dashboard Layer** (`apps/dashboard/`)
- **ConfigProvider** (`src/providers/ConfigProvider.tsx`): Fetches `/api/config` on mount to get OIDC configuration: `{issuer, clientId, audience}`. Provides config to auth provider.
- **AuthProvider** (`src/providers/AuthProvider.tsx`): Uses `react-oidc-context` library. Checks authentication state. If not authenticated: calls `signinRedirect()` to redirect browser to OIDC provider. On callback: exchanges authorization code for tokens via OIDC code flow. Stores tokens in `userStore`. Silent token renewal via `oidc-client-ts`.
- **ApiProvider** (`src/providers/ApiProvider.tsx`): Creates `ApiClient` instance with access_token. Injects `Authorization: Bearer {jwt}` header into all API requests.

**CLI/SDK Layer** (`apps/cli/auth/`, `libs/sdk-python/`)
- **CLI auth:** Interactive OIDC device flow or browser-based login. Stores credentials in config file. Supports API key creation/management.
- **SDK:** `Daytona(api_key="dtn_...")` constructor stores API key. All requests include `Authorization: Bearer dtn_...` header. No OIDC flow — API key only.

**API Layer** (`apps/api/src/auth/`)
- **CombinedAuthGuard** (`combined-auth.guard.ts`): `canActivate()` tries both JWT and API Key strategies. Returns first successful authentication. Applied to all protected endpoints.
- **JwtStrategy** (`jwt.strategy.ts`): Extracts token from `Authorization: Bearer` header. Fetches JWKS public keys from `{issuer}/.well-known/jwks.json` (cached by `jwks-rsa` library). Verifies RS256 signature. Validates claims: audience, issuer, expiration. Handles OKTA format (uid, cid claims). Extracts userId from `sub` or `uid`, email from `email` or `sub`. Auto-creates user if doesn't exist (with `defaultOrganizationQuota`). Updates email if changed. Returns `AuthContext{userId, role, email, organizationId}`.
- **ApiKeyStrategy** (`api-key.strategy.ts`): Extracts token from `Authorization: Bearer` header. Validation chain (6 steps): (1) SSH gateway API key (hardcoded config check), (2) Proxy API key (hardcoded config check), (3) User API key: queries database `SELECT FROM api_key WHERE keyHash = SHA256(token)`, checks expiration (`expiresAt > now`), updates `lastUsedAt`, caches in Redis at `api-key:validation:{hash}` with configurable TTL; (4) Runner API key, (5) Region proxy API key, (6) Region SSH gateway API key. Returns `AuthContextType{role, userId, email, apiKey, runnerId, organizationId, regionId}`.
- **OrganizationAccessGuard**: Verifies organization membership. Caches organization data at `organization:{orgId}` (10s TTL) and user membership at `organization-user:{orgId}:{userId}` (10s TTL).
- **OrganizationActionGuard**: Checks required role (OWNER, MEMBER).
- **OrganizationResourceActionGuard**: Checks specific permissions (READ_SANDBOXES, WRITE_SANDBOXES, DELETE_VOLUMES, etc.).

**Proxy Layer** (`apps/proxy/pkg/proxy/auth.go`)
- **Sandbox-level authentication** for preview URLs and toolbox access. 6-method auth chain (separate from API-level auth): (1) Bearer token, (2) X-Daytona-Preview-Token header, (3) DAYTONA_SANDBOX_AUTH_KEY query param, (4) encrypted secure cookie, (5) signed preview URL token via API lookup, (6) OIDC redirect with PKCE. Auth results cached **2 minutes**. On OIDC callback: exchanges code with PKCE verifier, validates sandbox access via API, sets encrypted cookie (HttpOnly, Secure, SameSite=None, 3600s TTL), redirects to original URL.

**SSH Gateway Layer** (`apps/ssh-gateway/main.go`)
- **Token-based authentication:** Extracts token from SSH username field. Validates via API `SandboxAPI.ValidateSshAccess(token)` on every connection. No caching — each connection validates fresh. Uses SSH gateway system API key for API calls.

---

## Summary

This document covers all major flows in the Daytona system:

| Flow | Entry Point | Key Components |
|------|-------------|----------------|
| Sandbox Creation | `Daytona.create()` | API → Runner → Docker → Daemon |
| Sandbox Start/Stop | `sandbox.start()` / `stop()` | API → SandboxManager → Runner → Docker |
| Command Execution | `sandbox.process.exec()` | SDK → Proxy → Daemon Toolbox |
| File Operations | `sandbox.fs.upload()` / `download()` | SDK → Proxy → Daemon FS |
| SSH Access | `sandbox.create_ssh_access()` | API → SSH Gateway → Runner → Daemon SSH |
| Preview URLs | `sandbox.get_preview_link()` | API → Proxy → Daemon Service |
| Snapshot Creation | `daytona snapshot create` | CLI → API → SnapshotManager → Runner → Docker |
| Code Interpreter | `sandbox.code_interpreter.run_code()` | SDK → Daemon → Python Worker |
| Git Operations | `sandbox.git.clone()` / `commit()` / `push()` | SDK → Daemon → go-git |
| Computer Use | `sandbox.computer_use.start()` / `screenshot()` | SDK → Daemon → Plugin → robotgo |
| Volume Management | `daytona.volume.create()` | API → VolumeManager → S3 → Runner mount-s3 |
| Authentication | Login / API Key | OIDC Provider / API Key Strategy |

Each flow has been traced from entry point through all system layers with exact file paths and function names.
