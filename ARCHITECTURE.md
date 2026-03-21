# Daytona Architecture Documentation

**Version:** 1.0
**Last Updated:** January 2026

Daytona is a secure and elastic infrastructure platform for running AI-generated code. This document provides a comprehensive technical overview of the entire system architecture.

---

## Table of Contents

1. [System Overview](#1-system-overview)
2. [Technology Stack](#2-technology-stack)
3. [High-Level Architecture](#3-high-level-architecture)
4. [API Server (apps/api)](#4-api-server-appsapi)
5. [Infrastructure Services](#5-infrastructure-services)
6. [SDKs](#6-sdks)
7. [CLI](#7-cli)
8. [Dashboard](#8-dashboard)
9. [API Clients & Shared Libraries](#9-api-clients--shared-libraries)
10. [Data Flow & Communication Patterns](#10-data-flow--communication-patterns)
11. [Authentication & Authorization](#11-authentication--authorization)
12. [Database Schema](#12-database-schema)
13. [Configuration Management](#13-configuration-management)
14. [Deployment Architecture](#14-deployment-architecture)

---

## 1. System Overview

Daytona provides a secure infrastructure for running AI-generated code with:
- **Fast sandbox creation** (sub-90ms)
- **Isolated runtime** environments
- **Programmatic control** via SDKs
- **Persistent environments** with snapshots
- **Multi-language support** (Python, TypeScript, Go)

### Core Capabilities

| Capability | Description |
|------------|-------------|
| Sandbox Management | Create, start, stop, delete isolated compute environments |
| Snapshot System | Container image snapshots for instant provisioning |
| File Operations | Upload, download, search, replace files in sandboxes |
| Process Execution | Shell commands, PTY terminals, background sessions |
| Code Interpreter | Stateful Python execution with context isolation |
| Computer Use | VNC desktop with mouse/keyboard/screenshot control |
| Git Integration | Clone, commit, push, pull operations |
| LSP Support | Language server protocol for code intelligence |
| Volume Management | Persistent storage across sandbox lifecycles |
| Multi-tenancy | Organization-based isolation with RBAC |

---

## 2. Technology Stack

### Backend Services

| Component | Technology | Purpose |
|-----------|------------|---------|
| API Server | TypeScript/NestJS | REST API, business logic, orchestration |
| Runner | Go | Sandbox container management |
| Daemon | Go | In-sandbox runtime (PID 1) |
| Proxy | Go | Request routing, authentication |
| SSH Gateway | Go | SSH tunnel access |
| Snapshot Manager | Go | Docker registry for snapshots |
| CLI | Go | Command-line interface |

### Frontend

| Component | Technology | Purpose |
|-----------|------------|---------|
| Dashboard | React + Vite | Web UI |
| Styling | Tailwind CSS | UI styling |
| State | React Query + Context | Server/client state |
| Routing | React Router | Navigation |

### Data Layer

| Component | Technology | Purpose |
|-----------|------------|---------|
| Database | PostgreSQL | Primary data store |
| Cache | Redis | Caching, pub/sub, rate limiting |
| Object Storage | MinIO (S3-compatible) | Build contexts, backups |
| Registry | Distribution (Docker) | Container images |

### Observability

| Component | Technology | Purpose |
|-----------|------------|---------|
| Tracing | OpenTelemetry + Jaeger | Distributed tracing |
| Analytics | PostHog | Usage analytics |
| Logging | Pino (API), Logrus (Go) | Structured logging |
| Audit | OpenSearch (optional) | Audit log storage |

### Authentication

| Component | Technology | Purpose |
|-----------|------------|---------|
| Identity | Auth0 / OIDC | User authentication |
| API Auth | JWT + API Keys | Request authentication |
| Internal | Bearer tokens | Service-to-service |

---

## 3. High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              CLIENTS                                         │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │
│  │  Dashboard   │  │  Python SDK  │  │    TS SDK    │  │     CLI      │    │
│  │   (React)    │  │              │  │              │  │     (Go)     │    │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘    │
└─────────┼─────────────────┼─────────────────┼─────────────────┼────────────┘
          │                 │                 │                 │
          │    ┌────────────┴─────────────────┴────────────┐    │
          │    │         Main API Client                   │    │
          │    │    (Sandbox lifecycle, config, auth)      │    │
          │    └────────────────────┬──────────────────────┘    │
          │                         │                           │
          │    ┌────────────────────┴──────────────────────┐    │
          │    │        Toolbox API Client                 │    │
          │    │   (Files, Git, Process, LSP, ComputerUse) │    │
          │    └────────────────────┬──────────────────────┘    │
          │                         │                           │
          ▼                         ▼                           ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           API LAYER                                          │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                    API SERVER (NestJS)                               │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │    │
│  │  │  Auth    │ │ Sandbox  │ │ Snapshot │ │   Org    │ │  Runner  │  │    │
│  │  │ Module   │ │  Module  │ │  Module  │ │  Module  │ │  Module  │  │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘  │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │    │
│  │  │  Volume  │ │  Audit   │ │ Webhook  │ │  Region  │ │  Admin   │  │    │
│  │  │ Module   │ │  Module  │ │  Module  │ │  Module  │ │  Module  │  │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│                    ┌───────────────┼───────────────┐                        │
│                    ▼               ▼               ▼                        │
│  ┌──────────────────────┐ ┌──────────────┐ ┌──────────────────────┐        │
│  │     PostgreSQL       │ │    Redis     │ │      MinIO           │        │
│  │   (Primary Store)    │ │   (Cache)    │ │  (Object Storage)    │        │
│  └──────────────────────┘ └──────────────┘ └──────────────────────┘        │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ Runner Adapter (HTTP/Jobs)
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        INFRASTRUCTURE LAYER                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                      RUNNER (Go) - Port 8080                         │    │
│  │  ┌──────────────────────────────────────────────────────────────┐   │    │
│  │  │ Sandbox Management: Create, Start, Stop, Destroy, Backup      │   │    │
│  │  │ Snapshot Operations: Pull, Build, Tag, Push                   │   │    │
│  │  │ Container Lifecycle: Docker API integration                   │   │    │
│  │  │ Job Execution: Poller → Executor (v2 API)                     │   │    │
│  │  └──────────────────────────────────────────────────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│              ┌─────────────────────┼─────────────────────┐                  │
│              ▼                     ▼                     ▼                  │
│  ┌───────────────────┐ ┌───────────────────┐ ┌───────────────────┐         │
│  │   PROXY (Go)      │ │  SSH GATEWAY (Go) │ │ SNAPSHOT MANAGER  │         │
│  │   Port 4000       │ │   Port 2222       │ │    Port 5000      │         │
│  │                   │ │                   │ │                   │         │
│  │ - OIDC auth       │ │ - Token auth      │ │ - Docker Registry │         │
│  │ - Request routing │ │ - SSH forwarding  │ │ - S3/FS storage   │         │
│  │ - Preview URLs    │ │ - PTY support     │ │ - Image caching   │         │
│  └───────────────────┘ └───────────────────┘ └───────────────────┘         │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ Docker Container
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          SANDBOX CONTAINER                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                      DAEMON (Go) - PID 1                             │    │
│  │  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────────┐   │    │
│  │  │  Toolbox   │ │    SSH     │ │  Terminal  │ │  Computer Use  │   │    │
│  │  │ Port 2280  │ │  Port 22   │ │ Port 22222 │ │  VNC + NoVNC   │   │    │
│  │  │            │ │            │ │            │ │                │   │    │
│  │  │ - Files    │ │ - SFTP     │ │ - WebSocket│ │ - Xvfb         │   │    │
│  │  │ - Git      │ │ - Shell    │ │ - PTY      │ │ - xfce4        │   │    │
│  │  │ - Process  │ │ - Forward  │ │            │ │ - x11vnc       │   │    │
│  │  │ - LSP      │ │            │ │            │ │ - novnc        │   │    │
│  │  └────────────┘ └────────────┘ └────────────┘ └────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                      USER WORKSPACE                                  │    │
│  │  /home/user/  - Working directory, code, data                       │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. API Server (apps/api)

The API server is a NestJS application that serves as the central orchestration layer.

### 4.1 Module Structure

```
apps/api/src/
├── main.ts                    # Entry point, bootstrap
├── app.module.ts              # Root module
├── app.service.ts             # Initialization logic
├── config/                    # Configuration
│   └── configuration.ts       # TypedConfigService
├── filters/                   # Exception filters
├── interceptors/              # Request interceptors
├── middlewares/               # HTTP middlewares
└── [feature modules]/         # Feature-specific modules
```

### 4.2 Feature Modules (24 Total)

| Module | Purpose | Key Components |
|--------|---------|----------------|
| **SandboxModule** | Core sandbox operations | SandboxService, SandboxManager, RunnerAdapter |
| **SnapshotModule** | Container image management | SnapshotService, SnapshotManager, BackupManager |
| **RunnerModule** | Runner node management | RunnerService, health scoring algorithm |
| **VolumeModule** | Persistent storage | VolumeService, VolumeManager |
| **JobModule** | Async job queue | JobService, JobStateHandler |
| **OrganizationModule** | Multi-tenancy | OrgService, OrgUserService, OrgRoleService |
| **AuthModule** | Authentication | JwtStrategy, ApiKeyStrategy |
| **UserModule** | User management | UserService |
| **ApiKeyModule** | API key management | ApiKeyService |
| **RegionModule** | Region management | RegionService |
| **AuditModule** | Audit logging | AuditService, OpenSearch adapter |
| **WebhookModule** | Webhook delivery | WebhookService (Svix) |
| **DockerRegistryModule** | Registry management | DockerRegistryService |
| **ObjectStorageModule** | S3 integration | ObjectStorageService |
| **EmailModule** | Email sending | EmailService (SMTP) |
| **NotificationModule** | Event notifications | NotificationService |
| **AdminModule** | Admin operations | AdminController |
| **HealthModule** | Health checks | HealthController |
| **UsageModule** | Resource tracking | UsageService |
| **AnalyticsModule** | Analytics events | PostHog integration |
| **OpenFeatureModule** | Feature flags | PostHog provider |

### 4.3 Sandbox State Machine

```
                                    ┌──────────────┐
                                    │   UNKNOWN    │
                                    └──────┬───────┘
                                           │
                                           ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                           CREATION FLOW                                   │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                           │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐               │
│  │PENDING_BUILD │───▶│   BUILDING   │───▶│  CREATING    │               │
│  └──────────────┘    │   SNAPSHOT   │    └──────┬───────┘               │
│         │            └──────────────┘           │                        │
│         │                   │                   │                        │
│         ▼                   ▼                   ▼                        │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐               │
│  │ BUILD_FAILED │    │ BUILD_FAILED │    │  RESTORING   │               │
│  └──────────────┘    └──────────────┘    └──────┬───────┘               │
│                                                  │                        │
└──────────────────────────────────────────────────┼───────────────────────┘
                                                   │
                                                   ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                           LIFECYCLE FLOW                                  │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                           │
│                          ┌──────────────┐                                │
│              ┌──────────▶│   STARTED    │◀──────────┐                   │
│              │           └──────┬───────┘           │                    │
│              │                  │                   │                    │
│              │                  ▼                   │                    │
│       ┌──────┴──────┐    ┌──────────────┐    ┌─────┴──────┐             │
│       │  STARTING   │    │   STOPPING   │    │  STARTING  │             │
│       └──────▲──────┘    └──────┬───────┘    └─────▲──────┘             │
│              │                  │                   │                    │
│              │                  ▼                   │                    │
│              │           ┌──────────────┐           │                    │
│              └───────────│   STOPPED    │───────────┘                   │
│                          └──────┬───────┘                                │
│                                 │                                        │
└─────────────────────────────────┼────────────────────────────────────────┘
                                  │
                                  ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                           TERMINATION FLOW                                │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                           │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐               │
│  │  ARCHIVING   │───▶│   ARCHIVED   │    │  DESTROYING  │───▶ DESTROYED │
│  └──────────────┘    └──────────────┘    └──────────────┘               │
│                                                                           │
│  ┌──────────────┐                                                        │
│  │    ERROR     │ ◀── Any state can transition to ERROR                 │
│  └──────────────┘                                                        │
│                                                                           │
└──────────────────────────────────────────────────────────────────────────┘
```

**State Definitions:**

| State | Description |
|-------|-------------|
| `UNKNOWN` | Initial/undefined state |
| `PENDING_BUILD` | Waiting for snapshot build |
| `BUILDING_SNAPSHOT` | Building container image |
| `BUILD_FAILED` | Image build failed |
| `CREATING` | Creating container |
| `RESTORING` | Restoring from snapshot |
| `STARTED` | Running and accessible |
| `STARTING` | Transitioning to started |
| `STOPPED` | Stopped but preserved |
| `STOPPING` | Transitioning to stopped |
| `ARCHIVING` | Creating archive backup |
| `ARCHIVED` | Archived to storage |
| `DESTROYING` | Being deleted |
| `DESTROYED` | Fully cleaned up |
| `ERROR` | Error state (recoverable) |

### 4.4 Runner Adapter Pattern

The API communicates with runners through an adapter pattern supporting two versions:

**RunnerAdapterV0 (Legacy - Direct HTTP):**
```typescript
interface RunnerAdapterV0 {
  createSandbox(dto: CreateSandboxDTO): Promise<void>
  startSandbox(sandboxId: string): Promise<void>
  stopSandbox(sandboxId: string): Promise<void>
  destroySandbox(sandboxId: string): Promise<void>
  createBackup(sandboxId: string): Promise<void>
  buildSnapshot(dto: BuildSnapshotDTO): Promise<void>
  pullSnapshot(dto: PullSnapshotDTO): Promise<void>
}
```

**RunnerAdapterV2 (New - Job Queue):**
```typescript
interface RunnerAdapterV2 {
  // Creates jobs in database instead of direct HTTP calls
  // Runner polls for jobs via API
  createJob(type: JobType, payload: any): Promise<Job>
}
```

### 4.5 Runner Health Scoring

Runners are scored for sandbox placement using a weighted algorithm:

```typescript
score = (
  availabilityScore * 0.3 +
  resourceScore * 0.4 +
  performanceScore * 0.3
)

// Resource utilization penalties (exponential decay)
cpuPenalty = exp(-cpuUsage / targetCpu)
memoryPenalty = exp(-memoryUsage / targetMemory)
diskPenalty = exp(-diskUsage / targetDisk)
```

### 4.6 Job Queue System

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  API Server │────▶│  PostgreSQL │◀────│   Runner    │
│             │     │   (Jobs)    │     │  (Poller)   │
└─────────────┘     └─────────────┘     └──────┬──────┘
                           │                    │
                           │                    ▼
                    ┌──────▼──────┐      ┌─────────────┐
                    │    Redis    │      │  Executor   │
                    │ (Notify)    │      │             │
                    └─────────────┘      └─────────────┘
```

**Job Types:**
- `BACKUP_SNAPSHOT` - Create sandbox backup
- `BUILD_SNAPSHOT` - Build image from Dockerfile
- `PULL_SNAPSHOT` - Pull image from registry

**Job States:**
- `PENDING` → `IN_PROGRESS` → `COMPLETED` | `FAILED`

### 4.7 Guards & Interceptors

**Authentication Guards:**
| Guard | Purpose |
|-------|---------|
| `CombinedAuthGuard` | JWT + API-Key authentication |
| `RunnerAuthGuard` | Runner service authentication |
| `ProxyGuard` | Proxy service authentication |
| `SshGatewayGuard` | SSH gateway authentication |

**Authorization Guards:**
| Guard | Purpose |
|-------|---------|
| `SystemActionGuard` | Admin-only endpoints |
| `SandboxAccessGuard` | Sandbox ownership |
| `OrganizationAccessGuard` | Organization membership |
| `OrganizationResourceActionGuard` | Resource-level permissions |

**Global Interceptors:**
| Interceptor | Purpose |
|-------------|---------|
| `MetricsInterceptor` | PostHog analytics |
| `AuditInterceptor` | Request/response logging |
| `LoggerErrorInterceptor` | Error logging |

**Global Middlewares:**
| Middleware | Purpose |
|------------|---------|
| `VersionHeaderMiddleware` | API version header |
| `FailedAuthRateLimitMiddleware` | Block IPs after failed auth |
| `MaintenanceMiddleware` | 503 during maintenance |

---

## 5. Infrastructure Services

### 5.1 Runner (apps/runner)

The Runner service manages Docker containers for sandboxes.

**Port:** 8080

**Responsibilities:**
- Container lifecycle (create, start, stop, destroy)
- Snapshot operations (pull, build, tag, push)
- Health monitoring and metrics
- Job execution (v2 API)

**Package Structure:**
```
apps/runner/
├── cmd/runner/
│   ├── main.go           # Entry point
│   └── config/           # Configuration
├── pkg/
│   ├── api/              # HTTP API (Gin)
│   │   ├── controllers/  # Request handlers
│   │   ├── dto/          # Data transfer objects
│   │   └── middlewares/  # Auth, logging
│   ├── docker/           # Docker client wrapper
│   │   ├── create.go     # Container creation
│   │   ├── start.go      # Container startup
│   │   ├── stop.go       # Container shutdown
│   │   ├── destroy.go    # Container removal
│   │   ├── backup.go     # Image commit
│   │   ├── snapshot_*.go # Snapshot operations
│   │   └── monitor.go    # Event listener
│   ├── cache/            # State caching
│   ├── services/         # High-level services
│   └── runner/
│       └── v2/           # V2 job system
│           ├── executor/ # Job execution
│           ├── poller/   # Job polling
│           └── healthcheck/
```

**API Endpoints:**
```
GET    /                              # Health check
GET    /metrics                       # Prometheus metrics
GET    /info                          # Runner info

POST   /sandboxes                     # Create sandbox
GET    /sandboxes/{id}                # Get sandbox info
POST   /sandboxes/{id}/start          # Start sandbox
POST   /sandboxes/{id}/stop           # Stop sandbox
POST   /sandboxes/{id}/destroy        # Destroy sandbox
POST   /sandboxes/{id}/backup         # Create backup
POST   /sandboxes/{id}/resize         # Resize resources
POST   /sandboxes/{id}/recover        # Recover from backup
POST   /sandboxes/{id}/network-settings # Update network
ANY    /sandboxes/{id}/toolbox/*      # Toolbox proxy

POST   /snapshots/pull                # Pull snapshot
POST   /snapshots/build               # Build snapshot
POST   /snapshots/tag                 # Tag image
GET    /snapshots/exists              # Check existence
GET    /snapshots/info                # Get metadata
POST   /snapshots/remove              # Remove snapshot
GET    /snapshots/logs                # Build logs
```

### 5.2 Daemon (apps/daemon)

The Daemon runs inside each sandbox as PID 1.

**Ports:**
- 2280: Toolbox API
- 22: SSH server
- 22222: Terminal WebSocket

**Responsibilities:**
- Entrypoint command execution
- Toolbox API (files, git, process, LSP)
- SSH server with SFTP
- Terminal server (WebSocket)
- Computer use (VNC desktop)

**Package Structure:**
```
apps/daemon/
├── cmd/daemon/
│   ├── main.go           # Entry point
│   └── config/           # Configuration
├── pkg/
│   ├── toolbox/          # Toolbox API server
│   │   ├── fs/           # File system operations
│   │   ├── git/          # Git operations
│   │   ├── process/      # Process execution
│   │   │   ├── interpreter/  # Language runtimes
│   │   │   ├── pty/          # PTY allocation
│   │   │   └── session/      # Session management
│   │   ├── lsp/          # Language servers
│   │   ├── computeruse/  # Desktop automation
│   │   ├── proxy/        # Internal proxy
│   │   └── port/         # Port forwarding
│   ├── ssh/              # SSH server
│   └── terminal/         # Terminal server
```

**Toolbox API Endpoints:**
```
GET    /work-dir                      # Working directory
GET    /user-home-dir                 # Home directory
GET    /version                       # Daemon version

# File System
GET    /fs/*path                      # Read file/list directory
POST   /fs/*path                      # Write file
DELETE /fs/*path                      # Delete file
POST   /fs/search                     # Search files

# Process
POST   /process/execute               # Execute command
GET    /process/list                  # List processes
POST   /process/{pid}/kill            # Kill process

# Git
POST   /git/clone                     # Clone repository
POST   /git/commit                    # Commit changes
POST   /git/push                      # Push to remote
POST   /git/pull                      # Pull from remote
GET    /git/status                    # Get status

# LSP
POST   /lsp/initialize                # Initialize LSP
POST   /lsp/request                   # LSP request

# Computer Use
POST   /computeruse/screenshot        # Take screenshot
POST   /computeruse/click             # Mouse click
POST   /computeruse/type              # Type text
POST   /computeruse/move-mouse        # Move mouse
```

### 5.3 Proxy (apps/proxy)

The Proxy handles request routing and authentication for sandbox access.

**Port:** 4000

**Responsibilities:**
- OIDC/OAuth2 authentication
- Request routing to sandboxes
- Preview URL signing
- Activity tracking

**Authentication Methods (Priority Order):**
1. Bearer token (Authorization header)
2. X-Daytona-Preview-Token header
3. DAYTONA_SANDBOX_AUTH_KEY query param
4. daytona-sandbox-auth-{sandboxId} cookie
5. Signed preview URL token
6. OIDC redirect (fallback)

**Caching:**
- Redis (primary): Runner info, sandbox public flag, auth key validity
- In-memory (fallback): Map-based cache

### 5.4 SSH Gateway (apps/ssh-gateway)

The SSH Gateway provides SSH tunnel access to sandboxes.

**Port:** 2222

**Flow:**
1. Accept SSH connection
2. Extract token from username field
3. Validate token via API
4. Get sandbox/runner info
5. Forward SSH to runner:2220 → sandbox:22

### 5.5 Snapshot Manager (apps/snapshot-manager)

A Docker-compatible registry for storing container snapshots.

**Port:** 5000

**Storage Backends:**
- Filesystem (default): Local disk at `./data`
- S3: AWS S3 or compatible (MinIO)

**Features:**
- OCI distribution spec compliant
- In-memory caching layer
- Optional basic authentication
- Health check endpoint

---

## 6. SDKs

### 6.1 Python SDK (libs/sdk-python)

**Package:** `daytona`

**Architecture:**
```
src/daytona/
├── __init__.py           # Public exports
├── _sync/                # Synchronous implementations
│   ├── daytona.py        # Daytona client
│   ├── sandbox.py        # Sandbox class
│   ├── filesystem.py     # FileSystem operations
│   ├── git.py            # Git operations
│   ├── process.py        # Process execution
│   ├── code_interpreter.py  # Python interpreter
│   ├── computer_use.py   # Desktop automation
│   └── lsp_server.py     # LSP client
├── _async/               # Async implementations (mirrors _sync)
└── common/
    ├── errors.py         # Exception classes
    └── charts.py         # Chart types
```

**Main Classes:**

| Class | Purpose |
|-------|---------|
| `Daytona` | Client entry point, sandbox CRUD |
| `Sandbox` | Sandbox instance with sub-services |
| `FileSystem` | File operations (upload, download, search) |
| `Git` | Git operations (clone, commit, push, pull) |
| `Process` | Command execution, sessions, PTY |
| `CodeInterpreter` | Stateful Python execution |
| `ComputerUse` | Mouse, keyboard, screenshot |
| `LspServer` | Language server protocol |
| `VolumeService` | Volume management |
| `SnapshotService` | Snapshot management |

**Configuration:**
```python
DaytonaConfig(
    api_key="...",           # API key auth
    jwt_token="...",         # JWT auth (requires org_id)
    organization_id="...",   # Organization context
    api_url="https://...",   # API endpoint
    target="us-east-1"       # Target region
)
```

### 6.2 TypeScript SDK (libs/sdk-typescript)

**Package:** `@daytonaio/sdk`

**Architecture:** Mirrors Python SDK with TypeScript idioms

**Main Classes:**

| Class | Purpose |
|-------|---------|
| `Daytona` | Client entry point |
| `Sandbox` | Sandbox instance |
| `FileSystem` | File operations |
| `Git` | Git operations |
| `Process` | Command execution |
| `CodeInterpreter` | Python execution |
| `ComputerUse` | Desktop automation |
| `LspServer` | LSP client |

### 6.3 SDK Communication Pattern

```
┌─────────────────────────────────────────────────────────────────┐
│                         SDK                                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────────┐    ┌──────────────────────────────┐  │
│  │   Main API Client    │    │    Toolbox API Client        │  │
│  │                      │    │                              │  │
│  │ - Sandbox CRUD       │    │ - File operations            │  │
│  │ - Volumes            │    │ - Git operations             │  │
│  │ - Snapshots          │    │ - Process execution          │  │
│  │ - Config             │    │ - Code interpreter           │  │
│  │                      │    │ - Computer use               │  │
│  └──────────┬───────────┘    └──────────────┬───────────────┘  │
│             │                               │                   │
│             │ HTTP                          │ HTTP + WebSocket  │
│             │                               │                   │
└─────────────┼───────────────────────────────┼───────────────────┘
              │                               │
              ▼                               ▼
       ┌─────────────┐               ┌─────────────────┐
       │  API Server │               │ Proxy → Toolbox │
       │  (NestJS)   │──────────────▶│ (in sandbox)    │
       └─────────────┘  Get proxy    └─────────────────┘
                        URL for
                        toolbox
```

**Key Pattern:** Lazy Toolbox URL Resolution
1. SDK calls main API to get toolbox proxy URL
2. URL cached per sandbox/region
3. Subsequent toolbox calls go directly to proxy

---

## 7. CLI

### 7.1 Command Structure

```
daytona
├── login                    # OAuth or API key auth
├── logout                   # Clear credentials
├── version                  # Show version
│
├── [Sandbox Shortcuts]      # Top-level for convenience
│   ├── create               # Create sandbox
│   ├── list                 # List sandboxes
│   ├── info                 # Get sandbox info
│   ├── start                # Start sandbox
│   ├── stop                 # Stop sandbox
│   ├── delete               # Delete sandbox
│   ├── archive              # Archive sandbox
│   ├── ssh                  # SSH into sandbox
│   ├── exec                 # Execute command
│   └── preview-url          # Get preview URL
│
├── snapshot
│   ├── create               # Create from Dockerfile/image
│   ├── list                 # List snapshots
│   ├── delete               # Delete snapshot
│   └── push                 # Push local Docker image
│
├── volume
│   ├── create               # Create volume
│   ├── list                 # List volumes
│   ├── get                  # Get volume info
│   └── delete               # Delete volume
│
├── organization
│   ├── create               # Create organization
│   ├── list                 # List organizations
│   ├── use                  # Set active org
│   └── delete               # Delete organization
│
└── mcp
    ├── init                 # Initialize MCP server
    ├── start                # Start MCP server
    └── config               # Configure MCP
```

### 7.2 Authentication

**OAuth Flow:**
1. Start local callback server (port 9000)
2. Open browser to Auth0 authorization URL
3. User authenticates in browser
4. Auth0 redirects to callback with code
5. Exchange code for tokens
6. Store tokens in config

**API Key Flow:**
1. User provides API key via `--api-key` flag
2. Key stored in config profile
3. Used as Bearer token in requests

### 7.3 Configuration

**Location:** `~/.config/daytona/config.json`

```json
{
  "activeProfileId": "default",
  "profiles": [
    {
      "id": "default",
      "name": "Default",
      "api": {
        "url": "https://api.daytona.io",
        "key": "dtn_...",
        "token": {
          "accessToken": "...",
          "refreshToken": "...",
          "expiresAt": "..."
        }
      },
      "activeOrganizationId": "org_...",
      "toolboxProxyUrls": {
        "us-east-1": "https://..."
      }
    }
  ]
}
```

### 7.4 MCP Integration

The CLI includes an MCP (Model Context Protocol) server for AI assistant integration.

**MCP Tools (12):**
- `create_sandbox` - Create sandbox
- `destroy_sandbox` - Delete sandbox
- `execute_command` - Run shell command
- `file_upload` - Upload file
- `file_download` - Download file
- `file_info` - Get file info
- `list_files` - List directory
- `move_file` - Move/rename file
- `delete_file` - Delete file
- `create_folder` - Create directory
- `git_clone` - Clone repository
- `preview_link` - Get preview URL

---

## 8. Dashboard

### 8.1 Routes

| Route | Page | Access |
|-------|------|--------|
| `/` | Landing | Public |
| `/dashboard` | Dashboard | Auth |
| `/dashboard/sandboxes` | Sandboxes | Auth |
| `/dashboard/snapshots` | Snapshots | Auth |
| `/dashboard/volumes` | Volumes | READ_VOLUMES |
| `/dashboard/keys` | API Keys | Auth |
| `/dashboard/members` | Members | Non-personal org |
| `/dashboard/settings` | Settings | Auth |
| `/dashboard/limits` | Limits | Owner |
| `/dashboard/billing/spending` | Spending | Owner + Billing |
| `/dashboard/billing/wallet` | Wallet | Owner + Billing |
| `/dashboard/audit-logs` | Audit Logs | READ_AUDIT_LOGS |
| `/dashboard/regions` | Regions | Feature flag |
| `/dashboard/runners` | Runners | Feature flag |

### 8.2 State Management

| Layer | Technology | Purpose |
|-------|-----------|---------|
| Server State | React Query | API data, caching |
| Auth State | react-oidc-context | OIDC authentication |
| Context State | React Context | Organization, theme |
| Local Storage | Browser | Persistence |

### 8.3 Context Providers

```jsx
<ErrorBoundary>
  <QueryProvider>
    <ThemeProvider>
      <ConfigProvider>
        <AuthProvider>
          <PostHogProvider>
            <BrowserRouter>
              <ApiProvider>
                <OrganizationsProvider>
                  <SelectedOrganizationProvider>
                    <RegionsProvider>
                      <NotificationSocketProvider>
                        <CommandPaletteProvider>
                          <App />
                        </CommandPaletteProvider>
                      </NotificationSocketProvider>
                    </RegionsProvider>
                  </SelectedOrganizationProvider>
                </OrganizationsProvider>
              </ApiProvider>
            </BrowserRouter>
          </PostHogProvider>
        </AuthProvider>
      </ConfigProvider>
    </ThemeProvider>
  </QueryProvider>
</ErrorBoundary>
```

### 8.4 API Integration

**Query Hooks:**
- `useSandboxes()` - Paginated sandbox list
- `useSnapshots()` - Paginated snapshot list
- `useOrganizations()` - Organization list
- `useRegions()` - Available regions
- `useOwnerWalletQuery()` - Wallet info
- `useOwnerTierQuery()` - Tier info
- `useOrganizationUsageOverviewQuery()` - Usage metrics

**Mutation Hooks:**
- `useUpgradeTierMutation()`
- `useDowngradeTierMutation()`
- `useRedeemCouponMutation()`
- `useSetAutomaticTopUpMutation()`
- `useDeleteOrganizationMutation()`

---

## 9. API Clients & Shared Libraries

### 9.1 Generated API Clients

All API clients are generated from OpenAPI specifications.

| Client | Language | Package | APIs |
|--------|----------|---------|------|
| api-client | TypeScript | @daytonaio/api-client | 22 |
| api-client-go | Go | github.com/daytonaio/api-client-go | 24 |
| api-client-python | Python | daytona_api_client | 22 |
| api-client-python-async | Python | daytona_api_client_async | 22 |
| runner-api-client | TypeScript | @daytonaio/runner-api-client | 6 |
| toolbox-api-client | TypeScript | @daytonaio/toolbox-api-client | 8 |
| toolbox-api-client-python | Python | daytona_toolbox_api_client | 8 |
| toolbox-api-client-python-async | Python | daytona_toolbox_api_client_async | 8 |

### 9.2 Shared Go Libraries

**libs/common-go:**
```go
// Error handling
type ErrorResponse struct {
    StatusCode int
    Message    string
    Code       string
}

// Cache interface
type ICache[T any] interface {
    Get(ctx, key) (*T, error)
    Set(ctx, key, value, expiration) error
    Delete(ctx, key) error
    Has(ctx, key) (bool, error)
}

// Implementations: MapCache, RedisCache

// Proxy handler
func NewProxyRequestHandler(
    getTarget func(*gin.Context) (*url.URL, map[string]string, error),
    modifyResponse func(*http.Response) error,
) gin.HandlerFunc
```

**libs/computer-use:**
```go
// Desktop automation
type ComputerUse struct {
    Initialize() error
    Start() error
    Stop() error
    GetProcessStatus() (*ProcessStatus, error)
}

// Processes managed:
// 1. Xvfb (X Virtual Framebuffer)
// 2. xfce4 (Desktop environment)
// 3. x11vnc (VNC server)
// 4. novnc (Web VNC client)
```

---

## 10. Data Flow & Communication Patterns

### 10.1 Sandbox Creation Flow

```
┌────────┐     ┌─────────┐     ┌──────────┐     ┌────────┐     ┌──────────┐
│  SDK   │────▶│   API   │────▶│  Runner  │────▶│ Docker │────▶│ Container│
└────────┘     └────┬────┘     └────┬─────┘     └────────┘     └──────────┘
                    │               │
                    │               │
              ┌─────▼─────┐   ┌─────▼─────┐
              │ PostgreSQL│   │ Snapshot  │
              │  (Jobs)   │   │  Manager  │
              └───────────┘   └───────────┘

1. SDK calls API: POST /sandbox
2. API validates quota, selects runner
3. API creates Job (v2) or calls runner directly (v0)
4. Runner pulls snapshot from Snapshot Manager
5. Runner creates Docker container
6. Daemon starts inside container
7. API updates sandbox state
8. SDK receives sandbox info
```

### 10.2 Command Execution Flow

```
┌────────┐     ┌────────┐     ┌──────────┐     ┌─────────┐
│  SDK   │────▶│ Proxy  │────▶│  Daemon  │────▶│ Process │
└────────┘     └────────┘     │ (Toolbox)│     └─────────┘
                              └──────────┘

1. SDK gets toolbox proxy URL from API
2. SDK calls Proxy: POST /process/execute
3. Proxy routes to Daemon toolbox port (2280)
4. Daemon executes command
5. Result returned through chain
```

### 10.3 SSH Access Flow

```
┌────────────┐     ┌─────────────┐     ┌────────┐     ┌──────────┐
│ SSH Client │────▶│ SSH Gateway │────▶│ Runner │────▶│  Daemon  │
└────────────┘     └──────┬──────┘     └────────┘     │  (SSH)   │
                          │                           └──────────┘
                    ┌─────▼─────┐
                    │    API    │
                    │ (validate)│
                    └───────────┘

1. SSH client connects to SSH Gateway (port 2222)
2. Token passed as username
3. Gateway validates token via API
4. Gateway gets runner info
5. Gateway forwards to runner:2220 → sandbox:22
6. Daemon SSH server handles connection
```

### 10.4 Preview URL Flow

```
┌─────────┐     ┌────────┐     ┌────────┐     ┌──────────┐
│ Browser │────▶│ Proxy  │────▶│ Runner │────▶│  Daemon  │
└─────────┘     └───┬────┘     └────────┘     │(Service) │
                    │                          └──────────┘
                    │
              ┌─────▼─────┐
              │   OIDC    │
              │ Provider  │
              └───────────┘

1. Browser requests preview URL
2. Proxy checks authentication
3. If not auth'd, redirect to OIDC
4. After auth, Proxy routes to sandbox
5. Service in sandbox responds
```

---

## 11. Authentication & Authorization

### 11.1 Authentication Methods

| Method | Use Case | Flow |
|--------|----------|------|
| JWT (OIDC) | Dashboard, SDK with JWT | OIDC authorization code flow |
| API Key | SDK, CLI, automation | Bearer token |
| Runner Token | Runner → API | Service token |
| Proxy Token | Proxy → API | Service token |
| SSH Token | SSH Gateway | Per-session token |
| Preview Token | Signed URLs | Time-limited signature |

### 11.2 Authorization Model

**System Roles:**
- `ADMIN` - Full system access
- `USER` - Standard user

**Organization Roles:**
- `OWNER` - Full org access
- `MEMBER` - Limited access
- Custom roles with granular permissions

**Permissions:**
```typescript
enum Permission {
  // Sandbox
  CREATE_SANDBOX
  READ_SANDBOX
  UPDATE_SANDBOX
  DELETE_SANDBOX

  // Snapshot
  CREATE_SNAPSHOT
  READ_SNAPSHOT
  DELETE_SNAPSHOT

  // Volume
  CREATE_VOLUME
  READ_VOLUMES
  DELETE_VOLUME

  // Organization
  MANAGE_MEMBERS
  MANAGE_ROLES
  UPDATE_ORGANIZATION
  DELETE_ORGANIZATION

  // Audit
  READ_AUDIT_LOGS

  // Infrastructure
  READ_RUNNERS
  MANAGE_RUNNERS
  READ_REGIONS
  MANAGE_REGIONS
}
```

### 11.3 Rate Limiting

| Profile | Purpose | Default |
|---------|---------|---------|
| anonymous | Public endpoints | 100/min |
| failedAuth | Failed auth tracking | 10/5min |
| authenticated | Authenticated users | 1000/min |
| sandboxCreate | Sandbox creation | 10/min |
| sandboxLifecycle | Sandbox operations | 100/min |

---

## 12. Database Schema

### 12.1 Core Entities

**Sandbox:**
```sql
CREATE TABLE sandbox (
  id UUID PRIMARY KEY,
  organization_id UUID NOT NULL,
  name VARCHAR NOT NULL,
  state VARCHAR NOT NULL,
  desired_state VARCHAR,
  runner_id UUID,
  snapshot_id UUID,
  cpu INTEGER,
  memory_gib INTEGER,
  disk_gib INTEGER,
  gpu VARCHAR,
  env JSONB,
  labels JSONB,
  public BOOLEAN DEFAULT false,
  auto_stop_interval INTEGER,
  auto_archive_interval INTEGER,
  auto_delete_interval INTEGER,
  last_activity_at TIMESTAMP,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);
```

**Runner:**
```sql
CREATE TABLE runner (
  id UUID PRIMARY KEY,
  name VARCHAR NOT NULL,
  domain VARCHAR NOT NULL,
  api_url VARCHAR,
  proxy_url VARCHAR,
  api_key VARCHAR,
  api_version INTEGER,
  region_id UUID,
  state VARCHAR NOT NULL,
  cpu INTEGER,
  memory_gib INTEGER,
  disk_gib INTEGER,
  current_allocated_cpu INTEGER,
  current_allocated_memory_gib INTEGER,
  current_allocated_disk_gib INTEGER,
  current_cpu_usage_percentage DECIMAL,
  current_memory_usage_percentage DECIMAL,
  current_disk_usage_percentage DECIMAL,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);
```

**Snapshot:**
```sql
CREATE TABLE snapshot (
  id UUID PRIMARY KEY,
  organization_id UUID,
  name VARCHAR NOT NULL,
  image_name VARCHAR NOT NULL,
  ref VARCHAR,
  state VARCHAR NOT NULL,
  size INTEGER,
  cpu INTEGER,
  memory_gib INTEGER,
  disk_gib INTEGER,
  general BOOLEAN DEFAULT false,
  build_info_id UUID,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);
```

**Organization:**
```sql
CREATE TABLE organization (
  id UUID PRIMARY KEY,
  name VARCHAR NOT NULL,
  personal BOOLEAN DEFAULT false,
  suspended BOOLEAN DEFAULT false,
  suspension_reason VARCHAR,
  default_region_id UUID,
  quota_total_cpu INTEGER,
  quota_total_memory_gib INTEGER,
  quota_total_disk_gib INTEGER,
  quota_max_sandboxes INTEGER,
  quota_max_snapshots INTEGER,
  quota_max_volumes INTEGER,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);
```

**Job:**
```sql
CREATE TABLE job (
  id UUID PRIMARY KEY,
  type VARCHAR NOT NULL,
  runner_id UUID NOT NULL,
  resource_type VARCHAR NOT NULL,
  resource_id UUID NOT NULL,
  status VARCHAR NOT NULL,
  payload JSONB,
  result JSONB,
  error VARCHAR,
  trace_context JSONB,
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  completed_at TIMESTAMP,
  UNIQUE (resource_type, resource_id, runner_id, status)
);
```

### 12.2 Entity Relationships

```
Organization ──┬── OrganizationUser ─── User
               ├── OrganizationRole
               ├── OrganizationInvitation
               ├── Sandbox ─── Volume (mount)
               ├── Snapshot ─── BuildInfo
               ├── Volume
               ├── ApiKey
               ├── AuditLog
               └── RegionQuota ─── Region

Runner ──┬── Sandbox
         ├── Job
         └── SnapshotRunner ─── Snapshot

Region ──┬── Runner
         └── SnapshotRegion ─── Snapshot
```

---

## 13. Configuration Management

### 13.1 API Server Configuration

**Environment Variables:**

| Category | Variables |
|----------|-----------|
| **Application** | `NODE_ENV`, `PORT`, `APP_URL`, `VERSION` |
| **Database** | `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_TLS_*` |
| **Redis** | `REDIS_HOST`, `REDIS_PORT`, `REDIS_TLS_*` |
| **OIDC** | `OIDC_CLIENT_ID`, `OIDC_ISSUER`, `OIDC_AUDIENCE` |
| **SMTP** | `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD` |
| **S3** | `S3_ENDPOINT`, `S3_REGION`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET` |
| **Registries** | `TRANSIENT_REGISTRY_*`, `INTERNAL_REGISTRY_*`, `BACKUP_REGISTRY_*` |
| **Proxy** | `PROXY_DOMAIN`, `PROXY_PROTOCOL`, `PROXY_API_KEY` |
| **Audit** | `AUDIT_RETENTION_DAYS`, `AUDIT_OPENSEARCH_*` |
| **Kafka** | `KAFKA_ENABLED`, `KAFKA_BROKERS`, `KAFKA_CLIENT_ID` |
| **Analytics** | `POSTHOG_API_KEY`, `POSTHOG_HOST` |
| **Defaults** | `DEFAULT_REGION_*`, `DEFAULT_RUNNER_*`, `DEFAULT_SNAPSHOT_*` |
| **Rate Limits** | `RATE_LIMIT_*_TTL`, `RATE_LIMIT_*_LIMIT` |

### 13.2 Runner Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `DAYTONA_API_URL` | - | Central API URL |
| `DAYTONA_RUNNER_TOKEN` | - | Auth token |
| `API_PORT` | 8080 | HTTP port |
| `API_VERSION` | 2 | API version (1 or 2) |
| `POLL_TIMEOUT` | 30s | Job polling timeout |
| `POLL_LIMIT` | 10 | Max jobs per poll |
| `HEALTHCHECK_INTERVAL` | 30s | Health check frequency |
| `BACKUP_TIMEOUT_MIN` | 60 | Backup timeout |
| `AWS_*` | - | S3 credentials |

### 13.3 Proxy Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `PROXY_PORT` | 4000 | Listen port |
| `PROXY_PROTOCOL` | https | HTTP or HTTPS |
| `PROXY_API_KEY` | - | API auth key |
| `COOKIE_DOMAIN` | - | Auth cookie domain |
| `DAYTONA_API_URL` | - | API endpoint |
| `OIDC_*` | - | OIDC configuration |
| `REDIS_*` | - | Cache backend |
| `TOOLBOX_ONLY_MODE` | false | Restrict to toolbox |

### 13.4 Daemon Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `DAYTONA_DAEMON_LOG_FILE_PATH` | - | Log file location |
| `DAYTONA_ENTRYPOINT_LOG_FILE_PATH` | - | Entrypoint logs |
| `ENTRYPOINT_SHUTDOWN_TIMEOUT_SEC` | 10 | Shutdown grace period |
| `SIGTERM_SHUTDOWN_TIMEOUT_SEC` | 5 | SIGTERM timeout |
| `DAYTONA_USER_HOME_AS_WORKDIR` | false | Use home as workdir |

---

## 14. Deployment Architecture

### 14.1 Docker Compose (Development)

```yaml
services:
  postgres:
    image: postgres:15
    ports: ["5432:5432"]

  redis:
    image: redis:7
    ports: ["6379:6379"]

  minio:
    image: minio/minio
    ports: ["9000:9000", "9001:9001"]

  dex:
    image: dexidp/dex
    ports: ["5556:5556"]

  registry:
    image: registry:2
    ports: ["5000:5000"]

  maildev:
    image: maildev/maildev
    ports: ["1080:1080", "1025:1025"]

  jaeger:
    image: jaegertracing/all-in-one
    ports: ["16686:16686", "4318:4318"]

  api:
    build: ./apps/api
    ports: ["3000:3000"]
    depends_on: [postgres, redis, minio]

  proxy:
    build: ./apps/proxy
    ports: ["4000:4000"]
    depends_on: [api, redis]

  runner:
    build: ./apps/runner
    ports: ["8080:8080"]
    depends_on: [api]
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock

  ssh-gateway:
    build: ./apps/ssh-gateway
    ports: ["2222:2222"]
    depends_on: [api]
```

### 14.2 Production Topology

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           LOAD BALANCER                                  │
│                    (SSL termination, routing)                           │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        │                       │                       │
        ▼                       ▼                       ▼
┌───────────────┐      ┌───────────────┐      ┌───────────────┐
│  API Cluster  │      │ Proxy Cluster │      │ SSH Gateway   │
│  (3+ nodes)   │      │  (3+ nodes)   │      │   Cluster     │
└───────┬───────┘      └───────────────┘      └───────────────┘
        │
        │
┌───────┴────────────────────────────────────────────────────┐
│                    DATA LAYER                               │
├─────────────────┬─────────────────┬────────────────────────┤
│   PostgreSQL    │     Redis       │      MinIO/S3          │
│   (Primary +    │   (Cluster)     │      (Cluster)         │
│    Replicas)    │                 │                        │
└─────────────────┴─────────────────┴────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                        RUNNER NODES (per region)                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐         │
│  │   Runner Node   │  │   Runner Node   │  │   Runner Node   │         │
│  │                 │  │                 │  │                 │         │
│  │ ┌─────────────┐ │  │ ┌─────────────┐ │  │ ┌─────────────┐ │         │
│  │ │  Sandbox 1  │ │  │ │  Sandbox 4  │ │  │ │  Sandbox 7  │ │         │
│  │ └─────────────┘ │  │ └─────────────┘ │  │ └─────────────┘ │         │
│  │ ┌─────────────┐ │  │ ┌─────────────┐ │  │ ┌─────────────┐ │         │
│  │ │  Sandbox 2  │ │  │ │  Sandbox 5  │ │  │ │  Sandbox 8  │ │         │
│  │ └─────────────┘ │  │ └─────────────┘ │  │ └─────────────┘ │         │
│  │ ┌─────────────┐ │  │ ┌─────────────┐ │  │ ┌─────────────┐ │         │
│  │ │  Sandbox 3  │ │  │ │  Sandbox 6  │ │  │ │  Sandbox 9  │ │         │
│  │ └─────────────┘ │  │ └─────────────┘ │  │ └─────────────┘ │         │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘         │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                    Snapshot Manager (per region)                 │    │
│  │                    S3/Filesystem storage                         │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 14.3 Scaling Considerations

| Component | Scaling Strategy |
|-----------|------------------|
| API Server | Horizontal (stateless) |
| Proxy | Horizontal (stateless with Redis) |
| SSH Gateway | Horizontal (stateless) |
| Runner | Vertical + Horizontal per region |
| PostgreSQL | Primary + Read replicas |
| Redis | Cluster mode |
| MinIO | Distributed mode |
| Snapshot Manager | Per region with S3 backend |

---

## Appendix A: File Locations

| Component | Path |
|-----------|------|
| API Server | `apps/api/` |
| Runner | `apps/runner/` |
| Daemon | `apps/daemon/` |
| Proxy | `apps/proxy/` |
| SSH Gateway | `apps/ssh-gateway/` |
| Snapshot Manager | `apps/snapshot-manager/` |
| CLI | `apps/cli/` |
| Dashboard | `apps/dashboard/` |
| Python SDK | `libs/sdk-python/` |
| TypeScript SDK | `libs/sdk-typescript/` |
| API Clients | `libs/api-client*/` |
| Common Go | `libs/common-go/` |
| Computer Use | `libs/computer-use/` |
| Docker Config | `docker/` |
| Examples | `examples/` |
| Guides | `guides/` |

## Appendix B: Port Reference

| Service | Port | Purpose |
|---------|------|---------|
| API Server | 3000 | REST API |
| Proxy | 4000 | Request routing |
| Runner | 8080 | Runner API |
| SSH Gateway | 2222 | SSH tunnel |
| Snapshot Manager | 5000 | Docker registry |
| PostgreSQL | 5432 | Database |
| Redis | 6379 | Cache |
| MinIO | 9000/9001 | Object storage |
| Daemon Toolbox | 2280 | Toolbox API |
| Daemon SSH | 22 | SSH server |
| Daemon Terminal | 22222 | WebSocket terminal |
| VNC | 5901 | VNC server |
| NoVNC | 6080 | Web VNC |

## Appendix C: API Endpoints Summary

### Main API (`/api`)
- `/sandbox/*` - Sandbox CRUD, state management
- `/snapshot/*` - Snapshot operations
- `/volume/*` - Volume management
- `/runner/*` - Runner management
- `/organization/*` - Organization CRUD, members, roles
- `/user/*` - User profile
- `/api-keys/*` - API key management
- `/region/*` - Region management
- `/audit/*` - Audit logs
- `/webhook/*` - Webhook configuration
- `/config` - Dashboard configuration
- `/health` - Health check

### Runner API
- `/sandboxes/*` - Container lifecycle
- `/snapshots/*` - Image operations
- `/info` - Runner info
- `/metrics` - Prometheus metrics

### Toolbox API (in Daemon)
- `/fs/*` - File operations
- `/git/*` - Git operations
- `/process/*` - Process execution
- `/lsp/*` - Language server
- `/computeruse/*` - Desktop automation
- `/port/*` - Port forwarding
