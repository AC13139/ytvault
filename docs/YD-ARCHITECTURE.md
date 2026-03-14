# YtVault Architecture

## 1. Architecture principles
1. API-first
2. Web and CLI share the same service layer
3. Credentials/cookies are first-class
4. Index / download / metadata are separate execution lanes
5. Business state and execution state are modeled explicitly
6. Idempotency and recovery are mandatory

## 2. High-level architecture
- Control Plane API
- Scheduler
- Indexer Worker
- Downloader Worker
- Metadata Worker
- PostgreSQL
- Redis
- Web UI
- CLI

## 3. Recommended stack
### Control plane
- Go
- REST API
- Structured logging

### Workers
- Python
- `yt-dlp`
- `ffmpeg`

### Storage
- PostgreSQL for persistent state
- Redis for queues, locks, and transient state
- Mounted filesystem for downloads

### Frontend
- React-based admin UI

### Delivery
- Docker Compose first

## 4. Service roles
### API
- source CRUD
- policy CRUD
- credentials management
- task query / retry endpoints
- health / metrics endpoints

### Scheduler
- detect due sources
- enqueue idempotent index jobs
- enforce source-level pacing

### Indexer
- inspect channel/playlist/video
- normalize discovered media
- create/update media records
- enqueue downloads when policy allows

### Downloader
- resolve format selection
- use credentials when required
- download into temp directory
- invoke ffmpeg when needed
- atomically move final artifacts

### Metadata worker
- fetch/normalize detail metadata
- generate sidecar files
- trigger media server refresh hooks later

## 5. Core data model
### sources
- id
- kind: channel | playlist | video
- external_id
- url
- title
- status
- policy_id
- credential_profile_id
- next_index_at
- last_index_at
- last_success_index_at

### source_policies
- id
- resolution_policy
- codec_policy
- audio_only
- subtitle_policy
- filename_template
- save_path
- fallback_policy

### credential_profiles
- id
- name
- type: cookies
- storage_ref
- status
- last_validated_at
- last_error_code

### media_items
- id
- source_id
- external_id
- title
- published_at
- media_type
- availability_status
- ingest_status
- download_status
- skip_reason

### jobs
- id
- type: index | download | metadata
- entity_type
- entity_id
- biz_key
- status
- retry_count
- last_error_code
- last_error_message
- run_after

### job_events
- id
- job_id
- event_type
- payload
- created_at

### download_records
- id
- media_id
- attempt_no
- selected_format
- output_path
- status
- filesize
- error_code
- error_message

## 6. Error taxonomy
At minimum:
- AUTH_BOT_CHECK
- TLS_CERT_INVALID
- UPSTREAM_RATE_LIMIT
- FORMAT_UNAVAILABLE
- SOURCE_NOT_FOUND
- PRIVATE_OR_REMOVED
- WORKER_LOCK_TIMEOUT
- ENV_STORAGE_UNWRITABLE
- DEPENDENCY_MISSING
- INTERNAL_BUG

## 7. Key flows
### Add source
Web or CLI -> API -> validate URL -> create source -> optionally enqueue initial index

### Index source
Scheduler -> index job -> indexer -> normalize media -> upsert DB -> enqueue downloads

### Download media
Downloader -> resolve policy/credentials -> yt-dlp -> ffmpeg/postprocess -> atomic move -> update records

### Diagnose environment
CLI/Web doctor -> API -> checks DB/Redis/fs/yt-dlp/ffmpeg/network/TLS/cookies status

## 8. Reliability mechanisms
- job idempotency keys
- source-level locking
- queue separation by task type
- retry with backoff by error class
- atomic file move
- stuck job detection
- worker heartbeat
- reconciliation jobs

## 9. Deployment model
### Phase 1 Compose services
- api
- scheduler
- worker-indexer
- worker-downloader
- worker-metadata
- postgres
- redis
- web

## 10. Future extension points
- more platforms
- object storage
- notifications
- media-server integrations
- tracked single-video mode
