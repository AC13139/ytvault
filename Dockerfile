# ── Build stage ────────────────────────────────────────────────────────────────
FROM golang:1.26-bookworm AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /app/ytvault-api ./apps/api

# ── Runtime stage ──────────────────────────────────────────────────────────────
FROM python:3.12-slim

# ffmpeg: needed by yt-dlp to merge bestvideo+bestaudio streams
RUN apt-get update && \
    apt-get install -y --no-install-recommends ffmpeg wget && \
    pip install --no-cache-dir yt-dlp && \
    rm -rf /var/lib/apt/lists/*

# Run as a non-root user; pre-create /data with correct ownership
RUN useradd -r -u 1000 -m ytvault && \
    mkdir -p /data/downloads && \
    chown -R ytvault:ytvault /data

COPY --from=builder /app/ytvault-api /usr/local/bin/ytvault-api

ENV YTVAULT_ADDR=:8080
ENV YTVAULT_DATA_PATH=/data/state.json
ENV YTVAULT_DOWNLOAD_PATH=/data/downloads

VOLUME /data
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/healthz || exit 1

USER ytvault
CMD ["ytvault-api"]
