# Daytona Directory Structure

Daytona is a secure and elastic infrastructure platform for running AI-generated code. This document provides a comprehensive overview of the project's directory structure and detailed responsibilities of each layer.

## Technology Stack

- **Backend**: Go (microservices: CLI, daemon, proxy, runner, snapshot-manager, SSH gateway)
- **Frontend**: React (Vite) + TypeScript
- **API**: TypeScript/NestJS with PostgreSQL + Redis
- **Documentation**: Astro with Starlight
- **SDKs**: Python, TypeScript
- **Build System**: Nx monorepo orchestration
- **Database**: PostgreSQL + TypeORM
- **Storage**: MinIO (S3-compatible)
- **Auth**: Auth0, OIDC, JWT

---

## Top-Level Directory Structure

```
daytona/
├── apps/                    # Main applications
├── libs/                    # Shared libraries & SDKs
├── examples/                # Usage examples
├── guides/                  # Developer guides
├── docker/                  # Docker & deployment configuration
├── functions/               # Serverless/function configuration
├── scripts/                 # Utility scripts
├── hack/                    # Development & build helpers
├── assets/                  # Project assets (logos, branding)
└── images/                  # Build & release images
```

---

## `/apps` - Main Applications

Contains all production applications as separate modules.

### `apps/api` (TypeScript/NestJS)

Main REST API server for Daytona.

```
apps/api/
├── src/
│   ├── admin/               # Admin module
│   ├── auth/                # Authentication
│   ├── sandbox/             # Sandbox orchestration
│   ├── docker-registry/     # Docker registry integration
│   ├── organization/        # Organization management
│   ├── region/              # Region configuration
│   ├── audit/               # Audit logging
│   ├── email/               # Email service
│   ├── notification/        # Notifications
│   ├── object-storage/      # Object storage integration
│   └── ...                  # 20+ modules
├── project.json
└── tsconfig.json
```

**Purpose**: Handles core business logic, user management, sandbox orchestration, and webhooks. Uses TypeORM for database ORM.

#### Detailed Responsibilities

**Authentication & Authorization (`auth/`)**:
- JWT strategy implementing OIDC with support for multiple providers (Auth0, Okta)
- Auto-creates users on first login from JWT claims
- API key strategy supporting user keys, runner keys, region proxy keys, and SSH gateway keys with Redis-backed validation caching (10s TTL)
- `CombinedAuthGuard` tries JWT first, then falls back to API key
- Rate limiting at three levels: anonymous (IP-based), failed-auth (IP blocking), and authenticated (organization-based with per-org overrides)

**Sandbox Module (`sandbox/`)**:
- `SandboxService` (1807 lines): Full lifecycle management including create, start, stop, destroy, resize, backup, restore, auto-stop/archive/delete, and quota enforcement
- State machine with states: CREATING, RESTORING, STARTED, STOPPED, DESTROYING, DESTROYED, ERROR, BUILD_FAILED, PENDING_BUILD, BUILDING_SNAPSHOT, PULLING_SNAPSHOT, ARCHIVED, ARCHIVING, and desired states: STARTED, STOPPED, DESTROYED, ARCHIVED, RESIZED
- Runner selection via weighted scoring algorithm (CPU 25%, memory 40%, disk 40%, plus allocated resource and started sandbox penalties)
- Warm pool management: pre-created sandboxes in STOPPED state keyed by organization + snapshot for instant provisioning
- Runner adapter pattern with V0 (legacy HTTP REST) and V2 (state-based polling) communication modes
- `BackupManager`: creates snapshot of running sandbox, stores in backup registry, supports multiple backups per sandbox
- Network configuration: global allow-list, per-sandbox blocking, per-organization egress limiting with CIDR validation

**Organization Module (`organization/`)**:
- Multi-tenant container with quotas, suspension support, and rate limit overrides
- Custom roles with granular permissions (registries, snapshots, sandboxes, volumes, regions, runners, audit)
- Email-based invitations with role assignment
- Per-region quota enforcement
- Usage tracking for resource consumption

**Webhook Module (`webhook/`)**:
- Svix integration for reliable webhook delivery with automatic retries
- Auto-triggered events: `sandbox.created`, `sandbox.state.updated`, `snapshot.created`, `snapshot.state.updated`, `snapshot.removed`, `volume.created`, `volume.state.updated`
- Portal access for webhook management UI

**Audit Module (`audit/`)**:
- Action logging via interceptor capturing actor, action, target, status code, IP, and user agent
- Multi-backend storage: PostgreSQL (default), OpenSearch (full-text search), Kafka (event streaming)
- Configurable retention policies

**Docker Registry Module (`docker-registry/`)**:
- Three registry types: TRANSIENT (build cache), INTERNAL (snapshot storage), BACKUP (disaster recovery)
- Encrypted credential storage with availability tracking

**Region Module (`region/`)**:
- Multi-region support with runner registration per region
- Snapshot availability tracking across regions
- Proxy and SSH gateway configuration per region

**Notification Module (`notification/`)**:
- WebSocket-based real-time updates for sandbox state changes, snapshot operations, and job completions

**Configuration (`config/`)**:
- `TypedConfigService` providing strongly-typed access to database, Redis, OIDC, SMTP, S3, Kafka, OpenSearch, Docker registries, runner scoring weights, rate limits, and quotas

