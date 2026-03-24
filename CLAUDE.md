# CLAUDE.md — Development Rules for ytvault

## UAT (User Acceptance Testing) Rules

**UAT is not a code review.** UAT means the feature actually works end-to-end. These rules apply to every UAT, including those delegated to Codex agents.

### Mandatory steps for every UAT

1. **Spin up a local sandbox** — never test against the user's production/deployment host.
   - Build the binary: `go build -o /tmp/ytvault-uat ./apps/api`
   - Start with a temp data dir (config is **env vars only**, no CLI flags):
     ```
     YTVAULT_ADDR=127.0.0.1:18080 \
     YTVAULT_DATA_PATH=/tmp/uat-data/state.json \
     YTVAULT_DOWNLOAD_PATH=/tmp/uat-data/downloads \
     /tmp/ytvault-uat &
     ```
   - Tear down and delete `/tmp/uat-data` after the test.

2. **yt-dlp must be available** — check with `which yt-dlp` before starting. If missing, install it: `pip3 install yt-dlp`.

3. **Test via CLI or API** — web UI automation is not required. Use `curl` or `go run ./apps/cli` against the sandbox server.

4. **A download UAT must include at least one successful download**:
   - Add a video source (single video URL)
   - Confirm the index task succeeds and a media item appears
   - Start a download task and wait for it to complete (poll `/api/v1/tasks` until status = `succeeded`)
   - Confirm the file exists on disk under the downloads directory

5. **Report PASS only when a file is on disk.** A green build and passing unit tests alone is not a UAT PASS.

### Example UAT sequence (curl)

```bash
BASE=http://127.0.0.1:18080  # matches YTVAULT_ADDR above

# Add video source
RESP=$(curl -sf -X POST $BASE/api/v1/sources \
  -H 'Content-Type: application/json' \
  -d '{"kind":"video","url":"https://www.youtube.com/watch?v=k1Pkg-hNFnM"}')
SOURCE_ID=$(echo $RESP | python3 -c "import sys,json; print(json.load(sys.stdin)['source']['id'])")
TASK_ID=$(echo $RESP | python3 -c "import sys,json; print(json.load(sys.stdin)['task']['id'])")

# Start index task
curl -sf -X POST $BASE/api/v1/tasks/$TASK_ID/start

# Wait for index to succeed, then start download
# (poll tasks, find download task, start it, wait for succeeded)
# Verify file exists under /tmp/uat-data/downloads/
```

## Code Review Rules

- All changes must pass `go build ./...` and `go test ./...` before commit.
- Codex reviews must state PASS or FAIL explicitly with evidence (test output, not just reasoning).
