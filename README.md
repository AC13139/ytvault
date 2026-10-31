# YtVault

Self-hosted YouTube archive system. Add channels, playlists, or individual videos — YtVault indexes them, schedules recurring scans, and downloads everything to your own storage.

[![CI](https://github.com/AC13139/ytvault/actions/workflows/ci.yml/badge.svg)](https://github.com/AC13139/ytvault/actions/workflows/ci.yml)
[![CodeQL](https://github.com/AC13139/ytvault/actions/workflows/codeql.yml/badge.svg)](https://github.com/AC13139/ytvault/actions/workflows/codeql.yml)
[![Security](https://github.com/AC13139/ytvault/actions/workflows/security.yml/badge.svg)](https://github.com/AC13139/ytvault/actions/workflows/security.yml)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/AC13139/ytvault/badge)](https://api.securityscorecards.dev/projects/github.com/AC13139/ytvault)
[![License: GPL-3.0](https://img.shields.io/badge/License-GPL--3.0-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/docker/pulls/ac13139/ytvault?logo=docker)](https://hub.docker.com/r/ac13139/ytvault)

## Screenshots

| Sources | Add New Source |
|---------|---------------|
| ![Sources](screenshots/sources.png) | ![Create](screenshots/create.png) |

| Media Library | Settings |
|---------------|----------|
| ![Media](screenshots/media.png) | ![Settings](screenshots/settings.png) |

## Features

- **Sources** — add YouTube channels, playlists, or single videos by URL
- **Collections** — group individual videos into custom local playlists
- **YouTube search** — search YouTube directly from the app to discover content
- **Indexing** — scans a source with `yt-dlp` and stores all video metadata
- **Downloading** — downloads video+audio merged to MP4 via `ffmpeg`
- **Scheduler** — set per-source intervals (1h–1 week); auto-download newly found videos
- **Media management** — pin videos, per-source retention policies, auto-cleanup
- **Themes** — YouTube 2026 (dark/light) and Classic Obsidian Glass (dark/light)
- **Web UI** — sidebar navigation, video thumbnails, responsive design
- **REST API** — full HTTP API for scripting and automation
- **Persistent storage** — JSON state file + download directory; survives restarts

---

## Installation

> **Docker is the recommended way to run YtVault.** The image includes all dependencies (yt-dlp, ffmpeg, Node.js, Python) pre-configured and supports `linux/amd64` and `linux/arm64`.
>
> **Tip:** For production use, pin a specific version tag (e.g. `ac13139/ytvault:1.1.0`) instead of `:latest`. The `:latest` tag updates on every commit to `main` and is also rebuilt weekly so base images and `yt-dlp` stay current — it may contain unreleased changes.

### Docker Compose (recommended)

```yaml
# docker-compose.yml
services:
  ytvault:
    image: ac13139/ytvault:1.1.0  # pin version for stability; use :latest for newest
    ports:
      - "${YTVAULT_PORT:-8080}:8080"
    volumes:
      - ./data:/data
    restart: unless-stopped
```

```bash
docker compose up -d
```

Open **http://localhost:8080** in your browser.

To pin a specific version:

```yaml
image: ac13139/ytvault:1.1.0
```

To use a custom port, create a `.env` file:

```bash
# .env
YTVAULT_PORT=9090
```

### Docker Run

```bash
docker run -d \
  --name ytvault \
  -p 8080:8080 \
  -v ./data:/data \
  --restart unless-stopped \
  ac13139/ytvault:latest
```

### Alternative: Build from Source

```bash
git clone https://github.com/AC13139/ytvault.git
cd ytvault
docker compose up --build -d
```

### Alternative: Run without Docker

**Prerequisites:** Go 1.26+, `yt-dlp`, `ffmpeg`, and `nodejs` on your PATH.

```bash
pip install "yt-dlp[default]"     # or: brew install yt-dlp
brew install ffmpeg nodejs        # or: apt install ffmpeg nodejs
go run ./apps/api
```

---

## YouTube Cookies (Required)

> **YouTube blocks automated downloads with bot detection.** You must provide cookies from a **logged-in** YouTube browser session. Without login cookies, most operations will fail with "Sign in to confirm you're not a bot" or "requires authentication" errors.

### How to Get Cookies

**Chrome / Edge / Brave / Arc (Chromium browsers):**

1. Install [Get cookies.txt LOCALLY](https://chromewebstore.google.com/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc) extension
2. Go to [youtube.com](https://www.youtube.com) and **make sure you are logged in**
3. Click the extension icon → Export → save the file

**Firefox:**

1. Install [cookies.txt](https://addons.mozilla.org/en-US/firefox/addon/cookies-txt/) add-on
2. Go to [youtube.com](https://www.youtube.com) and **make sure you are logged in**
3. Click the add-on → "Current Site" → save the file

**Safari (macOS):**

1. Install yt-dlp locally: `brew install yt-dlp`
2. Run: `yt-dlp --cookies-from-browser safari --cookies cookies.txt --skip-download https://www.youtube.com`
3. This generates a `cookies.txt` file in the current directory

### Upload Cookies

**Option A — Upload via Web UI (easiest):**

1. Open YtVault in your browser → go to the **Cookies** tab
2. Upload the `cookies.txt` file or paste its content
3. Click **Save Cookies** — validation runs automatically

**Option B — Copy to data directory:**

1. Copy `cookies.txt` to the `data/` directory on the host
2. Restart the container: `docker compose restart`

**Option C — Upload via API:**

```bash
curl -X POST http://localhost:8080/api/v1/cookies \
  --data-binary @cookies.txt \
  -H 'Content-Type: text/plain'

curl -X POST http://localhost:8080/api/v1/cookies/validate
```

### Notes

- Cookies must be from a **logged-in** session — visitor-only cookies will not work
- Cookies can be exported from **any machine** — they're not tied to the server
- Cookies expire over time (weeks to months) — re-export when downloads start failing
- For **age-restricted videos**: verify your age at [myaccount.google.com/birthday](https://myaccount.google.com/birthdayinfo), then re-export cookies
- The Cookies tab shows validation status and warns when login cookies are missing

---

## Web UI Guide

### Sources

The home page. Manage your YouTube sources.

- **+ Create** — add a new source by URL or search YouTube by keywords
- Each source card shows: title, type badge, URL, download progress bar
- **Reindex** — scan for new videos (with optional date filter)
- **Schedule** — set up recurring auto-scan (channel/playlist only)
- **Retention** — set per-source cleanup policy (delete old downloaded media)
- **Clean up** — manually run retention cleanup
- **View** — jump to the Media tab filtered to this source

### Media Library

Browse all indexed videos with thumbnails.

- **Download All** — start downloads for all pending/failed/missing media (respects source filter)
- **Filter by source** and **filter by status** (Pending / Downloading / Done / Failed / Missing)
- **Pin** (star icon) — protect a video from auto-cleanup
- **Download** / **Retry** / **Re-download** — per-video actions based on status
- Click the media info area to **copy the file path** to clipboard
- Metadata shown: channel, duration, file size, upload date, download date

### Tasks

Monitor all index and download operations.

- **Start** — manually start a queued task
- **Stop** — cancel a running task (kills the yt-dlp process)
- **Retry** — re-queue a failed task
- **Stop running** / **Retry failed** / **Clear completed** — bulk actions
- Filter by status (Queued / Running / Succeeded / Failed)
- Hover over a failed task's "details" link to see the error, click to copy

### Schedules

Overview of recurring scan schedules.

- Inline editing: change interval, date filter, or auto-download — saves automatically
- **Start / Pause** — enable or disable a schedule without removing it
- Shows last run and next run times

### Cookies

Manage YouTube authentication cookies.

- Status indicator: Valid (green) / Expired (amber) / Missing (red)
- Upload via file picker or paste
- **Test Now** — validates cookies against YouTube
- Browser-specific setup guides built in

### Settings

Configure download behavior and appearance.

- **Theme** — YouTube 2026 or Classic Obsidian Glass
- **Dark/Light** — toggle via moon/sun button in sidebar
- **Folder name** / **File name** — download path templates
- **Playlist overrides** — use playlist name as folder, playlist index in filename
- **Video quality** — Best MP4, Best any, 1080p, 720p, Audio only
- **Max concurrent downloads** — 1 to 5 parallel downloads
- **Default retention** — global cleanup policy (per-source can override)

---

## Configuration

All settings are environment variables:

| Variable | Default | Description |
|---|---|---|
| `YTVAULT_ADDR` | `:8080` | Listen address |
| `YTVAULT_DATA_PATH` | `/data/state.json` | State file path |
| `YTVAULT_DOWNLOAD_PATH` | `/data/downloads` | Download directory |
| `YTVAULT_COOKIES_PATH` | `/data/cookies.txt` | Path to cookies file |

**CLI:**

| Variable | Default | Description |
|---|---|---|
| `YTVAULT_API` | `http://127.0.0.1:8080` | API base URL |

---

## CLI Usage

```bash
go run ./apps/cli <command>
```

**Commands:**

```bash
yd health                                # check server health
yd source list                           # list all sources
yd source add video    <url>             # add a video source
yd source add playlist <url>             # add a playlist source
yd source add channel  <url>             # add a channel source
yd task list                             # list all tasks
yd task run <task_id>                    # stub-complete a queued task
```

---

## API Reference

### Sources

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/sources` | List all sources |
| `POST` | `/api/v1/sources` | Add a source |
| `DELETE` | `/api/v1/sources/{id}` | Remove a source (cascades) |
| `GET` | `/api/v1/sources/{id}/media` | List media items for a source |
| `POST` | `/api/v1/sources/{id}/reindex` | Start a new index task |
| `POST` | `/api/v1/sources/{id}/download-all` | Download all pending media for a source |
| `PUT` | `/api/v1/sources/{id}/retention` | Set retention policy |
| `GET` | `/api/v1/sources/{id}/cleanup-preview` | Preview cleanup |
| `POST` | `/api/v1/sources/{id}/cleanup` | Run cleanup |

### Collections

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/collections` | List all collections |
| `POST` | `/api/v1/collections` | Create a new collection |
| `POST` | `/api/v1/collections/{id}/items` | Add a video to a collection |

### Media

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/media/{id}/download` | Download a single media item |
| `POST` | `/api/v1/media/{id}/retry` | Retry a failed download |
| `POST` | `/api/v1/media/{id}/pin` | Toggle pin status |
| `DELETE` | `/api/v1/media/{id}` | Remove media (deletes file) |
| `POST` | `/api/v1/media/download-all-pending` | Download all pending/failed |

### Tasks

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/tasks` | List all tasks |
| `POST` | `/api/v1/tasks/{id}/start` | Start a queued task |
| `POST` | `/api/v1/tasks/{id}/cancel` | Cancel a running task |
| `POST` | `/api/v1/tasks/{id}/retry` | Retry a failed task |
| `POST` | `/api/v1/tasks/{id}/run` | Stub-complete a task (legacy/testing) |
| `POST` | `/api/v1/tasks/{id}/execute` | Execute with mode: `stub` or `fail_stub` |
| `DELETE` | `/api/v1/tasks/{id}` | Remove a completed task |
| `DELETE` | `/api/v1/tasks` | Clear all completed tasks |
| `POST` | `/api/v1/tasks/stop-all` | Cancel all running tasks |
| `POST` | `/api/v1/tasks/retry-failed` | Retry all failed tasks |

### Schedules

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/schedules` | List all schedules |
| `GET` | `/api/v1/sources/{id}/schedule` | Get schedule for a source |
| `POST` | `/api/v1/sources/{id}/schedule` | Create or update schedule |
| `DELETE` | `/api/v1/sources/{id}/schedule` | Remove schedule |

### Cookies

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/cookies/status` | Cookie file status + validation |
| `POST` | `/api/v1/cookies` | Upload cookies (file or raw text) |
| `DELETE` | `/api/v1/cookies` | Delete cookies file |
| `POST` | `/api/v1/cookies/validate` | Validate cookies against YouTube |

### Config & Search

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/config` | Get current config |
| `PUT` | `/api/v1/config` | Update config |
| `GET` | `/api/v1/youtube/search?q=keywords` | Search YouTube |
| `GET` | `/api/v1/youtube/probe?url=URL` | Diagnostic: test index a URL |
| `GET` | `/healthz` | Health check |

---

## Supported YouTube URL Formats

| Kind | Example URLs |
|---|---|
| Channel | `https://www.youtube.com/@Handle` |
| Channel | `https://www.youtube.com/channel/UCxxxxxx` |
| Playlist | `https://www.youtube.com/playlist?list=PLxxxxxx` |
| Video | `https://www.youtube.com/watch?v=xxxxxxx` |
| Video | `https://youtu.be/xxxxxxx` |

---

## Project Structure

```
apps/
  api/        HTTP server entry point
  cli/        CLI client
internal/
  api/        HTTP handlers and Web UI
  models/     shared data types
  scheduler/  background schedule runner
  store/      JSON state persistence
  worker/     async task executor (yt-dlp)
  yt/         YouTube URL normalisation
  ytdlp/      yt-dlp wrapper (index, download, search)
```

---

## Security

> ⚠️ **YtVault has no built-in authentication.** Every API endpoint — including
> destructive ones like creating sources, triggering downloads, and uploading
> cookies — is open to anyone who can reach the port.
>
> **Run it on a trusted network only.** Do not expose port 8080 to the internet.
> Put it behind an authenticating reverse proxy (Caddy/nginx/Traefik) with TLS,
> bind it to localhost with `YTVAULT_ADDR=127.0.0.1:8080`, or reach it over a
> VPN.

Your `cookies.txt` is a **logged-in YouTube session** — treat it as a password.

Full details, the security model, and the reporting process are in
**[SECURITY.md](SECURITY.md)**.

### Automated security checks

Every push and pull request runs:

| Check | Tool |
|---|---|
| Static analysis | [CodeQL](https://github.com/github/codeql) (`security-extended`) |
| Known vulnerabilities | [`govulncheck`](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) |
| Dependency review | [`dependency-review-action`](https://github.com/actions/dependency-review-action) |
| Formatting / vet / tests | `gofmt`, `go vet`, `go test -race` |
| Dependency updates | [Dependabot](https://docs.github.com/en/code-security/dependabot) (Actions, Docker, Go) |
| Container image CVEs | [Trivy](https://github.com/aquasecurity/trivy) scan of the published image |
| Secret detection | [Secret scanning](https://docs.github.com/en/code-security/secret-scanning) + push protection |
| Supply-chain posture | [OpenSSF Scorecard](https://github.com/ossf/scorecard) |

Report vulnerabilities privately via
[GitHub Security Advisories](https://github.com/AC13139/ytvault/security/advisories/new) — please don't open a public issue.

---

## Acknowledgments

YtVault is built on these open-source projects:

- [yt-dlp](https://github.com/yt-dlp/yt-dlp) — video extraction (Unlicense)
- [FFmpeg](https://github.com/FFmpeg/FFmpeg) — media processing (GPL-2.0-or-later)
- [Go](https://github.com/golang/go) — language and runtime (BSD-3-Clause)
- [Node.js](https://github.com/nodejs/node) — JavaScript runtime for yt-dlp (MIT)
- [Python](https://github.com/python/cpython) — runtime for yt-dlp (PSF-2.0)

## License

YtVault is licensed under the [GNU General Public License v3.0](LICENSE).