**Startup Initialization**:
- Creates default region, admin user with system API key, transient/internal/backup Docker registries, default runner, and default snapshot

---

### `apps/cli` (Go)

Command-line interface for Daytona.

```
apps/cli/
├── cmd/                     # Command definitions
│   ├── auth/                # Login/logout commands
│   ├── sandbox/             # Sandbox operations
│   ├── snapshot/            # Snapshot management
│   ├── volume/              # Volume management
│   ├── organization/        # Organization management
│   ├── mcp/                 # MCP server integration
│   │   └── agents/          # Agent-specific setup (Claude, Cursor, Windsurf)
│   └── common/              # Shared command utilities
├── apiclient/               # API client wrapper
├── auth/                    # Authentication handling
├── config/                  # Configuration management
├── docker/                  # Docker integration
├── mcp/                     # MCP server and tools
│   └── tools/               # 12 MCP tool implementations
├── views/                   # Terminal UI views
│   └── common/              # Shared styles (lipgloss)
├── internal/                # Internal packages
├── pkg/minio/               # MinIO client for build contexts
└── util/                    # Utilities
```

**Purpose**: Provides CLI commands for auth, organization, sandbox, snapshot, and volume management. Uses Cobra CLI framework with MCP (Model Context Protocol) support.

#### Detailed Responsibilities

**Command Groups**:
- **User commands**: `login` (OAuth browser flow or API key), `logout`, `organization create/list/use/delete`
- **Sandbox commands** (top-level shortcuts): `create`, `list`, `info`, `delete`, `start`, `stop`, `archive`, `ssh`, `exec`, `preview-url`
- **Snapshot commands**: `snapshot create` (from Dockerfile or image), `snapshot list`, `snapshot push`, `snapshot delete`
- **Volume commands**: `volume create/list/get/delete`
- **MCP commands**: `mcp start` (stdio server), `mcp config` (output JSON config), `mcp init [claude|cursor|windsurf]`

**Authentication (`auth/`)**:
- OAuth 2.0 browser-based flow via Auth0: starts local callback server, opens browser, receives auth code, exchanges for tokens (access, refresh, ID)
- API key authentication as simpler alternative
- Automatic token refresh when within 5 minutes of expiry via `RefreshTokenIfNeeded()`

**Configuration (`config/`)**:
- JSON config at `~/.config/daytona/config.json` with multiple profile support
- Each profile stores API endpoint, credentials (API key or OAuth token), active organization ID, and regional toolbox proxy URL cache
- Environment variable overrides: `DAYTONA_API_URL`, `DAYTONA_API_KEY`, `DAYTONA_CONFIG_DIR`

**API Client (`apiclient/`)**:
- Wraps generated Go client from `libs/api-client-go`
- Custom `versionCheckTransport` compares CLI vs API versions via `X-Daytona-Api-Version` header and shows one-time warning if CLI is behind
- Adds `X-Daytona-Source: cli` and `X-Daytona-Organization-ID` headers

**MCP Server (`mcp/`)**:
- 12 tools: `create_sandbox`, `destroy_sandbox`, `file_upload`, `file_download`, `file_info`, `list_files`, `move_file`, `delete_file`, `create_folder`, `execute_command`, `git_clone`, `preview_link`
- Communicates via stdio, uses `X-Daytona-Source: daytona-mcp` header
- Retry with exponential backoff on quota errors

**Build Context Management (`pkg/minio/`)**:
- Uploads Dockerfile build contexts to MinIO as tar archives
- SHA256 hashing for deduplication (skips re-upload if hash exists)
- Parses Dockerfile `COPY`/`ADD` instructions to auto-detect context files
- Respects `.dockerignore` patterns

**Terminal UI (`views/`)**:
- Interactive mode with charmbracelet/lipgloss styling, charmbracelet/huh forms, and colored output
- Structured output via `-f json` or `-f yaml` flags with stdout blocking to prevent TUI noise
- Responsive table rendering with terminal width detection

---

### `apps/daemon` (Go)

In-container sandbox runtime manager running inside each sandbox.

```
apps/daemon/
├── cmd/daemon/              # Entry point & config
├── internal/util/           # Shared utilities
├── pkg/
│   ├── toolbox/             # Main API server (port 2280)
│   │   ├── process/         # Session/PTY/Interpreter managers
│   │   ├── fs/              # File operations (14 handlers)
│   │   ├── git/             # Git wrappers
│   │   ├── lsp/             # Language server support
│   │   ├── computeruse/     # Desktop automation plugin
│   │   ├── port/            # Port detection
│   │   └── proxy/           # Request routing to localhost
│   ├── ssh/                 # SSH server (port 22220)
│   ├── terminal/            # WebSocket terminal (port 22222)
│   ├── git/                 # Git service layer
│   ├── gitprovider/         # Git provider abstractions
│   └── common/              # Shell, TTY, error utilities
└── tools/                   # Build-time tools (xterm.js downloader)
```

**Purpose**: Multi-service sandbox runtime that provides HTTP REST API, SSH, and WebSocket terminal access inside each sandbox container.

#### Detailed Responsibilities

**Three Parallel Services** launched simultaneously on container startup:

