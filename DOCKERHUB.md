# YtVault

Self-hosted YouTube archive system. Add channels, playlists, or individual videos — YtVault indexes them, schedules recurring scans, and downloads everything to your own storage.

## Features

- Add YouTube channels, playlists, or single videos by URL
- YouTube search built-in — discover content directly from the app
- Scheduled recurring scans with auto-download
- Media management — pin videos, retention policies, auto-cleanup
- Cookie management for age-restricted content
- Configurable download quality and filename templates
- Two UI themes: YouTube 2026 and Classic Obsidian Glass (dark/light)
- Responsive web UI with sidebar navigation and video thumbnails
- REST API for scripting and automation

## Quick Start

```bash
docker run -d \
  --name ytvault \
  -p 8080:8080 \
  -v ./data:/data \
  ac13139/ytvault:latest
```

Open **http://localhost:8080** in your browser.

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
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/healthz"]
      interval: 30s
      timeout: 5s
      start_period: 10s
      retries: 3
```

Then run:

```bash
docker compose up -d
```

## Pin a Specific Version

```bash
docker pull ac13139/ytvault:1.0.0
```

Or in docker-compose.yml:

```yaml
image: ac13139/ytvault:1.0.0
```

## Custom Port

To use a different port (e.g., 9090):

```bash
docker run -d --name ytvault -p 9090:8080 -v ./data:/data ac13139/ytvault:latest
```

Or with docker-compose, create a `docker-compose.override.yml`:

```yaml
services:
  ytvault:
    ports:
      - "9090:8080"
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `YTVAULT_ADDR` | `:8080` | Listen address |
| `YTVAULT_DATA_PATH` | `./data/state.json` | State file path |
| `YTVAULT_DOWNLOAD_PATH` | `./data/downloads` | Download directory |
| `YTVAULT_COOKIES_PATH` | _(empty)_ | Path to Netscape-format cookies file |

## Volumes

| Path | Description |
|------|-------------|
| `/data` | Persistent storage — state file, downloads, and cookies |

The `./data` directory on the host contains:
- `state.json` — application state (sources, media, tasks, schedules, config)
- `downloads/` — downloaded video files, organized by channel/playlist
- `cookies.txt` — YouTube cookies for authentication (optional)

## YouTube Cookies (Required for most videos)

YouTube blocks automated downloads without authentication. Export cookies from your browser:

1. Install a cookie export extension (e.g., [Get cookies.txt LOCALLY](https://chromewebstore.google.com/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc) for Chrome)
2. Go to youtube.com (logged in), export cookies
3. Save as `data/cookies.txt`
4. Restart the container: `docker compose restart`

Or upload cookies directly through the web UI (Cookies tab).

## Platforms

| Architecture | Support |
|---|---|
| `linux/amd64` | Linux servers, Intel Macs |
| `linux/arm64` | Apple Silicon Macs (Docker Desktop), Raspberry Pi, ARM servers |

## Advanced: Build from Source

```bash
git clone https://github.com/AC13139/ytvault.git
cd ytvault
docker compose up --build -d
```

## Links

- **GitHub**: [https://github.com/AC13139/ytvault](https://github.com/AC13139/ytvault)
- **Issues**: [https://github.com/AC13139/ytvault/issues](https://github.com/AC13139/ytvault/issues)
