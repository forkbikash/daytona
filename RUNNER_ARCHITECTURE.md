# Daytona Runner Architecture

The Runner is a Go service that manages Docker containers on behalf of the Daytona API. It handles sandbox lifecycle (create, start, stop, destroy), image operations (pull, build, push), S3 volume mounting via FUSE, network isolation via iptables, backup/snapshot management, and SSH forwarding into sandbox containers.

---

## Table of Contents

1. [Directory Structure](#1-directory-structure)
2. [Initialization & Startup Sequence](#2-initialization--startup-sequence)
3. [High-Level Architecture](#3-high-level-architecture)
4. [Package Breakdown](#4-package-breakdown)
   - [cmd/runner — Entry Point & Config](#41-cmdrunner--entry-point--config)
   - [pkg/api — REST API Server](#42-pkgapi--rest-api-server)
   - [pkg/docker — Container Orchestration](#43-pkgdocker--container-orchestration)
   - [pkg/runner — Singleton & V2 Job System](#44-pkgrunner--singleton--v2-job-system)
   - [pkg/services — State Management & Sync](#45-pkgservices--state-management--sync)
   - [pkg/sshgateway — SSH Forwarding](#46-pkgsshgateway--ssh-forwarding)
   - [pkg/netrules — Network Isolation](#47-pkgnetrules--network-isolation)
   - [pkg/cache — In-Memory State Cache](#48-pkgcache--in-memory-state-cache)
   - [pkg/common — Shared Utilities](#49-pkgcommon--shared-utilities)
   - [pkg/models — Data Models & Enums](#410-pkgmodels--data-models--enums)
   - [pkg/storage — Object Storage Client](#411-pkgstorage--object-storage-client)
   - [pkg/apiclient — API Client Wrapper](#412-pkgapiclient--api-client-wrapper)
   - [pkg/daemon — Embedded Binaries](#413-pkgdaemon--embedded-binaries)
   - [internal/ — Build Info, Constants, Metrics, Util](#414-internal--build-info-constants-metrics-util)
5. [REST API Endpoints](#5-rest-api-endpoints)
6. [V2 Job Queue System](#6-v2-job-queue-system)
7. [Docker Operations](#7-docker-operations)
8. [Sandbox State Machine](#8-sandbox-state-machine)
9. [Network Rules (iptables)](#9-network-rules-iptables)
10. [Volume Mounting (S3 FUSE)](#10-volume-mounting-s3-fuse)
11. [Backup & Snapshot Operations](#11-backup--snapshot-operations)
12. [SSH Gateway](#12-ssh-gateway)
13. [Metrics & Observability](#13-metrics--observability)
14. [Configuration Reference](#14-configuration-reference)
15. [Deployment & Packaging](#15-deployment--packaging)

---

## 1. Directory Structure

```
apps/runner/
├── .env                          # Default environment variables
├── Dockerfile                    # Container build definition
├── project.json                  # Nx project config
├── go.mod / go.sum               # Go module definition
│
├── cmd/runner/
│   ├── main.go                   # Entry point, initialization sequence
│   └── config/
│       └── config.go             # Environment-based configuration struct
│
├── internal/
│   ├── buildinfo.go              # Compile-time version injection
│   ├── constants/
│   │   ├── auth.go               # Auth constants (Bearer keyword)
│   │   └── keys.go               # Context key constants
│   ├── metrics/
│   │   └── collector.go          # System metrics collector (CPU, memory, disk)
│   └── util/
│       ├── error_extract.go      # Error message extraction
│       ├── log_formatter.go      # Log formatting
│       └── log_writer.go         # Log writer adapters
│
├── pkg/
│   ├── api/
│   │   ├── server.go             # Gin HTTP server setup, route registration
│   │   ├── validator.go          # Custom request validator
│   │   ├── controllers/
│   │   │   ├── sandbox.go        # Sandbox CRUD endpoints
│   │   │   ├── snapshot.go       # Image/snapshot endpoints
│   │   │   ├── proxy.go          # Toolbox proxy (HTTP + WebSocket)
│   │   │   ├── command_logs.go   # WebSocket log streaming
│   │   │   ├── health.go         # Health check endpoint
│   │   │   └── info.go           # Runner info & Prometheus metrics
│   │   ├── dto/
│   │   │   ├── sandbox.go        # CreateSandboxDTO, ResizeSandboxDTO, etc.
│   │   │   ├── snapshot.go       # BuildSnapshotRequestDTO, PullSnapshotRequestDTO
│   │   │   ├── backup.go         # CreateBackupDTO
│   │   │   ├── image.go          # TagImageRequestDTO
│   │   │   ├── info.go           # RunnerInfoResponseDTO, RunnerMetrics
│   │   │   ├── registry.go       # RegistryDTO
│   │   │   └── volume.go         # VolumeDTO
│   │   ├── middlewares/
│   │   │   ├── auth.go           # Bearer token authentication
│   │   │   ├── logging.go        # Request/response logging
│   │   │   └── recoverable_errors.go  # Recoverable error transformation
│   │   └── docs/
│   │       ├── docs.go           # Swagger metadata
│   │       ├── swagger.json      # OpenAPI spec (JSON)
│   │       └── swagger.yaml      # OpenAPI spec (YAML)
│   │
│   ├── docker/
│   │   ├── client.go             # DockerClient struct & constructor
│   │   ├── create.go             # Container creation (pull → config → create → start)
│   │   ├── start.go              # Container start + daemon wait
│   │   ├── stop.go               # Graceful stop with SIGKILL + retry
│   │   ├── destroy.go            # Container removal + rule cleanup
│   │   ├── state.go              # State deduction from Docker status
│   │   ├── container_configs.go  # CPU, memory, disk, env, labels, entrypoint
│   │   ├── container_exec.go     # Exec into container (synchronous)
│   │   ├── container_inspect.go  # Container inspection wrapper
│   │   ├── container_commit.go   # Container commit to image
│   │   ├── daemon.go             # Start daemon inside container + health poll
│   │   ├── daemon_version.go     # Query daemon version endpoint
│   │   ├── image_build.go        # Dockerfile build with context from S3
│   │   ├── image_pull.go         # Pull image from registry
│   │   ├── image_push.go         # Push image to registry
│   │   ├── image_exists.go       # Local image existence check
│   │   ├── image_info.go         # Image metadata (size, entrypoint, hash)
│   │   ├── image_remove.go       # Image deletion
│   │   ├── tag_image.go          # Image tagging
│   │   ├── snapshot_build.go     # Build + tag + optional push
│   │   ├── snapshot_pull.go      # Pull + tag + optional re-push
│   │   ├── backup.go             # Backup (commit → push, with export fallback)
│   │   ├── volumes_mountpaths.go # S3 FUSE mounting via mount-s3
│   │   ├── volumes_cleanup.go    # Orphaned volume cleanup
│   │   ├── network.go            # Apply network settings to container
│   │   ├── resize.go             # Live CPU/memory resize
│   │   ├── recover.go            # Sandbox recovery orchestration
│   │   ├── recover_from_storage_limit.go  # Storage expansion via rsync
│   │   └── monitor.go            # Docker event monitor + rule reconciliation
│   │
│   ├── runner/
│   │   ├── runner.go             # Runner singleton (holds all services)
│   │   └── v2/
│   │       ├── executor/
│   │       │   ├── executor.go   # Job dispatch + status reporting + tracing
│   │       │   ├── sandbox.go    # Sandbox job handlers (create, start, stop, destroy)
│   │       │   ├── snapshot.go   # Snapshot job handlers (build, pull, remove, inspect)
│   │       │   └── backup.go     # Backup job handler
│   │       ├── poller/
│   │       │   └── poller.go     # Long-poll API for pending jobs
│   │       └── healthcheck/
│   │           └── healthcheck.go # Periodic health + metrics reporting
│   │
│   ├── services/
│   │   ├── sandbox.go            # SandboxService (state query, cache)
│   │   └── sandbox_sync.go       # SandboxSyncService (local ↔ remote reconciliation)
│   │
│   ├── sshgateway/
│   │   ├── config.go             # SSH config, key management
│   │   └── service.go            # SSH server + channel forwarding
│   │
│   ├── netrules/
│   │   ├── netrules.go           # NetRulesManager struct, Start/Stop
│   │   ├── set.go                # Create iptables chains with allow-list
│   │   ├── assign.go             # Enable rules in DOCKER-USER chain
│   │   ├── unassign.go           # Disable rules without deleting chain
│   │   ├── delete.go             # Full chain deletion
│   │   ├── limiter.go            # Packet marking for egress rate limiting
│   │   └── utils.go              # CIDR parsing, chain naming, rule listing
│   │
│   ├── cache/
│   │   └── cache.go              # StatesCache (sandbox + backup state)
│   │
│   ├── common/
│   │   ├── container.go          # GetContainerIpAddress()
│   │   ├── daemon.go             # DAEMON_PATH constant
│   │   ├── errors.go             # Docker error → HTTP status mapping
│   │   ├── metrics.go            # Prometheus histogram + counter definitions
│   │   ├── recovery.go           # Recoverable error detection + formatting
│   │   ├── rsync.go              # rsync wrapper for data migration
│   │   └── storage.go            # Storage size parsing (GB, MB, bytes)
│   │
│   ├── models/
│   │   ├── cached_states.go      # CachedStates struct
│   │   ├── recovery_type.go      # RecoveryType enum
│   │   ├── system_metrics.go     # RunnerHealthMetrics struct
│   │   └── enums/
│   │       ├── sandbox_state.go  # SandboxState enum (14 states)
│   │       └── snapshot_state.go # SnapshotState enum
│   │
│   ├── storage/
│   │   ├── client.go             # Storage client interface
│   │   └── minio_client.go       # MinIO/S3 client implementation
│   │
│   ├── apiclient/
│   │   ├── api_client.go         # API client factory
│   │   └── error_handler.go      # API error response handling
│   │
│   └── daemon/
│       ├── assets.go             # Embedded binary extraction (go:embed)
│       ├── util.go               # Binary write helpers
│       └── static/
│           └── .gitkeep          # Placeholder for embedded binaries
│
└── packaging/
    ├── deb/DEBIAN/
    │   ├── control               # Debian package metadata
    │   ├── postinst              # Post-install script
    │   ├── postrm                # Post-remove script
    │   └── prerm                 # Pre-remove script
    └── systemd/
        └── daytona-runner.service # systemd unit file
```

**Statistics:** 94 Go source files, 29 files in `pkg/docker/` alone.

---

## 2. Initialization & Startup Sequence

The runner starts from `cmd/runner/main.go`. The initialization is ordered so that each dependency is available before its consumers.

```
init()                                      [runs before main()]
  │
  ├─► Load .env file (non-fatal if missing)
  ├─► Configure logrus (level from LOG_LEVEL, optional file output)
  ├─► Configure zerolog (syncs level with logrus)
  └─► Configure stdlib log (routes to DebugLogWriter)

main()
  │
  ├─ 1. config.GetConfig()                  Load & validate env vars
  │
  ├─ 2. client.NewClientWithOpts()          Create Docker API client
  │      (FromEnv, WithAPIVersionNegotiation)
  │
  ├─ 3. netrules.NewNetRulesManager()       Create iptables manager
  │      netRulesManager.Start()            Start persistence loop (if non-dev)
  │
  ├─ 4. daemon.WriteStaticBinary()          Extract embedded daemon + plugin binaries
  │      ("daemon-amd64", "daytona-computer-use")
  │
  ├─ 5. context.WithCancel(Background)      Create cancellable root context
  │
  ├─ 6. cache.GetStatesCache()              Initialize in-memory state cache
  │      docker.NewDockerClient()           Wrap Docker client with config
  │
  ├─ 7. docker.NewDockerMonitor()           Create Docker event monitor
  │      ► goroutine: monitor.Start()       Listens for container events
  │
  ├─ 8. services.NewSandboxService()        Create sandbox state service
  │
  ├─ 9. services.NewSandboxSyncService()    Create state sync service
  │      syncService.StartSyncProcess()     ► goroutine: periodic sync (10s)
  │
  ├─ 10. sshgateway.NewService()            Create SSH gateway (if enabled)
  │       ► goroutine: sshService.Start()   Listens on port 2220
  │
  ├─ 11. newSLogger()                       Create slog structured logger
  │
  ├─ 12. metrics.NewCollector()             Create system metrics collector
  │       metricsCollector.Start()          ► goroutine: periodic collection
  │
  ├─ 13. runner.GetInstance()               Initialize runner singleton
  │
  ├─ 14. [API v2 only]
  │      ├─ healthcheck.NewService()        Create healthcheck reporter
  │      │   ► goroutine: healthcheck.Start()  Periodic metrics push (30s)
  │      │
  │      ├─ executor.NewExecutor()          Create job executor
  │      │
  │      └─ poller.NewService()             Create job poller
  │          ► goroutine: poller.Start()    Long-polls API for jobs
  │
  ├─ 15. api.NewApiServer()                 Create Gin HTTP server
  │       ► goroutine: apiServer.Start()    Serves on API_PORT (default 8080)
  │
  └─ 16. signal.Notify(os.Interrupt)        Block until SIGINT
         apiServer.Stop()                   Graceful shutdown
         defer: monitor.Stop()
         defer: netRulesManager.Stop()
```

**Goroutines spawned at startup:**

| # | Service | Purpose | Signal |
|---|---------|---------|--------|
| 1 | Docker Monitor | Listen for container lifecycle events, reconcile network rules | `monitor.Stop()` |
| 2 | Sandbox Sync | Push local container states to API every 10s | `ctx.Done()` |
| 3 | SSH Gateway | Accept SSH connections on port 2220 (if enabled) | `ctx.Done()` |
| 4 | Metrics Collector | Collect CPU/memory/disk/container metrics | `ctx.Done()` |
| 5 | Healthcheck | Push metrics to API every 30s (v2 only) | `ctx.Done()` |
| 6 | Poller | Long-poll API for pending jobs (v2 only) | `ctx.Done()` |
| 7 | API Server | Serve REST API on configured port | `apiServer.Stop()` |

---

## 3. High-Level Architecture

```
                        ┌─────────────────────────────────────┐
                        │          Daytona API Server          │
                        │           (NestJS, remote)           │
                        └──────┬───────────────┬──────────────┘
                               │               │
                    Health/Metrics          Job Queue
                    Push (v2)              Long-Poll (v2)
                               │               │
┌──────────────────────────────┼───────────────┼──────────────────────────────┐
│                              │               │                              │
│  RUNNER PROCESS              │               │                              │
│                              ▼               ▼                              │
│  ┌──────────────┐   ┌──────────────┐  ┌──────────────┐                     │
│  │  Healthcheck │   │    Poller    │  │   Executor   │                     │
│  │  (periodic)  │   │ (long-poll)  │──│ (job dispatch)│                    │
│  └──────┬───────┘   └──────────────┘  └──────┬───────┘                     │
│         │                                     │                             │
│         ▼                                     ▼                             │
│  ┌──────────────┐                    ┌──────────────────┐                   │
│  │   Metrics    │                    │   DockerClient   │◄── 29 files      │
│  │  Collector   │                    │  (orchestration) │                   │
│  └──────────────┘                    └──┬────┬────┬─────┘                   │
│                                         │    │    │                         │
│                              ┌──────────┘    │    └──────────┐              │
│                              ▼               ▼               ▼              │
│                    ┌──────────────┐  ┌──────────────┐ ┌─────────────┐      │
│                    │    Docker    │  │  NetRules    │ │   Storage   │      │
│                    │    Engine    │  │  (iptables)  │ │   (MinIO)   │      │
│                    └──────┬───────┘  └──────────────┘ └─────────────┘      │
│                           │                                                 │
│                           ▼                                                 │
│                    ┌──────────────┐                                         │
│                    │  Containers  │ ◄── Each runs a Daemon (Toolbox)       │
│                    │  (sandboxes) │                                         │
│                    └──────┬───────┘                                         │
│                           │                                                 │
│  ┌──────────────┐         │         ┌──────────────────┐                   │
│  │  API Server  │         │         │  Docker Monitor   │                  │
│  │    (Gin)     │─────────┤         │  (event listener) │                  │
│  └──────────────┘         │         └──────────────────┘                   │
│                           │                                                 │
│  ┌──────────────┐         │         ┌──────────────────┐                   │
│  │ SSH Gateway  │─────────┘         │  Sandbox Sync    │                  │
│  │ (port 2220)  │                   │  (10s interval)  │                  │
│  └──────────────┘                   └──────────────────┘                   │
│                                                                             │
│  ┌──────────────┐                   ┌──────────────────┐                   │
│  │ States Cache │                   │  Runner Singleton │                  │
│  │ (in-memory)  │                   │  (holds all svcs) │                  │
│  └──────────────┘                   └──────────────────┘                   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Package Breakdown

### 4.1 cmd/runner — Entry Point & Config

**`cmd/runner/main.go`** — Orchestrates the full initialization sequence described in section 2. Contains `init()` for logging setup and `main()` for service creation, goroutine launch, and signal handling.

**`cmd/runner/config/config.go`** — Configuration struct populated from environment variables via `envconfig`. Validates all fields with `go-playground/validator`. Auto-detects `RUNNER_DOMAIN` from system routing tables (netlink) if not set. Applies defaults and minimum bounds for all timeouts and intervals.

---

### 4.2 pkg/api — REST API Server

**`server.go`** — Creates Gin HTTP server. Registers routes in two groups: public (health check, Swagger docs in dev mode) and protected (all other endpoints behind Bearer auth middleware). Supports optional TLS via cert/key files.

**Middleware chain (applied in order):**

| Order | Middleware | File | Purpose |
|-------|-----------|------|---------|
| 1 | Recovery | common-go | Catches panics, converts to error responses |
| 2 | Logging | `middlewares/logging.go` | Logs method, URI, status, latency for every request |
| 3 | Error | common-go | Converts errors to structured HTTP responses |
| 4 | Recoverable Errors | `middlewares/recoverable_errors.go` | Post-handler: wraps recoverable errors as `{errorReason, recoverable: true}` with HTTP 400 |
| 5 | Auth | `middlewares/auth.go` | Validates `Authorization: Bearer <token>` or `X-Daytona-Authorization: Bearer <token>` against configured API token |

**`validator.go`** — Custom Gin validator wrapping `go-playground/validator/v10`. Uses `validate` struct tag (not default `binding`). Supports custom `optional` tag that always passes.

**`controllers/proxy.go`** — Reverse-proxies requests to sandbox toolbox. Gets container IP via Docker inspect, builds target URL `http://{ip}:2280{path}`. Special case: paths matching `^/process/session/.+/command/.+/logs$` with `follow=true` query param upgrade to WebSocket using `gorilla/websocket`.

---

### 4.3 pkg/docker — Container Orchestration

The largest package (29 files). Wraps the Docker Engine API with Daytona-specific orchestration logic.

**`client.go` — DockerClient struct:**

```
DockerClient
  ├── apiClient                 Docker Engine API client
  ├── statesCache               In-memory sandbox state cache
  ├── logWriter                 Debug log writer
  ├── netRulesManager           iptables network rules manager
  │
  ├── awsRegion                 AWS region for S3 volume mounts
  ├── awsEndpointUrl            S3-compatible endpoint URL
  ├── awsAccessKeyId            AWS access key
  ├── awsSecretAccessKey        AWS secret key
  │
  ├── daemonPath                Path to daemon binary on disk
  ├── computerUsePluginPath     Path to computer-use plugin binary
  │
  ├── volumeMutexes             map[string]*sync.Mutex — per-volume concurrency
  ├── volumeMutexesMutex        sync.Mutex — protects volumeMutexes map
  ├── volumeCleanupMutex        sync.Mutex — throttles cleanup runs
  ├── lastVolumeCleanup         time.Time — last cleanup timestamp
  ├── volumeCleanupIntervalSec  Cleanup throttle interval
  │
  ├── resourceLimitsDisabled    Skip CPU/memory limits if true
  ├── daemonStartTimeoutSec     Daemon startup timeout (default 60)
  ├── sandboxStartTimeoutSec    Container start timeout (default 30)
  ├── useSnapshotEntrypoint     Use image entrypoint instead of daemon
  └── backupTimeoutMin          Backup operation timeout (default 60)
```

**Key methods:**

| Method | File | Purpose |
|--------|------|---------|
| `Create()` | `create.go` | Full sandbox creation: pull image → validate arch → mount volumes → configure container → create → start → apply network rules |
| `Start()` | `start.go` | Start container, wait for running state, spawn daemon, wait for daemon HTTP, apply egress limiter |
| `Stop()` | `stop.go` | Stop with retry (SIGKILL + 2s timeout), wait for NotRunning, fallback to ContainerKill |
| `Destroy()` | `destroy.go` | Cancel backup, stop if needed, remove container (force), delete iptables rules |
| `RemoveDestroyed()` | `destroy.go` | Final cleanup of already-destroyed container |
| `DeduceSandboxState()` | `state.go` | Maps Docker container status → Daytona sandbox state enum |
| `PullImage()` | `image_pull.go` | Pull from registry, skip if local (except `:latest`), platform linux/amd64 |
| `BuildImage()` | `image_build.go` | Build from Dockerfile + S3 context, multi-registry auth, platform linux/amd64 |
| `PushImage()` | `image_push.go` | Push to registry with auth |
| `ImageExists()` | `image_exists.go` | Check local images by RepoTags/RepoDigests |
| `RemoveImage()` | `image_remove.go` | Delete image, prune children, idempotent |
| `GetImageInfo()` | `image_info.go` | Size, entrypoint, cmd, digest hash |
| `InspectImageInRegistry()` | `image_info.go` | Remote digest via DistributionInspect |
| `BuildSnapshot()` | `snapshot_build.go` | Build → tag → optional push to internal registry |
| `PullSnapshot()` | `snapshot_pull.go` | Pull → tag → optional push to destination registry |
| `CreateBackup()` | `backup.go` | Synchronous: commit → push (with export/import fallback) |
| `CreateBackupAsync()` | `backup.go` | Asynchronous: same as above in goroutine |
| `getVolumesMountPathBinds()` | `volumes_mountpaths.go` | FUSE-mount S3 volumes via mount-s3 |
| `CleanupOrphanedVolumeMounts()` | `volumes_cleanup.go` | Unmount/remove FUSE mounts not attached to any container |
| `UpdateNetworkSettings()` | `network.go` | Apply iptables allow-list, block-all, egress limiter |
| `Resize()` | `resize.go` | Live update CPU and memory limits |
| `RecoverSandbox()` | `recover.go` | Orchestrate recovery based on error type |
| `RecoverFromStorageLimit()` | `recover_from_storage_limit.go` | Expand storage via new container + rsync data migration |
| `startDaytonaDaemon()` | `daemon.go` | Exec daemon binary inside container |
| `waitForDaemonRunning()` | `daemon.go` | HTTP poll `http://{ip}:2280/version` every 5ms |

**Container Configuration (`container_configs.go`):**

| Setting | Value |
|---------|-------|
| Hostname | sandbox ID |
| Privileged | `true` |
| CPU | `CPUPeriod=100000`, `CPUQuota = dto.CpuQuota * 100000` |
| Memory | `Memory = GBToBytes(quota)`, `MemorySwap = same` (no swap) |
| Disk | XFS: `StorageOpt["size"] = "<GB>G"`, other FS: no quota |
| Extra hosts | `host.docker.internal:host-gateway` |
| Env vars | `DAYTONA_SANDBOX_ID`, `DAYTONA_SANDBOX_SNAPSHOT`, `DAYTONA_SANDBOX_USER`, custom env |
| Labels | `daytona.organization_id`, `daytona.organization_name` |
| Entrypoint | Default: `["/usr/local/bin/daytona"]` (daemon binary); optionally snapshot's original entrypoint |
| Volumes | Daemon binary (ro), computer-use plugin (ro), S3 FUSE mounts |

**Docker Event Monitor (`monitor.go`):**

The `DockerMonitor` listens to the Docker event stream and performs two functions:

1. **Rule lifecycle:** On container start → assign network rules. On stop/kill → unassign rules, remove limiter. On destroy → delete all rules for that container.
2. **Reconciliation loop:** Every 60 seconds, scans all `DAYTONA-SB-*` iptables chains and rules in `DOCKER-USER`/`PREROUTING`. Detects orphaned rules (referencing non-existent container IPs), removes them. Handles Docker connection errors (EOF, reset) by reconnecting after 2 seconds.

---

### 4.4 pkg/runner — Singleton & V2 Job System

**`runner.go` — Runner Singleton:**

```go
var runner *Runner  // package-level singleton

type Runner struct {
    StatesCache      *cache.StatesCache
    Docker           *docker.DockerClient
    MetricsCollector *metrics.Collector
    SandboxService   *services.SandboxService
    NetRulesManager  *netrules.NetRulesManager
    SSHGatewayService *sshgateway.Service
}
```

Fatal on double-init or uninitialized access. Thread-unsafe (assumes single initialization in `main()`). Accessed globally via `runner.GetInstance(nil)` from controllers and services.

**V2 Executor (`v2/executor/executor.go`):**

Dispatches jobs by type to handler functions:

| Job Type | Handler | Payload DTO | Response |
|----------|---------|-------------|----------|
| `CREATE_SANDBOX` | `createSandbox()` | `CreateSandboxDTO` | `StartSandboxResponse{DaemonVersion}` |
| `START_SANDBOX` | `startSandbox()` | `map[string]string` | `StartSandboxResponse{DaemonVersion}` |
| `STOP_SANDBOX` | `stopSandbox()` | None (uses ResourceId) | nil |
| `DESTROY_SANDBOX` | `destroySandbox()` | None (uses ResourceId) | nil |
| `CREATE_BACKUP` | `createBackup()` | `CreateBackupDTO` | nil |
| `BUILD_SNAPSHOT` | `buildSnapshot()` | `BuildSnapshotRequestDTO` | `SnapshotInfoResponse` |
| `PULL_SNAPSHOT` | `pullSnapshot()` | `PullSnapshotRequestDTO` | `SnapshotInfoResponse` |
| `REMOVE_SNAPSHOT` | `removeSnapshot()` | String (snapshot name) | nil |
| `INSPECT_SNAPSHOT_IN_REGISTRY` | `inspectSnapshotInRegistry()` | `InspectSnapshotInRegistryRequestDTO` | `SnapshotDigestResponse` |
| `UPDATE_SANDBOX_NETWORK_SETTINGS` | `updateNetworkSettings()` | `UpdateNetworkSettingsDTO` | nil |
| `RECOVER_SANDBOX` | `recoverSandbox()` | `RecoverSandboxDTO` | nil |

After execution, reports status back to API via `JobsAPI.UpdateJobStatus()` with `COMPLETED` or `FAILED` status and optional JSON result metadata.

**OpenTelemetry integration:** Extracts W3C trace context from job, creates spans for execution and status update, records errors and attributes.

**Error classification:** Uses `common.FormatRecoverableError()` to detect patterns like "no space left on device", "disk quota exceeded" and wrap them as `{errorReason, recoverable: true}` JSON.

**V2 Poller (`v2/poller/poller.go`):**

1. **Startup recovery:** Fetches all `IN_PROGRESS` jobs and re-executes them in goroutines.
2. **Main loop:** Long-polls `JobsAPI.PollJobs()` with configurable timeout (default 30s) and limit (default 10). API holds the request until jobs are available or timeout expires. HTTP 408 = normal timeout, returns empty list and polls again immediately. Other errors trigger a 5-second backoff.
3. **Job dispatch:** Each job spawns a goroutine calling `executor.Execute()`. No worker pool or backpressure — all jobs execute concurrently.

**V2 Healthcheck (`v2/healthcheck/healthcheck.go`):**

Sends health + metrics report to API at fixed intervals (default 30s). Collects via `metricsCollector.Collect()`:

- CPU load average and usage percentage
- Memory usage percentage
- Disk usage percentage
- Allocated CPU / memory / disk across all sandboxes
- Snapshot count (cached images)
- Started sandbox count
- Total CPU / memory / disk capacity

Reports to `RunnersAPI.RunnerHealthcheck()` with runner version, domain, apiUrl, proxyUrl (scheme from TLS config).

---

### 4.5 pkg/services — State Management & Sync

**`sandbox.go` — SandboxService:**

| Method | Purpose |
|--------|---------|
| `GetSandboxStatesInfo(sandboxId)` | Calls `docker.DeduceSandboxState()` to derive state from container status, writes to cache, returns `CachedStates{SandboxState, BackupState, BackupErrorReason}` |
| `RemoveDestroyedSandbox(sandboxId)` | Validates state is destroyed/destroying, then calls `docker.Destroy()` |

**`sandbox_sync.go` — SandboxSyncService:**

Bi-directional state reconciliation between local Docker containers and remote API. Runs every 10 seconds via a background goroutine.

| Method | Purpose |
|--------|---------|
| `GetLocalContainerStates()` | Lists all Docker containers, calls `DeduceSandboxState()` for each, returns `map[sandboxId]SandboxState` |
| `GetRemoteSandboxStates()` | Queries API for STARTED sandboxes assigned to this runner (skips reconciling sandboxes), returns `map[sandboxId]SandboxState` |
| `PerformSync()` | Compares local vs remote, pushes local state to API via `UpdateSandboxState()` for any mismatch |
| `StartSyncProcess()` | Initial sync + periodic ticker at configured interval |

State conversion functions handle 10+ state types: Creating, Restoring, Destroyed, Started, Stopped, PullingSnapshot, Starting, Stopping, Destroying, Error, Unknown.

---

### 4.6 pkg/sshgateway — SSH Forwarding

Transparent SSH proxy that forwards client connections through the runner to sandbox containers.

**Connection flow:**

```
External SSH Client
    │
    ▼ TCP port 2220
SSH Gateway (runner)
    │
    ├─► SSH handshake with public key auth
    ├─► Extract sandbox ID from username
    ├─► Docker inspect → get container IP
    │
    ▼ TCP port 22220
Sandbox Daemon SSH Server
    │
    ├─► Password auth: "sandbox-ssh"
    ├─► User: "daytona"
    └─► Bidirectional channel + data forwarding
```

**Key details:**
- **Public key authentication:** Validates client's public key against `SSH_PUBLIC_KEY` env var (base64 encoded).
- **Host key management:** Reads from `SSH_HOST_KEY_PATH` (default `/root/.ssh/id_rsa`). Auto-generates 2048-bit RSA key if missing.
- **Channel types:** Forwards all SSH channel types (session, direct-tcpip, subsystem) transparently. PTY allocation, window changes, exec, and shell commands are all proxied as-is.
- **Bidirectional forwarding:** 4 goroutines per channel: client→sandbox requests, sandbox→client requests, client→sandbox data (`io.Copy`), sandbox→client data.
- **Sandbox connection:** Dials `{containerIP}:22220` with password `"sandbox-ssh"`, user `"daytona"`, 30-second timeout, `InsecureIgnoreHostKey` (internal network).

---

### 4.7 pkg/netrules — Network Isolation

Manages per-sandbox network rules via `iptables` for traffic filtering and egress rate limiting.

**`NetRulesManager` struct:**

```go
type NetRulesManager struct {
    ipt        *iptables.IPTables    // iptables client
    mu         sync.Mutex            // thread-safe operations
    persistent bool                  // enable iptables-save persistence
    ctx        context.Context
    cancel     context.CancelFunc
}
```

**Operations:**

| Method | Table | Chain | Purpose |
|--------|-------|-------|---------|
| `SetNetworkRules(name, sourceIp, allowList)` | filter | Custom `DAYTONA-SB-{name}` + DOCKER-USER | Creates chain with per-CIDR RETURN rules + final DROP. Inserts jump from DOCKER-USER. |
| `AssignNetworkRules(name, sourceIp)` | filter | DOCKER-USER | Inserts jump rule to enable existing chain |
| `UnassignNetworkRules(name, sourceIp)` | filter | DOCKER-USER | Removes jump rule without deleting chain |
| `DeleteNetworkRules(name)` | filter | DOCKER-USER + custom chain | Finds and deletes all referencing rules, then clears and deletes chain |
| `SetNetworkLimiter(name, sourceIp)` | mangle | Custom chain + PREROUTING | Creates chain that marks packets with `--set-mark 999`, appends jump from PREROUTING |
| `RemoveNetworkLimiter(name, sourceIp)` | mangle | PREROUTING + custom chain | Removes jump rules and deletes chain |

**Persistence:** When `persistent=true` (non-development), a background goroutine runs `iptables-save > /etc/iptables/rules.v4` every 60 seconds. Ensures rules survive daemon restart.

**Chain naming:** All chains use prefix `DAYTONA-SB-` followed by the sandbox name.

**Allow-list flow:**

```
Packet from sandbox container
    │
    ▼
DOCKER-USER chain
    │
    ├─► Jump to DAYTONA-SB-{sandboxId} (match source IP)
    │       │
    │       ├─► RETURN if dest matches allowed CIDR #1
    │       ├─► RETURN if dest matches allowed CIDR #2
    │       └─► DROP (block everything else)
    │
    └─► Continue normal Docker routing
```

---

### 4.8 pkg/cache — In-Memory State Cache

**`StatesCache`** stores per-sandbox state information:

| Field | Type | Purpose |
|-------|------|---------|
| `SandboxState` | `enums.SandboxState` | Current lifecycle state |
| `BackupState` | `enums.BackupState` | Last backup state |
| `BackupErrorReason` | `*string` | Error reason if backup failed |

Used by controllers to return state quickly without Docker inspect calls. Cache is populated by `SandboxService.GetSandboxStatesInfo()` and `DockerClient` operations (create, start, stop, destroy).

---

### 4.9 pkg/common — Shared Utilities

| File | Functions | Purpose |
|------|-----------|---------|
| `container.go` | `GetContainerIpAddress()` | Extracts IP from Docker inspect NetworkSettings (bridge network) |
| `daemon.go` | `DAEMON_PATH` | Constant: `/usr/local/bin/daytona` |
| `errors.go` | `HandlePossibleDockerError()` | Maps containerd error types to HTTP status codes (401, 409, 400, 500) |
| `metrics.go` | `ContainerOperationDuration`, `ContainerOperationCount` | Prometheus histogram (0.1s–300s buckets) and counter, labeled by operation + status |
| `recovery.go` | `IsRecoverable()`, `FormatRecoverableError()`, `DeduceRecoveryType()` | Detects "no space left on device", "storage limit", "disk quota exceeded" → `RecoveryTypeStorageExpansion` |
| `rsync.go` | `RsyncCopy(src, dst)` | Wrapper for `rsync -aAX` with timeout, stdout/stderr capture |
| `storage.go` | `ParseStorageOptSizeGB()`, `GBToBytes()` | Parses Docker storage-opt sizes ("10G", "10240M", bytes), converts GB to bytes (1 GB = 1,073,741,824) |

---

### 4.10 pkg/models — Data Models & Enums

**`CachedStates`** — Combined sandbox + backup state returned by cache.

**`RecoveryType`** — Enum: `RecoveryTypeStorageExpansion`, `UnknownRecoveryType`.

**`RunnerHealthMetrics`** — System metrics struct sent to API: CPU load, CPU%, memory%, disk%, allocated resources, snapshot count, started sandboxes, total capacity.

**`enums/sandbox_state.go` — SandboxState enum (14 states):**

```
SandboxStateUnknown
SandboxStateCreating
SandboxStateStarting
SandboxStateStarted
SandboxStateStopping
SandboxStateStopped
SandboxStateDestroying
SandboxStateDestroyed
SandboxStateError
SandboxStatePullingSnapshot
SandboxStateBuildingSnapshot
SandboxStateRestoring
SandboxStateArchiving
SandboxStateArchived
```

---

### 4.11 pkg/storage — Object Storage Client

**`client.go`** — Interface for object storage operations (download context files for image builds).

**`minio_client.go`** — MinIO/S3 client implementation. Downloads build context files by SHA256 hash during `BuildImage()`.

---

### 4.12 pkg/apiclient — API Client Wrapper

**`api_client.go`** — Factory that creates the auto-generated `apiclient.APIClient` configured with the API base URL and runner bearer token.

**`error_handler.go`** — Parses API error responses for structured error messages.

---

### 4.13 pkg/daemon — Embedded Binaries

**`assets.go`** — Uses `//go:embed static/*` to embed daemon and computer-use plugin binaries at compile time.

**`util.go`** — `WriteStaticBinary(name)` extracts embedded binary to disk with executable permissions. Called at startup for `daemon-amd64` and `daytona-computer-use`.

---

### 4.14 internal/ — Build Info, Constants, Metrics, Util

| File | Purpose |
|------|---------|
| `buildinfo.go` | Compile-time `Version` variable injected via `-ldflags` |
| `constants/auth.go` | `BearerKeyword = "Bearer"` |
| `constants/keys.go` | Context key constants for request-scoped data |
| `metrics/collector.go` | System metrics collector: CPU load (via `/proc/loadavg` or `sysctl`), CPU usage %, memory %, disk %, allocated resources (sum across containers), snapshot count (Docker images), started sandbox count. Uses sliding window for averages (configurable `CollectorWindowSize`). |
| `util/error_extract.go` | Extracts error messages from nested error chains |
| `util/log_formatter.go` | Custom logrus formatter |
| `util/log_writer.go` | Adapts logrus logger to `io.Writer` interface |

---

## 5. REST API Endpoints

### Public Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/` | Health check — returns `{status: "ok", version: "..."}` |
| `GET` | `/api/*` | Swagger docs (development mode only) |

### Sandbox Endpoints (Protected)

| Method | Path | Handler | Request DTO | Response |
|--------|------|---------|-------------|----------|
| `POST` | `/sandboxes` | `Create` | `CreateSandboxDTO` | `StartSandboxResponse{DaemonVersion}` |
| `GET` | `/sandboxes/{id}` | `Info` | — | `SandboxInfoResponse{State, BackupState, DaemonVersion}` |
| `DELETE` | `/sandboxes/{id}` | `RemoveDestroyed` | — | `"Sandbox removed"` |
| `POST` | `/sandboxes/{id}/start` | `Start` | `map[string]string` | `StartSandboxResponse` |
| `POST` | `/sandboxes/{id}/stop` | `Stop` | — | `"Sandbox stopped"` |
| `POST` | `/sandboxes/{id}/destroy` | `Destroy` | — | `"Sandbox destroyed"` |
| `POST` | `/sandboxes/{id}/backup` | `CreateBackup` | `CreateBackupDTO` | `"Backup started"` |
| `POST` | `/sandboxes/{id}/resize` | `Resize` | `ResizeSandboxDTO` | `"Sandbox resized"` |
| `POST` | `/sandboxes/{id}/recover` | `Recover` | `RecoverSandboxDTO` | `"Sandbox recovered"` |
| `POST` | `/sandboxes/{id}/is-recoverable` | `IsRecoverable` | `IsRecoverableDTO` | `{recoverable: bool}` |
| `POST` | `/sandboxes/{id}/network-settings` | `UpdateNetworkSettings` | `UpdateNetworkSettingsDTO` | `"Network settings updated"` |
| `GET` | `/sandboxes/{id}/network-settings` | `GetNetworkSettings` | — | `UpdateNetworkSettingsDTO` |
| `GET/POST/DELETE` | `/sandboxes/{id}/toolbox/{path}` | `ProxyRequest` | Proxied | Proxied |

### Snapshot Endpoints (Protected)

| Method | Path | Handler | Request DTO | Response |
|--------|------|---------|-------------|----------|
| `POST` | `/snapshots/pull` | `PullSnapshot` | `PullSnapshotRequestDTO` | `"Snapshot pulled"` |
| `POST` | `/snapshots/build` | `BuildSnapshot` | `BuildSnapshotRequestDTO` | `"Snapshot built"` |
| `POST` | `/snapshots/tag` | `TagImage` (deprecated) | `TagImageRequestDTO` | `"Image tagged"` |
| `GET` | `/snapshots/exists` | `SnapshotExists` | query: `snapshot` | `{exists: bool}` |
| `GET` | `/snapshots/info` | `SnapshotInfo` | query: `snapshot` | `SnapshotInfoResponse` |
| `POST` | `/snapshots/remove` | `RemoveSnapshot` | query: `snapshot` | `"Snapshot removed"` |
| `GET` | `/snapshots/logs` | `StreamBuildLogs` | query: `snapshotRef`, `follow` | octet-stream |
| `POST` | `/snapshots/inspect` | `InspectInRegistry` | `InspectSnapshotInRegistryRequestDTO` | `SnapshotDigestResponse` |

### Info Endpoints (Protected)

| Method | Path | Handler | Response |
|--------|------|---------|----------|
| `GET` | `/info` | `GetInfo` | `RunnerInfoResponseDTO{Metrics, AppVersion}` |
| `GET` | `/metrics` | `GetMetrics` | Prometheus text format |

---

## 6. V2 Job Queue System

The V2 architecture replaces direct HTTP calls from the API to the runner with a job queue pattern. The API creates `Job` records in the database; the runner polls for them and executes them asynchronously.

```
┌─────────────┐     Job records      ┌──────────────┐
│  API Server  │ ──────────────────► │  PostgreSQL   │
│  (NestJS)    │                     │  (job table)  │
└─────────────┘                      └──────┬────────┘
                                            │
                                     Long-poll GET
                                            │
                                     ┌──────┴────────┐
                                     │    Poller      │
                                     │ (runner-side)  │
                                     └──────┬────────┘
                                            │
                                     Job dispatch
                                            │
                                     ┌──────┴────────┐
                                     │   Executor     │ ─► goroutine per job
                                     │ (runner-side)  │
                                     └──────┬────────┘
                                            │
                                     Status update
                                            │
                                     ┌──────┴────────┐
                                     │  API Server    │
                                     │ UpdateJobStatus│
                                     └───────────────┘
```

**Polling details:**
- Endpoint: `JobsAPI.PollJobs(ctx).Timeout(timeout).Limit(limit).Execute()`
- Default timeout: 30 seconds (server holds request)
- Default limit: 10 jobs per poll
- HTTP 408 = normal timeout → poll again immediately
- Other errors → 5-second backoff

**Job execution:**
- Each job runs in its own goroutine (no worker pool)
- OpenTelemetry span per job (continues API-side trace)
- Result reported via `UpdateJobStatus()` as COMPLETED or FAILED
- Recoverable errors wrapped as `{errorReason, recoverable: true}`

**Startup recovery:** On runner startup, the poller fetches all `IN_PROGRESS` jobs and re-executes them. This handles the case where the runner crashed mid-job.

---

## 7. Docker Operations

### Container Lifecycle

```
Create()
  ├─► DeduceSandboxState() — check idempotency
  ├─► PullImage() — pull from registry (skip if local & not :latest)
  ├─► ValidateArchitecture() — reject non-amd64
  ├─► getVolumesMountPathBinds() — FUSE-mount S3 volumes
  ├─► getContainerConfigs() — CPU, memory, disk, env, labels, entrypoint
  ├─► ContainerCreate() — Docker API
  └─► Start()
        ├─► ContainerStart() — Docker API
        ├─► waitForContainerRunning() — poll every 10ms, timeout 30s
        ├─► startDaytonaDaemon() — exec daemon binary inside container
        ├─► waitForDaemonRunning() — HTTP poll :2280/version every 5ms, timeout 60s
        └─► SetNetworkLimiter() — if metadata flag set

Stop()
  ├─► cancelBackup() — cancel in-progress backup
  ├─► stopContainerWithRetry() — SIGKILL + 2s, exponential backoff
  └─► ContainerWait(NotRunning) — block until stopped

Destroy()
  ├─► cancelBackup()
  ├─► ContainerRemove(force=true)
  └─► netRulesManager.DeleteNetworkRules() — async cleanup
```

### Backup Flow

```
CreateBackup()
  ├─► Cancel existing backup for same container
  ├─► context.WithTimeout(backupTimeoutMin)
  ├─► Set BackupState = InProgress
  ├─► ContainerCommit() — commit container to image
  │     └─► [on "failed to get digest" error] → Export/Import fallback
  │           ├─► ContainerExport() — filesystem tar
  │           └─► ImageImport() — with preserved config (CMD, ENV, WORKDIR, etc.)
  ├─► PushImage() — push to registry
  ├─► Set BackupState = Completed
  └─► RemoveImage() — cleanup local (non-fatal)
```

### Storage Recovery

```
RecoverFromStorageLimit()
  ├─► Inspect current container for overlay2 path
  ├─► Calculate expansion: 100MB increments, max 10% of original
  ├─► Validate XFS filesystem
  ├─► Stop container if running
  ├─► Rename old container: {id}-recovery-{timestamp}
  ├─► Create new container with expanded StorageOpt
  ├─► rsync -aAX from old overlay2 UpperDir to new (5-min timeout)
  ├─► Remove old container
  └─► Return (SandboxManager will restart)
```

---

## 8. Sandbox State Machine

State deduction from Docker container status (`state.go`):

| Docker Status | Exit Code | Daytona State |
|---------------|-----------|---------------|
| `created` | — | `Creating` |
| `running` | — | `Started` (default) |
| `running` | — | `PullingSnapshot` (if last logs contain "Pulling from", "Downloading", "Extracting") |
| `paused` | — | `Stopped` |
| `restarting` | — | `Starting` |
| `removing` | — | `Destroying` |
| `exited` | 0, 137, 143 | `Stopped` |
| `exited` | other | `Error` |
| `dead` | — | `Destroyed` |
| not found | — | `Destroyed` |

Exit code meanings: 0 = normal, 137 = SIGKILL, 143 = SIGTERM.

Full state enum (14 states):

```
Unknown ─── Creating ─── Starting ─── Started ─── Stopping ─── Stopped
                                         │                        │
                                         │                     Archiving ─── Archived
                                         │
                                      Destroying ─── Destroyed
                                         │
PullingSnapshot ─── BuildingSnapshot     │
                                         │
Restoring ───────────────────────────────┘

                                       Error
```

---

## 9. Network Rules (iptables)

### Architecture

```
┌──────────────────────────────────────────────────────┐
│                    FILTER TABLE                       │
│                                                      │
│  DOCKER-USER chain:                                  │
│  ┌─────────────────────────────────────────────────┐│
│  │ -j DAYTONA-SB-sandbox1 -s 172.17.0.2 -p all    ││
│  │ -j DAYTONA-SB-sandbox2 -s 172.17.0.3 -p all    ││
│  └─────────────────────────────────────────────────┘│
│                                                      │
│  DAYTONA-SB-sandbox1:                               │
│  ┌─────────────────────────────────────────────────┐│
│  │ -j RETURN -d 10.0.0.0/8      (allow private)   ││
│  │ -j RETURN -d 8.8.8.8/32      (allow DNS)       ││
│  │ -j DROP                       (block all else)  ││
│  └─────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────┐
│                    MANGLE TABLE                       │
│                                                      │
│  PREROUTING chain:                                   │
│  ┌─────────────────────────────────────────────────┐│
│  │ -j DAYTONA-SB-sandbox1-limiter -s 172.17.0.2   ││
│  └─────────────────────────────────────────────────┘│
│                                                      │
│  DAYTONA-SB-sandbox1-limiter:                       │
│  ┌─────────────────────────────────────────────────┐│
│  │ --set-mark 999                (mark for tc/QoS) ││
│  └─────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────┘
```

**Persistence:** When `persistent=true`, a goroutine runs `iptables-save > /etc/iptables/rules.v4` every 60 seconds.

**Reconciliation:** Docker Monitor checks every 60 seconds for orphaned chains referencing IPs not assigned to any running container.

---

## 10. Volume Mounting (S3 FUSE)

```
getVolumesMountPathBinds(volumes []VolumeDTO)
    │
    ▼ For each volume:
    │
    ├─► Compute volume key: "daytona-volume-{volumeId}[-{md5(subpath)[:8]}]"
    ├─► Acquire per-volume mutex (prevents concurrent mounts)
    ├─► Check if already mounted via `mountpoint -q {path}`
    │
    ├─► [If not mounted]:
    │   ├─► os.MkdirAll(path, 0755)
    │   ├─► Build mount-s3 command:
    │   │     mount-s3 {volumeId} {path}
    │   │       --allow-other
    │   │       --allow-delete
    │   │       --allow-overwrite
    │   │       --file-mode 0666
    │   │       --dir-mode 0777
    │   │       [--prefix {subpath}/]
    │   │
    │   │     Environment:
    │   │       AWS_ENDPOINT_URL, AWS_ACCESS_KEY_ID,
    │   │       AWS_SECRET_ACCESS_KEY, AWS_REGION
    │   │
    │   └─► waitForMountReady() — up to 50 polls × 100ms = 5 seconds
    │         ├─► `mountpoint -q {path}` (still mounted?)
    │         ├─► os.Stat(path)           (filesystem responsive?)
    │         └─► os.ReadDir(path)        (fully operational?)
    │
    └─► Append bind: "{mountPath}/:{containerMountPath}/"
```

**Mount paths:**
- Production: `/mnt/daytona-volume-{id}`
- Development: `/tmp/daytona-volume-{id}`
- With subpath: `/mnt/daytona-volume-{id}-{md5(subpath)[:8]}`

**Cleanup:** `CleanupOrphanedVolumeMounts()` is throttled to run at most once per `volumeCleanupIntervalSec` (default 30s, minimum 10s). Scans `/mnt/daytona-volume-*`, cross-references with running container mounts, unmounts and removes orphans.

---

## 11. Backup & Snapshot Operations

### Snapshot Build

```
BuildSnapshot(BuildSnapshotRequestDTO)
  ├─► BuildImage() — Dockerfile + S3 context → Docker image
  ├─► TagImage() — tag with snapshot reference
  └─► [if PushToInternalRegistry]:
        PushImage() → internal registry
```

### Snapshot Pull

```
PullSnapshot(PullSnapshotRequestDTO)
  ├─► PullImage() — from source registry
  ├─► [if DestinationRef]:
  │     TagImage() — apply new reference
  ├─► [if DestinationRegistry]:
  │     PushImage() → destination registry
  └─► [if NewTag]:
        TagImage() — apply new tag
```

### Image Build Details

1. Validates image name format (must contain exactly one colon, not empty tag)
2. Checks if image already exists locally (skips if found)
3. Creates TAR archive context: Dockerfile content + context files from S3 (by SHA256 hash)
4. For each source registry with auth: adds credentials to `AuthConfigs` (special Docker Hub handling: uses `https://index.docker.io/v1/` key)
5. Calls `ImageBuild()` with platform `linux/amd64`, `ForceRemove=true`, `PullParent=true`
6. Streams build output to log file and `logWriter` via `DisplayJSONMessagesStream`

---

## 12. SSH Gateway

See [section 4.6](#46-pkgsshgateway--ssh-forwarding) for detailed architecture.

**Summary:** Listens on port 2220, authenticates via SSH public key, extracts sandbox ID from username, inspects Docker container for IP, dials container at port 22220 with password "sandbox-ssh", then bidirectionally forwards all SSH channels and data.

---

## 13. Metrics & Observability

### Prometheus Metrics

| Metric | Type | Labels | Purpose |
|--------|------|--------|---------|
| `container_operation_duration` | Histogram | `operation` | Tracks duration of create/destroy operations (buckets: 0.1s–300s) |
| `container_operation_count` | Counter | `operation`, `status` | Counts success/failure of container operations |

Exposed at `GET /metrics` (protected endpoint).

### System Metrics (Healthcheck)

Collected by `internal/metrics/collector.go` using sliding window averaging:

| Metric | Source | Unit |
|--------|--------|------|
| CPU Load Average | `/proc/loadavg` or `sysctl` | 1-min average |
| CPU Usage % | `/proc/stat` or `ps` | Percentage |
| Memory Usage % | `/proc/meminfo` or `vm_stat` | Percentage |
| Disk Usage % | `syscall.Statfs` | Percentage |
| Allocated CPU | Sum of container CPU quotas | Cores |
| Allocated Memory | Sum of container memory limits | GiB |
| Allocated Disk | Sum of container storage opts | GiB |
| Snapshot Count | Docker image list (filtered) | Count |
| Started Sandboxes | Containers with state=started | Count |
| Total CPU/Memory/Disk | System capacity | Cores/GiB |

Reported to API via `RunnersAPI.RunnerHealthcheck()` at configurable interval (default 30s).

### OpenTelemetry Tracing

V2 executor extracts W3C trace context from jobs (propagated from API), creates spans for:
- `execute_{jobType}` — full job execution
- `update_job_status` — status reporting back to API

Attributes: `job.id`, `job.type`, `job.status`, `resource.type`, `resource.id`.

### Logging

Three logging systems used simultaneously:
- **logrus** (legacy): Request logging, general operations
- **zerolog**: Internal library logging
- **slog** with `tint` handler: Structured logging with color output (v2 services)

Levels synchronized via `LOG_LEVEL` env var. Optional file output via `LOG_FILE_PATH`.

---

## 14. Configuration Reference

### Required

| Variable | Description |
|----------|-------------|
| `DAYTONA_API_URL` or `SERVER_URL` | API server URL |
| `DAYTONA_RUNNER_TOKEN` or `API_TOKEN` | Bearer token for API authentication |

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `API_PORT` | `8080` | REST API listen port |
| `API_VERSION` | `2` | API version (2 = job queue) |
| `ENABLE_TLS` | `false` | Enable HTTPS |
| `TLS_CERT_FILE` | — | Path to TLS certificate |
| `TLS_KEY_FILE` | — | Path to TLS private key |
| `RUNNER_DOMAIN` | auto-detected | Public domain/IP (auto-detected from default route if not set) |

### Docker

| Variable | Default | Description |
|----------|---------|-------------|
| `CONTAINER_RUNTIME` | — | Container runtime |
| `CONTAINER_NETWORK` | — | Docker network name |
| `RESOURCE_LIMITS_DISABLED` | `false` | Skip CPU/memory limits |
| `USE_SNAPSHOT_ENTRYPOINT` | `false` | Use image entrypoint instead of daemon |

### Timeouts

| Variable | Default | Min | Description |
|----------|---------|-----|-------------|
| `DAEMON_START_TIMEOUT_SEC` | `60` | — | Daemon HTTP readiness timeout |
| `SANDBOX_START_TIMEOUT_SEC` | `30` | — | Container start timeout |
| `BACKUP_TIMEOUT_MIN` | `60` | `1` | Backup operation timeout (minutes) |
| `POLL_TIMEOUT` | `30` | — | Job poll long-poll timeout (seconds) |
| `HEALTHCHECK_TIMEOUT` | `10s` | — | Health check request timeout |
| `HEALTHCHECK_INTERVAL` | `30s` | `10s` | Health check push interval |

### AWS / S3

| Variable | Description |
|----------|-------------|
| `AWS_REGION` | S3 region |
| `AWS_ENDPOINT_URL` | S3-compatible endpoint |
| `AWS_ACCESS_KEY_ID` | Access key |
| `AWS_SECRET_ACCESS_KEY` | Secret key |
| `AWS_DEFAULT_BUCKET` | Default bucket name |

### Intervals & Limits

| Variable | Default | Min | Description |
|----------|---------|-----|-------------|
| `VOLUME_CLEANUP_INTERVAL_SEC` | `30` | `10` | Orphan volume cleanup throttle |
| `POLL_LIMIT` | `10` | `1` (max `100`) | Max jobs per poll |
| `COLLECTOR_WINDOW_SIZE` | `60` | `1` | Metrics sliding window size |
| `CACHE_RETENTION_DAYS` | — | — | State cache retention |

### SSH Gateway

| Variable | Default | Description |
|----------|---------|-------------|
| `SSH_GATEWAY_ENABLE` | `false` | Enable SSH gateway |
| `SSH_GATEWAY_PORT` | `2220` | SSH listen port |
| `SSH_PUBLIC_KEY` | — | Base64-encoded public key for client auth |
| `SSH_HOST_KEY_PATH` | `/root/.ssh/id_rsa` | Host key file (auto-generated if missing) |

### Logging

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `warn` | Log level: debug, info, warn, error |
| `LOG_FILE_PATH` | — | Optional log file path |

### Development

| Variable | Default | Description |
|----------|---------|-------------|
| `ENVIRONMENT` | — | Set to `development` for ephemeral net rules, dev-mode Swagger docs |

---

## 15. Deployment & Packaging

### Debian Package

Located in `packaging/deb/DEBIAN/`:
- `control` — Package metadata (name, version, architecture, description)
- `postinst` — Post-install: enables and starts `daytona-runner` systemd service
- `postrm` — Post-remove: disables and stops service, removes data
- `prerm` — Pre-remove: stops service

### systemd Service

Located in `packaging/systemd/daytona-runner.service`:
- Type: simple
- Restart: always
- Reads environment from `/etc/daytona/runner.env`
- Executes `/usr/local/bin/daytona-runner`

### Docker

`Dockerfile` at project root builds the runner as a container image.

### Build

Version injected at compile time via:
```
go build -ldflags "-X internal.Version=<version>"
```