1. **Toolbox HTTP Server (port 2280)** - Primary API for sandbox interaction via Gin framework:
   - **File operations** (11 endpoints): upload (single/bulk), download (single/bulk as ZIP), list, search, find-in-files, move, delete, create folder, set permissions, replace-in-files
   - **Process management**: Three execution models:
     - **Sessions**: Persistent interactive shell with command tracking, per-session file storage at `~/.daytona/sessions/`, graceful termination (SIGTERM 5s then SIGKILL to process tree)
     - **PTY**: Multi-attach pseudo-terminals with shared output via WebSocket, multiple simultaneous clients, LazyStart mode, resize support
     - **Interpreter**: REPL-style Python/Node execution with persistent context and WebSocket output streaming
     - **Simple execute**: One-shot command execution with timeout (default 360s)
   - **Git operations** (12 endpoints): clone, add, commit, push, pull, branch CRUD, status, history, upstream tracking with multi-provider auth (GitHub, GitLab, Gitea, Bitbucket)
   - **LSP** (7 endpoints): Start/stop language servers (Python via Pylance/pyright, TypeScript via tsserver), code completions, document/workspace symbols, file open/close notifications
   - **Computer use** (20+ methods): Desktop automation via HashiCorp go-plugin RPC system — screenshots (full/region/compressed), mouse control (click/drag/scroll), keyboard input (type/press/hotkey), display info, window enumeration. Plugin at `/usr/local/lib/daytona-computer-use` with graceful degradation
   - **Port detection**: Background scanner every 1 second tracking listening TCP sockets, exposes API for port availability checks (range 3000-9999)
   - **Proxy**: Routes requests to `localhost:{port}` for user-space services

2. **SSH Server (port 22220)**: Public key auth (accepts all) and password auth (hardcoded `sandbox-ssh`), PTY support, SFTP subsystem, TCP and Unix socket port forwarding

3. **WebSocket Terminal Server (port 22222)**: xterm.js-based web terminal with UTF-8 decoder, resize messages, embedded static files

**Entrypoint Management**: Executes initialization scripts in background, doesn't block daemon startup. Graceful shutdown: SIGTERM first (configurable timeout, default 10s), then SIGKILL (default 5s).

---

### `apps/dashboard` (React/Vite)

Web UI for Daytona.

```
apps/dashboard/
├── src/
│   ├── pages/               # 20+ page components
│   ├── components/          # Reusable UI components
│   │   ├── ui/              # shadcn/ui component library
│   │   ├── SandboxTable/    # Complex sandbox table with filters
│   │   ├── Organizations/   # Organization management dialogs
│   │   └── OrganizationMembers/  # Member management
│   ├── hooks/               # Custom React hooks (queries, mutations)
│   ├── contexts/            # React contexts (API, org, config, theme, notifications)
│   ├── providers/           # Context providers (auth, query, theme)
│   ├── api/                 # API client & error handling
│   ├── billing-api/         # Separate billing API client
│   ├── lib/                 # Utility functions
│   ├── enums/               # Route paths, feature flags, localStorage keys
│   └── mocks/               # MSW mocks for testing
├── public/                  # Static assets
├── vite.config.ts
└── tailwind.config.js
```

**Purpose**: Web dashboard for managing sandboxes, organizations, and settings. Uses React Router, TanStack Query, and Tailwind CSS.

#### Detailed Responsibilities

**Pages & Features**:
- **Sandboxes**: Paginated list with advanced filtering (state, resources, labels, time range), sorting, bulk actions (start/stop/delete/archive), SSH access token creation/revocation, VNC desktop integration, real-time state updates via WebSocket with optimistic UI updates
- **Snapshots**: Create from Docker images with CPU/memory/disk/region configuration, delete, paginated list
- **Volumes**: Volume management (requires READ_VOLUMES permission)
- **API Keys**: Create with custom permissions and expiration, role-based permission inheritance, revoke
- **Organization Settings**: Update details, set default region, delete/leave organization
- **Organization Members**: Invite via email, manage roles, view pending invitations, remove members
- **Billing** (owner-only): Wallet balance, automatic top-up, coupon redemption, billing email management, spending breakdown by metric (RAM/CPU/Disk) over time, tier upgrade/downgrade via Stripe
- **Regions/Runners**: Custom infrastructure management (feature-flagged), API key regeneration
- **Audit Logs**: Date range filtering, cursor-based pagination, auto-refresh toggle
- **Account Settings**: Link/unlink OAuth providers (GitHub), SMS MFA enrollment
- **Onboarding**: First-time setup guide

**Authentication**: OIDC via `react-oidc-context` with Auth0 provider, automatic token refresh, protected routes with role/permission guards

**State Management**:
- TanStack React Query for server state with query key hierarchy
- React Context for client state: `ApiContext` (API client), `SelectedOrganizationContext` (current org with permission checking), `OrganizationsContext` (org list), `NotificationSocketContext` (socket.io WebSocket), `ConfigContext` (dashboard config from `/api/config`), `ThemeContext` (light/dark)

**Real-time Updates**: Socket.io WebSocket connection receiving sandbox state changes, runner events — integrated with React Query cache invalidation

**Styling**: Tailwind CSS with shadcn/ui (Radix UI primitives), CSS custom properties for light/dark themes, responsive design with mobile-first approach, lucide-react icons

---

