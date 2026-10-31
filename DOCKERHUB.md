# YtVault

Self-hosted YouTube archive system. Add channels, playlists, or individual videos — YtVault indexes them, schedules recurring scans, and downloads everything to your own storage.

## Quick Start

```bash
docker run -d \
  --name ytvault \
  -p 8080:8080 \
  -v ./data:/data \
  --restart unless-stopped \
  ac13139/ytvault:latest
```

Open **http://localhost:8080** in your browser.

> ⚠️ **Security notice:** YtVault has **no authentication by default**. Anyone who
> can reach port 8080 can drive the API and upload cookies.
>
> **Set `YTVAULT_AUTH_USER` and `YTVAULT_AUTH_PASSWORD`** to put the whole API
> behind HTTP Basic auth — the browser prompts once, the web UI needs no
> changes, and the CLI picks up the same two variables. If you leave them
> unset, run YtVault on a trusted network only: bind to localhost, put it
> behind an authenticating reverse proxy with TLS, or reach it over a VPN. Do
> not publish the port to the internet.
>
> `data/cookies.txt` is a **logged-in YouTube session** — treat it as a
> password. Details: [SECURITY.md](https://github.com/AC13139/ytvault/blob/main/SECURITY.md)

## Docker Compose

Create a `docker-compose.yml`:

```yaml
services:
  ytvault:
    image: ac13139/ytvault:latest
    ports:
      - "8080:8080"
    volumes:
      - ./data:/data
    environment:
      YTVAULT_ADDR: ":8080"
      YTVAULT_DATA_PATH: /data/state.json
      YTVAULT_DOWNLOAD_PATH: /data/downloads
      YTVAULT_COOKIES_PATH: /data/cookies.txt
      # Uncomment to require HTTP Basic auth on every route except /healthz.
      # YTVAULT_AUTH_USER: admin
      # YTVAULT_AUTH_PASSWORD: change-me
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/healthz"]
      interval: 30s
      timeout: 5s
      start_period: 10s
      retries: 3
```

Run:

```bash
docker compose up -d
```

## Pin a Version

```bash
# Specific version
docker pull ac13139/ytvault:1.0.0

# In docker-compose.yml
image: ac13139/ytvault:1.0.0
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `YTVAULT_ADDR` | `:8080` | Listen address |
| `YTVAULT_DATA_PATH` | `/data/state.json` | State file path |
| `YTVAULT_DOWNLOAD_PATH` | `/data/downloads` | Download directory |
| `YTVAULT_COOKIES_PATH` | `/data/cookies.txt` | Path to cookies file |
| `YTVAULT_AUTH_USER` | *(unset)* | HTTP Basic auth user. Requires `YTVAULT_AUTH_PASSWORD` too |
| `YTVAULT_AUTH_PASSWORD` | *(unset)* | HTTP Basic auth password. Requires `YTVAULT_AUTH_USER` too |

## Volumes

| Path | Description |
|------|-------------|
| `/data` | State file, downloaded videos, and cookies |

Contents of `./data/` on the host:
- `state.json` — application state (sources, media, tasks, schedules, config)
- `downloads/` — video files organized by channel/playlist
- `cookies.txt` — YouTube authentication cookies (required)

## Platforms

| Architecture | Support |
|---|---|
| `linux/amd64` | Linux servers, Intel Macs |
| `linux/arm64` | Apple Silicon Macs, Raspberry Pi, ARM servers |

## YouTube Cookies (Required)

YouTube blocks automated downloads. Export cookies from your browser **while logged in to YouTube** and either:

1. **Upload via Web UI** — go to the Cookies tab, upload or paste your cookies
2. **Copy to data directory** — save as `data/cookies.txt`, restart the container

Browser extensions for exporting cookies:
- Chrome/Edge/Brave: [Get cookies.txt LOCALLY](https://chromewebstore.google.com/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc)
- Firefox: [cookies.txt](https://addons.mozilla.org/en-US/firefox/addon/cookies-txt/)
- Safari: `yt-dlp --cookies-from-browser safari --cookies cookies.txt --skip-download https://www.youtube.com`

## Web UI Guide

### Sources
Manage your YouTube sources. Click **+ Create** to add a channel, playlist, or video by URL. Each source shows a progress bar (downloaded / indexed) and actions for reindexing, scheduling, retention, and cleanup.

### Media Library
Browse all videos with thumbnails. Filter by source or status. Download individual items or use **Download All** for batch operations. Pin important videos to protect them from auto-cleanup.

### Tasks
Monitor index and download operations. Start, stop, retry, or remove tasks. Bulk actions: stop all running, retry all failed, clear completed.

### Schedules
Set up recurring scans. Edit interval, date filter, and auto-download inline — changes save automatically. Start/pause schedules without removing them.

### Cookies
Upload and validate YouTube cookies. Status indicator shows Valid/Expired/Missing. Browser-specific setup guides included.

### Settings
Choose theme (YouTube 2026 or Classic), configure download templates, video quality, concurrent downloads, and default retention policy.

## Links

- **GitHub**: [github.com/AC13139/ytvault](https://github.com/AC13139/ytvault)
- **Issues**: [github.com/AC13139/ytvault/issues](https://github.com/AC13139/ytvault/issues)
- **Security policy**: [SECURITY.md](https://github.com/AC13139/ytvault/blob/main/SECURITY.md) — report vulnerabilities privately, not in a public issue

## License

[GNU General Public License v3.0](https://github.com/AC13139/ytvault/blob/main/LICENSE)
