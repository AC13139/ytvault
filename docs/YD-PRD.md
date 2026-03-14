# YtVault PRD

## 1. Product positioning
YtVault is a self-hosted YouTube acquisition and sync platform for reliably ingesting content into local storage and media workflows.

It is not just a one-off downloader. Its core value is stable long-running operation, task visibility, error diagnosis, and recovery.

## 2. Internal / external naming
- Internal codename: `YD`
- Formal project name: `YtVault`
- Proposed repo slug: `ytvault`
- Proposed CLI: `yd`

## 3. Target users
- Self-hosters who archive YouTube content
- Users who need recurring sync for channels/playlists
- Users who occasionally need one-off single-video capture
- Users who need Docker deployment and CLI automation

## 4. Core problems to solve
- Existing TubeSync-style deployments becoming unstable over time
- YouTube anti-bot checks causing metadata/download failures
- Poor visibility into why tasks failed
- Task locking / queue blockage / environment-specific failures
- Lack of a proper first-class single video workflow

## 5. Product goals
### Must achieve in Phase 1
1. Support source types:
   - Channel
   - Playlist
   - Single video
2. Support both:
   - Web UI
   - CLI
3. Make credentials/cookies first-class
4. Separate task types:
   - Index
   - Download
   - Metadata
5. Provide stable Docker Compose deployment
6. Provide operational visibility and diagnostics

### Explicit non-goals in Phase 1
- Multi-platform beyond YouTube
- Multi-tenant SaaS-style architecture
- Fancy consumer-grade UI
- Complex social/community features
- Advanced retention/deletion automation

## 6. Source model
### Channel
Recurring sync source that periodically discovers and downloads matching media.

### Playlist
Recurring sync source that periodically discovers and downloads matching media.

### Single video
First-class source type; not a playlist workaround.

Two future modes are envisioned:
- one-shot
- tracked

Phase 1 only requires `one-shot`.

## 7. Key capabilities
### 7.1 Source management
- Add/edit/pause/delete source
- Trigger manual reindex
- Trigger immediate download
- Configure per-source policies

### 7.2 Download policy
- Resolution preference
- Codec preference
- Audio-only option
- Subtitle policy
- Filename template
- Save path
- Fallback behavior

### 7.3 Credentials / cookies
- Import cookies file
- Import from browser export flow
- Store multiple credentials profiles
- Bind default or per-source credentials
- Detect invalid credentials
- Show auth-related failures clearly

### 7.4 Task center
- Show queued/running/failed/completed tasks
- Retry failed tasks
- Inspect task errors
- Show lock/stuck state clearly

### 7.5 Diagnostics
- Health checks
- Environment checks
- TLS / network sanity checks
- Worker heartbeat status
- yt-dlp / ffmpeg version visibility

## 8. Interfaces
### Web UI
Priority pages:
- Dashboard
- Sources
- Source detail
- Tasks
- Errors
- Credentials
- Settings

Style requirements:
- Simple
- Fast
- High information density
- Operationally focused

### CLI
Must support equivalent core actions:
- `yd source add channel <url>`
- `yd source add playlist <url>`
- `yd source add video <url>`
- `yd source list`
- `yd source reindex <id>`
- `yd task list`
- `yd task retry <id>`
- `yd media list`
- `yd doctor`
- `yd credentials import ...`

## 9. Success criteria for Phase 1
- A user can run YtVault via Docker Compose
- A user can add channel/playlist/video sources via Web or CLI
- The system indexes and downloads YouTube content reliably
- Failures are classified and understandable
- Credentials can be managed explicitly
- The system can recover from common operational failures without manual DB surgery

## 10. Value proposition
YtVault wins not by having more buttons, but by being more reliable, diagnosable, and recoverable than typical yt-dlp web wrappers.