### `apps/docs` (Astro)

Static documentation site.

```
apps/docs/
├── src/
│   ├── content/             # Documentation content
│   ├── components/          # Custom components
│   ├── pages/               # Page templates
│   └── i18n/                # Internationalization
├── public/
└── astro.config.mjs
```

**Purpose**: Starlight-based documentation site with multi-language support and Scalar API documentation integration.

---

### `apps/proxy` (Go)

Reverse HTTP proxy for sandbox access.

```
apps/proxy/
├── cmd/proxy/               # Entry point & config
│   └── config/              # Environment-based configuration
├── internal/                # Build info
└── pkg/proxy/               # Core proxy logic
    ├── proxy.go             # HTTP server, routing, middleware
    ├── auth.go              # Multi-method authentication
    ├── auth_callback.go     # OIDC OAuth2 flow
    ├── get_sandbox_target.go    # Sandbox routing
    ├── get_snapshot_target.go   # Build log proxying
    ├── get_sandbox_build_target.go  # Sandbox build logs
    └── warning_page.go      # Preview warning page
```

**Purpose**: Sits between users and sandbox instances, handling authentication, request routing, and activity tracking.

#### Detailed Responsibilities

**Request Routing**:
- Parses host headers in format `<port>-<sandboxId>.proxy.domain` (e.g., `3000-abc123.proxy.example.com`)
- Routes: `/callback` (OIDC), `/health` (version), `/snapshots/{id}/build-logs`, `/sandboxes/{id}/build-logs`, `/toolbox/{sandboxId}/*` (port 2280), `<port>-<id>.*` (sandbox port proxy)
- Reverse proxies to runner at `{runnerApiUrl}/sandboxes/{sandboxId}/toolbox/proxy/{port}{path}`

**Multi-layered Authentication** (tried in order):
1. Bearer token (Authorization header) — validated via API's `HasSandboxAccess`
2. Auth key header (`X-Daytona-Preview-Token`) — validated via `IsValidAuthToken`
3. Auth key query parameter (`DAYTONA_SANDBOX_AUTH_KEY`) — auto-removed from proxied request
4. Secure cookie (`daytona-sandbox-auth-<sandboxId>`) — encrypted with `PROXY_API_KEY`, 1-hour expiry
5. Signed preview URL token — creates cookie on success, falls back to OIDC redirect if expired
- Public sandboxes skip authentication; terminal (22222) and toolbox (2280) ports always require auth

**OIDC Flow**: OAuth2 with PKCE (S256 challenge), state stored in base64-encoded JSON containing random state, return URL, and sandbox ID. Supports internal/private OIDC domains with public domain override.

**Caching**: Dual-mode (Redis or in-memory MapCache) with TTLs — runner info 2 min, sandbox public status 1 hour, auth key validation 2 min, activity update throttle 45s

**Activity Tracking**: Polls `UpdateLastActivity` every 50 seconds to keep sandboxes alive during active connections. Stops when connection closes.

**Middleware Stack**: Connection monitoring (WebSocket hijacking) → error recovery → error handling → CORS (dynamic, all origins) → browser warning page (optional) → main router

**Preview Warning Page**: Detects browser user agents, shows HTML warning with "I Understand, Continue" button, sets 1-day acceptance cookie, skips for non-browser clients and WebSocket requests

---

### `apps/runner` (Go)

Distributed container orchestration service managing sandbox execution.

```
apps/runner/
├── cmd/runner/              # Entry point & config
├── internal/
│   ├── metrics/             # System & container metrics collector
│   ├── constants/           # Auth constants, context keys
│   └── util/                # Logging utilities
├── pkg/
│   ├── docker/              # Docker API wrapper (25+ operations)
│   ├── api/                 # REST API server & controllers
│   │   ├── controllers/     # Sandbox, snapshot, proxy controllers
│   │   ├── dto/             # Data transfer objects
│   │   └── middlewares/     # Auth, logging, error handling
│   ├── runner/              # Singleton runner instance
│   │   └── v2/              # V2 job execution system
│   │       ├── poller/      # Long-polling job fetcher
│   │       ├── executor/    # Async job executor
│   │       └── healthcheck/ # Periodic health reporting
│   ├── services/            # Sandbox service, sync service
│   ├── netrules/            # iptables firewall management
│   ├── cache/               # In-memory state cache
│   ├── storage/             # MinIO/S3 client for backups
│   ├── sshgateway/          # Optional SSH gateway (port 2220)
│   ├── daemon/              # Embedded daemon binary management
│   ├── common/              # Prometheus metrics, utilities
│   └── models/              # State enums, cached states
└── packaging/               # Systemd service & Debian package
```

**Purpose**: Manages Docker containers as isolated sandboxes, executes jobs from the API, enforces network rules, and collects resource metrics.

#### Detailed Responsibilities

**Docker Container Management (`pkg/docker/`)**:
- **Creation pipeline**: Deduce existing state → pull image from registry → validate amd64/x86_64 architecture → get volume mount binds → build container config (resource limits, env vars, ports) → create container → start with daemon initialization → configure networking
- **Operations**: create, start, stop, destroy, resize (CPU/memory adjustment), backup (commit + push), recover (restore from backup), snapshot pull/build/push/remove/tag/inspect
- **State management**: Deduces sandbox state from Docker container status + cache, states include creating, restoring, started, stopped, destroying, destroyed, resizing, error, pulling_snapshot

