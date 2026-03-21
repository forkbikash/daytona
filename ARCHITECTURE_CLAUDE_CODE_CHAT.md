
 ▐▛███▜▌   Claude Code v2.1.23
▝▜█████▛▘  Opus 4.5 · Claude Max
  ▘▘ ▝▝    ~/go/src/daytona

❯ i want the code part where we are actually spawning the vm 

⏺ Explore(Find VM spawning code)
  ⎿  Done (30 tool uses · 46.3k tokens · 1m 12s)

⏺ The VM spawning in Daytona is Docker-based. Here are the key locations:

  Primary spawn point - apps/runner/pkg/docker/create.go:88:

⏺ Read 1 file (ctrl+o to expand)

⏺ This is the core VM spawning code. The key parts:

  - Line 88-91: d.apiClient.ContainerCreate() - Actually creates the Docker
  container (the "VM")
  - Line 100: d.Start() - Starts the container after creation

  The flow is:
  1. Check if sandbox already exists (lines 36-57)
  2. Pull the image (line 62)
  3. Validate architecture (line 69)
  4. Prepare volumes (lines 75-81)
  5. Build container configs (line 83)
  6. Create container (line 88) ← The actual spawn
  7. Start container (line 100)
  8. Apply network rules (lines 112-135)

  The Start() method is in apps/runner/pkg/docker/start.go:51 where
  d.apiClient.ContainerStart() is called.

✻ Cooked for 1m 35s

❯ but docker container will eventually run somewhere right? where does it   
  run?                                                                      
  ⎿  Found 4 new diagnostic issues in 3 files (ctrl+o to expand)

⏺ The Docker containers run on Runner nodes. Let me find where the runner
  infrastructure is set up.

⏺ Read 1 file (ctrl+o to expand)

⏺ Based on the architecture documentation and code, here's how it works:

  Where Docker Containers Actually Run

  The containers run on Runner nodes - these are Go services deployed on host
  machines. The architecture is:

  ┌─────────────────────────────────────┐
  │       Daytona API Server            │
  │         (NestJS, remote)            │
  └──────────────┬──────────────────────┘
                 │ Job Queue / Long-Poll
                 ▼
  ┌─────────────────────────────────────┐
  │         RUNNER PROCESS              │  ← Go binary running on a host
  │  (deployed on physical/VM hosts)    │
  │                                     │
  │    ┌──────────────┐                 │
  │    │ DockerClient │                 │
  │    └──────┬───────┘                 │
  │           │                         │
  │           ▼                         │
  │    ┌──────────────┐                 │
  │    │   Docker     │  ← Docker Engine on the same host
  │    │   Engine     │
  │    └──────┬───────┘                 │
  │           │                         │
  │           ▼                         │
  │    ┌──────────────┐                 │
  │    │  Containers  │  ← Your sandboxes run here
  │    │  (sandboxes) │
  │    └──────────────┘                 │
  └─────────────────────────────────────┘

  Key points:

  1. Runner = Host machine - The runner is deployed on bare-metal or VM hosts
  that have Docker installed
  2. No separate VM hypervisor - Daytona uses Docker containers directly, not
  Firecracker/QEMU VMs
  3. Docker Engine - The runner talks to the local Docker daemon via the Docker
  API (d.apiClient.ContainerCreate() at apps/runner/pkg/docker/create.go:88)
  4. Deployment options:
    - As a systemd service (packaging/systemd/daytona-runner.service)
    - As a Debian package
    - As a Docker container itself (Dockerfile in the repo)

  The runner registers with the API server and polls for jobs. When a
  "CREATE_SANDBOX" job arrives, it uses the local Docker Engine to create and
  start containers.