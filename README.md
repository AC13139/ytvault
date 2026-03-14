# YtVault

YtVault is a self-hosted YouTube sync and download system focused on long-running stability, recoverability, and clear operations.

Internal codename: YD

## Phase 1 goals
- Support YouTube channel / playlist / single video sources
- Provide both Web UI and CLI
- Support credentials/cookies as a first-class capability
- Separate indexing, downloading, and metadata processing
- Ship as a stable Docker Compose deployment

## Current prototype boundary
- Source intake is real: API / Web / CLI validate supported YouTube URLs, normalize them, and persist a source record.
- Source presentation is intentionally product-like but honest: stored source rows include normalized URL, original input URL, locator, path label, intake stage, and prototype notes.
- Initial index tasks are queue stubs only. Live channel/playlist/video inspection and downloading are **not** wired yet.

### Example channel input accepted in this iteration
- `http://www.youtube.com/@SpeechLab-m7o`

## Quick validation
1. Start API: `YTVAULT_ADDR=:8080 YTVAULT_DATA_PATH=./tmp/demo-state.json go run ./apps/api`
2. API intake:
   `curl -s http://127.0.0.1:8080/api/v1/sources -X POST -H 'Content-Type: application/json' -d '{"kind":"channel","url":"http://www.youtube.com/@SpeechLab-m7o"}'`
3. CLI intake:
   `YTVAULT_API=http://127.0.0.1:8080 go run ./apps/cli source add channel 'http://www.youtube.com/@SpeechLab-m7o'`
4. Web intake: open `http://127.0.0.1:8080/`, choose `channel`, submit the same URL, then confirm the source row shows `@handle`, `validated`, normalized/original URL, and the prototype note.

## Documents
- `docs/YD-PRD.md`
- `docs/YD-ARCHITECTURE.md`
- `docs/YD-PHASE1-PLAN.md`