**V2 Job Execution System (`pkg/runner/v2/`)**:
- **Poller**: Long-polls API for pending jobs with configurable timeout (default 30s) and limit (default 10). Handles 408 timeouts gracefully, recovers in-progress jobs on startup
- **Executor**: Dispatches jobs by type to handlers — `CREATE_SANDBOX`, `START_SANDBOX`, `STOP_SANDBOX`, `DESTROY_SANDBOX`, `CREATE_BACKUP`, `BUILD_SNAPSHOT`, `PULL_SNAPSHOT`, `REMOVE_SNAPSHOT`, `UPDATE_SANDBOX_NETWORK_SETTINGS`, `INSPECT_SNAPSHOT_IN_REGISTRY`, `RECOVER_SANDBOX`. Reports results and errors via API with OpenTelemetry tracing
- **Healthcheck**: Reports runner health and metrics to API every 30 seconds

**Network Rules (`pkg/netrules/`)**:
- Thread-safe iptables rule management using DOCKER-USER chain and DAYTONA-SB-* chains
- Supports: block all traffic, CIDR allow lists, egress bandwidth limiting
- Background reconciliation goroutine for persistent mode with save to `/etc/iptables/rules.v4`

**Metrics Collection (`internal/metrics/`)**:
- Sliding-window CPU measurement with ring buffer
- Tracks CPU load/usage, memory usage, disk usage, allocated resources, snapshot count, started sandbox count
- Prometheus histogram (container_operation_duration_seconds) and counter (container_operation_total)

**State Synchronization (`pkg/services/sandbox_sync.go`)**:
- Background sync every 10 seconds comparing local Docker container states with remote API expected states
- Prevents state drift between runner and API server

**REST API (`pkg/api/`)**:
- Bearer token auth, endpoints for sandbox CRUD + lifecycle, toolbox proxy (forwards to container port 2280), snapshot management, Prometheus metrics
- Toolbox proxy dynamically resolves container IP at request time

**SSH Gateway (`pkg/sshgateway/`)**: Optional SSH access on port 2220 with public key authentication, routes by sandbox ID

**Packaging**: Systemd service unit (runs as root for Docker socket), Debian package with postinst/prerm/postrm scripts

---

### `apps/snapshot-manager` (Go)

OCI-compliant container image registry for sandbox snapshots.

```
apps/snapshot-manager/
├── cmd/main.go              # Entry point with graceful shutdown
└── internal/
    ├── config/              # Environment-based configuration
    ├── logger/              # slog with tint formatting, logrus bridge
    └── server/
        ├── server.go        # HTTP server & registry lifecycle
        └── config.go        # Builder pattern for distribution config
```

**Purpose**: Docker-compatible container image registry built on the OCI Distribution spec (distribution/distribution v3) that stores sandbox snapshots for fast provisioning.

#### Detailed Responsibilities

**Storage Backends**:
- **Filesystem**: Local disk storage at configurable directory (auto-created with 0755 permissions), suitable for development
- **S3/MinIO**: Scalable cloud storage with configurable region, bucket, access keys, custom endpoint (for MinIO), server-side encryption, HTTPS toggle, and root directory prefix

**Authentication**: HTTP Basic Auth via bcrypt-hashed htpasswd file generated on startup, or no authentication. Supports shared HTTP secret for multi-instance deployments behind load balancers.

**OCI Registry Endpoints**: Full V2 spec — blob uploads (POST/PATCH/PUT), manifest GET/PUT/DELETE, catalog listing, tag listing, plus `/healthz` health check

**Caching**: Optional in-memory blob descriptor cache to reduce storage backend reads

**Integration with Runners**: Runners push built images and pull snapshot images using standard Docker registry protocol. API manages credentials per region, supporting credential rotation via `POST /regions/:id/regenerate-snapshot-manager-credentials`.

**Build**: Multi-stage Docker build producing minimal Alpine image with statically linked binary (CGO_ENABLED=0). Version injected via ldflags.

---

### `apps/ssh-gateway` (Go)

SSH tunnel service providing external SSH access to sandboxes.

```
apps/ssh-gateway/
└── main.go                  # Single-file application (~470 lines)
```

**Purpose**: Stateless SSH forwarding proxy that authenticates users via tokens and tunnels connections through the runner to sandbox containers.

#### Detailed Responsibilities

**Token-Based Authentication**:
- Extracts token from SSH username field (no password or public key auth)
- Validates token via API: `POST /sandbox/ssh-access/validate?token={token}` returning `{valid, sandboxId}`
- Rejects connection if token invalid or sandbox not in STARTED state

**Connection Routing** (three-stage):
1. Token → sandbox ID via API validation
2. Sandbox ID → runner domain via `GET /runners/{sandboxId}`
3. Verify sandbox state is STARTED via `GET /sandbox/{sandboxId}`

**SSH Forwarding**:
- Accepts connections on port 2222 (configurable via `SSH_GATEWAY_PORT`)
- Opens outbound SSH connection to runner's SSH gateway on port 2220 using sandbox ID as username and private key for authentication
- Bidirectional channel and data forwarding between client and runner via 4 concurrent goroutines per channel
- Supports SSH subsystems, PTY requests, and shell sessions

