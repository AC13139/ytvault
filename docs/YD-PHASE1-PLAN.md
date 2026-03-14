# YtVault Phase 1 Plan

## Phase 1 objective
Deliver a stable self-hosted MVP for YouTube channel / playlist / single-video ingestion with Web UI, CLI, Docker deployment, cookies support, and diagnosable task execution.

## Workstreams

### WS1. Project skeleton
- initialize repo structure
- set up monorepo or multi-dir layout
- add lint/format/test scaffolding
- define configuration model

### WS2. API and persistence
- source CRUD
- policy CRUD
- task query endpoints
- credential profile endpoints
- PostgreSQL schema and migrations

### WS3. Queue and scheduler
- Redis-backed queues
- source due-scan scheduler
- job idempotency
- retry/backoff policies
- worker heartbeat

### WS4. YouTube ingestion workers
- URL validation
- channel indexing
- playlist indexing
- single video indexing
- metadata normalization
- download execution

### WS5. Credentials/cookies
- import cookie file
- bind credential profile to source/global default
- auth health check
- error propagation for bot-check scenarios

### WS6. Operator interfaces
- basic Web UI
- basic CLI
- doctor command
- task/error inspection views

### WS7. Packaging and deployment
- Dockerfiles
- Docker Compose
- environment configuration
- health checks
- local runbook

## Suggested build order
1. repository skeleton
2. database schema
3. API scaffold
4. job model and scheduler scaffold
5. single-video path first
6. playlist path
7. channel path
8. downloader path
9. credentials support
10. doctor / diagnostics
11. web UI and CLI polish
12. compose deployment validation

## Deliverables
### D1
Repo skeleton + architecture docs + config model

### D2
API + DB migrations + source/task models

### D3
Single-video ingest/download happy path

### D4
Playlist + channel recurring sync

### D5
Credentials support + failure classification

### D6
Web UI + CLI usable for daily ops

### D7
Docker Compose release candidate

## Acceptance checklist
- can add a channel
- can add a playlist
- can add a single video
- can import credentials/cookies
- can diagnose environment basics
- can view and retry failed tasks
- can download successfully into mounted storage
- can restart services without corrupting state

## Risks to manage first
- YouTube anti-bot/cookie handling
- TLS/network environment variance
- duplicate task generation
- filesystem consistency on interruption
- over-scoping UI before backend stabilizes