**Keep-Alive**: Calls `UpdateLastActivity(sandboxId)` every 45 seconds to prevent sandbox auto-stop during active SSH sessions

**Port Chain**: Client → SSH Gateway (2222) → Runner SSH Gateway (2220) → Daemon SSH Server (22220)

**Configuration**: Environment variables for `SSH_GATEWAY_PORT` (default 2222), `API_URL` (default http://localhost:3000), `API_KEY` (required), `SSH_PRIVATE_KEY` and `SSH_HOST_KEY` (both base64-encoded, required). Supports OpenSSH and PKCS1 key formats.

---

## `/libs` - Shared Libraries & SDKs

### API Clients (Auto-generated from OpenAPI)

All API clients are auto-generated from OpenAPI specifications using OpenAPI Generator. Regeneration preserves files listed in `.openapi-generator-ignore`.

| Directory | Language | Purpose |
|-----------|----------|---------|
| `api-client-go/` | Go | Go API client (267+ files) |
| `api-client-python/` | Python | Python sync API client |
| `api-client-python-async/` | Python | Python async API client |
| `api-client/` | TypeScript | TypeScript/JavaScript API client |
| `runner-api-client/` | TypeScript | Runner-specific API client |
| `toolbox-api-client/` | TypeScript | Toolbox API client |
| `toolbox-api-client-python/` | Python | Toolbox Python sync client |
| `toolbox-api-client-python-async/` | Python | Toolbox Python async client |

### SDKs (Main User-Facing Libraries)

#### `libs/sdk-python/`

Primary Python SDK with async-first design and auto-generated sync wrappers.

```
libs/sdk-python/
└── src/daytona_sdk/
    ├── _async/              # Core async implementations
    ├── _sync/               # Auto-generated sync wrappers (via sync_generator.py)
    ├── Daytona              # Main client class
    ├── Sandbox              # Sandbox management
    ├── Process              # Process execution
    ├── FileSystem           # File operations
    ├── Git                  # Git operations
    ├── LspServer            # LSP integration
    ├── ObjectStorage        # Object storage
    ├── Volume               # Volume management
    ├── Image                # Image handling
    ├── CodeInterpreter      # Stateful Python code execution via WebSocket
    ├── ComputerUse          # Desktop automation (mouse, keyboard, screenshot)
    ├── Snapshot             # Snapshot management
    └── PtyHandle            # PTY handling
```

**Detailed Capabilities**:
- **Daytona client**: Manages API authentication (API key or JWT), configuration from environment variables, sandbox CRUD (`create`, `delete`, `get`, `find_one`, `list`, `start`, `stop`), and service initialization (volume, snapshot)
- **Sandbox**: Provides access to `fs`, `git`, `process`, `computer_use`, `code_interpreter`, `lsp_server` sub-objects. Methods: `wait_for_sandbox_start()`, `start()`, `stop()`, `delete()`, `restart()`
- **Process**: `exec()` for shell commands, `code_run()` for language-specific execution with artifact parsing (charts, structured output), `create_session()` for persistent shells, `connect_pty()` for interactive terminals
- **CodeInterpreter**: Stateful Python execution via WebSocket with streaming callbacks (`on_stdout`, `on_stderr`, `on_error`), persistent contexts, returns `ExecutionResult` with stdout/stderr/error
- **FileSystem**: Upload (multipart for large files), download (memory or streaming), create folder, delete, find, read, write, replace (regex patterns)
- **Git**: Clone, add, commit, push, pull, branches, checkout, status, delete_branch
- **LSP**: Start/stop language servers, code completions, document symbols, definition, hover — supports Python and TypeScript/JavaScript
- **ComputerUse**: Mouse (position, move, click, drag, scroll), keyboard (type, press, hotkey), screenshots (full/region), display info, window enumeration
- **Code Toolboxes**: Language-specific execution adapters for Python, TypeScript, JavaScript — generate shell commands from code with parameter support
- **Error handling**: `DaytonaError`, `DaytonaNotFoundError`, `DaytonaRateLimitError`, `DaytonaTimeoutError` with `@intercept_errors()` decorator for API error transformation

#### `libs/sdk-typescript/`

Primary TypeScript SDK (mirrors Python SDK structure with fully async/await design).

```
libs/sdk-typescript/
└── src/
    ├── Daytona.ts           # Main client class
    ├── Sandbox.ts           # Sandbox management
    ├── Process.ts           # Process execution
    ├── FileSystem.ts        # File operations
    ├── Git.ts               # Git operations
    ├── LspServer.ts         # LSP integration
    ├── CodeInterpreter.ts   # Stateful Python execution
    ├── ComputerUse.ts       # Desktop automation
    └── ...
```

Same capabilities as Python SDK: sandbox CRUD, process execution with artifact parsing, code interpreter via WebSocket, file operations, git operations, LSP, computer use. Uses Axios for HTTP and generated API clients from `@daytonaio/api-client` and `@daytonaio/toolbox-api-client`.

### Shared Libraries

| Directory | Purpose |
|-----------|---------|
| `common-go/` | Go common utilities: reverse proxy handler (`pkg/proxy/`), generic caching interface with MapCache and RedisCache implementations (`pkg/cache/`), HTTP error middleware (`pkg/errors/`), exponential backoff retry (`pkg/utils/`), connection monitoring with WebSocket hijacking support |
| `computer-use/` | VNC desktop environment management via 4 priority-ordered processes: Xvfb (virtual display), xfce4 (desktop), x11vnc (VNC server on port 5901), noVNC (web VNC on port 6080). Auto-restart on failure, configurable resolution (default 1920x1080), runs as daytona user |

---

## `/examples` - Usage Examples

Demonstrates SDK usage with real code examples.

```
examples/
├── python/                  # Python examples
│   ├── auto-archive/
│   ├── auto-delete/
│   ├── charts/
│   ├── file-operations/
│   ├── git-lsp/
│   ├── lifecycle/
│   ├── network-settings/
│   ├── pagination/
│   ├── pty/
│   ├── region/
│   ├── volumes/
│   └── ...                  # 15+ examples
├── jupyter/                 # Jupyter notebook examples
└── typescript/              # TypeScript examples (mirrors Python)
```

---

## `/guides` - Developer Guides

```
guides/
├── python/                  # Python-specific guides
└── typescript/              # TypeScript-specific guides
```

**Purpose**: Educational and technical guides for developers.

---

## `/docker` - Docker & Deployment Configuration

```
docker/
├── docker-compose.yaml      # Complete local development stack
├── docker-compose.build.override.yaml  # Source build override
├── dex/                     # Dex OIDC configuration
├── otel/                    # OpenTelemetry configuration
├── pgadmin4/                # PgAdmin4 configuration
└── README.md                # Docker setup guide
```

**Services included in docker-compose:**
- PostgreSQL 18 (database, port 5432 internal, volume: `db_data`)
- Redis latest (cache, port 6379 internal)
- Dex v2.42.0 (OIDC provider, port 5556, SQLite storage, default user: `dev@daytona.io`/`password`)
- Docker Registry v2.8.2 (port 6000, delete enabled, volume: `registry`)
- Registry UI (port 5100, Joxit web UI)
- MinIO latest (S3-compatible storage, console port 9001, credentials: `minioadmin`/`minioadmin`, volume: `minio_data`)
- MailDev (email testing, port 1080)
- Jaeger v1.67.0 (distributed tracing, UI port 16686)
- OpenTelemetry Collector v0.138.0 (OTLP receivers on ports 4317/4318, exports traces to Jaeger, metrics to Prometheus)
- PgAdmin4 v9.2.0 (database UI, port 5050, credentials: `dev@daytona.io`/`pgadmin`)
- API (port 3000, privileged, with OpenTelemetry)
- Proxy (port 4000)
- Runner (port 3003, privileged)
- SSH Gateway (port 2222)

All services connected via `daytona-network` bridge network.

**Build Override**: `docker-compose.build.override.yaml` enables building from source instead of pre-built images for API, Proxy, Runner, and SSH Gateway.

---

## `/functions` - Serverless Configuration

```
functions/
└── auth0/                   # Auth0 authentication integration
    ├── validateEmailUnused.onExecutePostLogin.js   # Email uniqueness check on login
    ├── setCustomClaims.onExecutePostLogin.js       # JWT enrichment (email, name, phone_verified, identities)
    ├── validateEmailUnused.onExecutePreRegister.js # Prevent email reuse on registration
    └── verifyAliasEmail.onExecutePreRegister.js    # Block email aliases (containing +)
```

---

## `/scripts` - Utility Scripts

```
scripts/
├── setup-proxy-dns.sh       # DNS configuration for wildcard proxy domains
│                            # macOS: dnsmasq + /etc/resolver/ for *.proxy.localhost
│                            # Linux: dnsmasq + /etc/resolv.conf
├── computer-use/            # Computer use related scripts
└── python-client/           # Python client related scripts
```

---

## `/hack` - Development & Build Helpers

```
hack/
├── computer-use/
│   └── build-computer-use-amd64.sh  # Build computer-use binary (native or Docker cross-compile)
└── python-client/
    └── postprocess.sh       # Post-process generated Python OpenAPI client
                             # (license fix, urllib3 pin, Pydantic V2 Field alias fix)
```

---

## `/assets` - Project Assets

```
assets/
└── images/                  # Logo, branding assets
```

---

## Root Configuration Files

### Build & Project Management

| File | Purpose |
|------|---------|
| `go.work` | Go workspace (Go 1.25.4) linking 6 app modules + 3 lib modules with local api-client-go replacement |
| `go.work.sum` | Go workspace checksums |
| `nx.json` | Nx monorepo: plugins for webpack/eslint/jest/vite/react-router, Docker build targets with multi-arch support (amd64/arm64), conventional commits release config |
| `project.json` | Root Nx project configuration |
| `package.json` | Node.js workspace (yarn 4.6.0) with scripts for format, lint, build, serve, generate:openapi, generate:api-client, migration management, Docker builds |

### Python

| File | Purpose |
|------|---------|
| `pyproject.toml` | Poetry configuration for Python dependencies |
| `poetry.lock` | Poetry lock file |

### Code Quality & Linting

| File | Purpose |
|------|---------|
| `.golangci.yaml` | Go linter configuration |
| `eslint.config.mjs` | ESLint configuration |
| `.prettierrc` | Prettier formatting rules |
| `.markdownlint-cli2.jsonc` | Markdown linting |

### CI/CD & Version Control

| File/Directory | Purpose |
|----------------|---------|
| `.github/workflows/` | GitHub Actions workflows |
| `.github/ISSUE_TEMPLATE/` | GitHub issue templates |
| `openapitools.json` | OpenAPI code generator configuration |
| `ecosystem.config.js` | PM2 ecosystem configuration |

### Development & Git Hooks

| File/Directory | Purpose |
|----------------|---------|
| `.husky/` | Git hooks — lint-staged runs: ESLint+Prettier (TS), gofmt (Go), markdownlint (MD), isort+black+pylint+basedpyright (Python) |
| `.gitignore` | Git ignore rules |
| `.editorconfig` | Editor configuration |

### TypeScript & Testing

| File | Purpose |
|------|---------|
| `tsconfig.base.json` | Base TypeScript configuration |
| `jest.config.ts` | Jest testing configuration |
| `jest.preset.js` | Jest presets |

### Documentation & License

| File | Purpose |
|------|---------|
| `README.md` | Project overview and quick start |
| `CONTRIBUTING.md` | Contribution guidelines |
| `LICENSE` | AGPL-3 license |
| `CODE_OF_CONDUCT.md` | Community code of conduct |
| `SECURITY.md` | Security policy |

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Dashboard (React)                        │
│  Web UI for sandbox/org/billing management, real-time updates    │
└─────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                         API (NestJS)                             │
│  Central orchestrator: auth, sandbox lifecycle, org management,  │
│  webhooks, audit, billing, job dispatch, runner scoring          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │
│  │   Auth   │ │ Sandbox  │ │   Org    │ │  Docker Registry │   │
│  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
         │              │              │              │
         ▼              ▼              ▼              ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌─────────────┐
│    Proxy     │ │    Runner    │ │ SSH Gateway  │ │   Daemon    │
│     (Go)     │ │     (Go)     │ │     (Go)     │ │    (Go)     │
│ Auth + route │ │ Docker mgmt  │ │ Token-based  │ │ In-container│
│ to sandboxes │ │ job execution│ │ SSH tunneling│ │ runtime     │
└──────────────┘ └──────────────┘ └──────────────┘ └─────────────┘
                        │
                        ▼
              ┌──────────────────┐
              │ Snapshot Manager │
              │ OCI registry for │
              │ container images │
              └──────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                CLI (Go) + MCP Server                              │
│  Command-line interface with OAuth/API key auth, MCP tools       │
│  for AI agent integration (Claude, Cursor, Windsurf)             │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                      SDKs & API Clients                          │
│  ┌────────────┐  ┌──────────────────┐  ┌─────────────────────┐  │
│  │ Python SDK │  │ TypeScript SDK   │  │  Go API Client      │  │
│  │ async-first│  │ async/await      │  │  auto-generated     │  │
│  └────────────┘  └──────────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                        Infrastructure                            │
│  ┌────────────┐  ┌───────┐  ┌───────┐  ┌────────┐  ┌─────────┐ │
│  │ PostgreSQL │  │ Redis │  │ MinIO │  │  Dex   │  │ Jaeger  │ │
│  │ Primary DB │  │ Cache │  │  S3   │  │ OIDC   │  │ Tracing │ │
│  └────────────┘  └───────┘  └───────┘  └────────┘  └─────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### Layer Communication Flow

```
User → Dashboard/CLI/SDK
  → API (NestJS) [auth, business logic, job creation]
    → Runner [job polling, Docker container management]
      → Daemon [in-container: toolbox API, SSH, terminal]
    → Snapshot Manager [image storage/retrieval]
  → Proxy [auth + reverse proxy to sandbox ports]
  → SSH Gateway [token auth + SSH tunnel to runner → daemon]
```

### Port Map

| Service | Port | Protocol | Purpose |
|---------|------|----------|---------|
| API | 3000 | HTTP | REST API + WebSocket notifications |
| Proxy | 4000 | HTTP | Sandbox access proxy |
| Runner | 3003 | HTTP | Runner REST API |
| SSH Gateway | 2222 | SSH | External SSH entry point |
| Runner SSH | 2220 | SSH | Internal SSH forwarding |
| Daemon Toolbox | 2280 | HTTP | In-container REST API |
| Daemon SSH | 22220 | SSH | In-container SSH |
| Daemon Terminal | 22222 | WebSocket | Web terminal |
| Snapshot Manager | 5000 | HTTP | OCI registry |
| Dex | 5556 | HTTP | OIDC provider |
| Docker Registry | 6000 | HTTP | Container registry |
| PostgreSQL | 5432 | TCP | Database |
| Redis | 6379 | TCP | Cache |
| MinIO | 9001 | HTTP | S3 console |
| Jaeger | 16686 | HTTP | Tracing UI |
| PgAdmin | 5050 | HTTP | Database admin |
| MailDev | 1080 | HTTP | Email testing |

---

## Key Purpose

Daytona provides a secure infrastructure for running AI-generated code with:
- **Fast sandbox creation** (sub-90ms)
- **Isolated runtime** environments
- **Programmatic control** via SDKs
- **Persistent environments** with snapshots
- **Multi-language support** (Python, TypeScript)
