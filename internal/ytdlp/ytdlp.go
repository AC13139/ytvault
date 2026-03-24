// Package ytdlp wraps the yt-dlp CLI for indexing and downloading YouTube content.
package ytdlp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// cookiesPath returns the cookies file path from YTVAULT_COOKIES_PATH if set
// and the file exists.
func cookiesPath() string {
	p := os.Getenv("YTVAULT_COOKIES_PATH")
	if p == "" {
		return ""
	}
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		return ""
	}
	return p
}

// VideoInfo represents a single video returned by yt-dlp.
type VideoInfo struct {
	ID       string
	Title    string
	URL      string
	Duration int // seconds
}

// probeResult matches the JSON output of `yt-dlp --flat-playlist -J`.
type probeResult struct {
	ID         string        `json:"id"`
	Title      string        `json:"title"`
	WebpageURL string        `json:"webpage_url"`
	URL        string        `json:"url"`
	Duration   *float64      `json:"duration"` // null or number
	Entries    []probeResult `json:"entries"`
}

func (p probeResult) videoURL() string {
	if p.WebpageURL != "" {
		return p.WebpageURL
	}
	if p.URL != "" && strings.HasPrefix(p.URL, "http") {
		return p.URL
	}
	return fmt.Sprintf("https://www.youtube.com/watch?v=%s", p.ID)
}

func (p probeResult) duration() int {
	if p.Duration != nil {
		return int(*p.Duration)
	}
	return 0
}

// Index fetches the list of videos for a YouTube URL (channel, playlist, or
// single video) using `yt-dlp --flat-playlist -J`.
//
// dateAfter is an optional yt-dlp date filter (e.g. "now-30days", "now-365days",
// or "YYYYMMDD"). Pass an empty string to index all available videos.
//
// Returns the channel/playlist title and the list of videos found.
func Index(url, dateAfter string) (string, []VideoInfo, error) {
	args := []string{"--flat-playlist", "-J", "--no-warnings", "--skip-download", "--no-check-formats", "-f", "ba/b/w"}
	if cp := cookiesPath(); cp != "" {
		args = append(args, "--cookies", cp)
	}
	if dateAfter != "" {
		args = append(args, "--dateafter", dateAfter)
	}
	args = append(args, url)

	cmd := exec.Command("yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", nil, fmt.Errorf("yt-dlp: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", nil, fmt.Errorf("yt-dlp exec: %w", err)
	}

	var root probeResult
	if err := json.Unmarshal(out, &root); err != nil {
		return "", nil, fmt.Errorf("parse yt-dlp output: %w", err)
	}

	// Single video: no entries
	if len(root.Entries) == 0 {
		return root.Title, []VideoInfo{{
			ID:       root.ID,
			Title:    root.Title,
			URL:      root.videoURL(),
			Duration: root.duration(),
		}}, nil
	}

	// Playlist or channel — root.Title is the channel/playlist name
	videos := make([]VideoInfo, 0, len(root.Entries))
	for _, e := range root.Entries {
		if e.ID == "" {
			continue
		}
		videos = append(videos, VideoInfo{
			ID:       e.ID,
			Title:    e.Title,
			URL:      e.videoURL(),
			Duration: e.duration(),
		})
	}
	return root.Title, videos, nil
}

// Download downloads a single video to outputDir using yt-dlp.
// Returns the path of the downloaded file.
func Download(videoURL, outputDir string) (string, error) {
	template := filepath.Join(outputDir, "%(uploader,channel,id)s", "%(title)s [%(id)s].%(ext)s")
	args := []string{
		"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/bestvideo+bestaudio/best",
		"--merge-output-format", "mp4",
		"--no-warnings",
		"--print", "after_move:filepath",
		"-o", template,
	}
	if cp := cookiesPath(); cp != "" {
		args = append(args, "--cookies", cp)
	}
	args = append(args, videoURL)
	cmd := exec.Command("yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("yt-dlp: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("yt-dlp exec: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line, nil
		}
	}
	return "", fmt.Errorf("yt-dlp: no output filepath returned")
}

// ValidateCookies checks whether the cookies file at the given path allows
// authenticated YouTube access. It attempts to fetch the user's watch history,
// which requires valid authentication. Returns nil if valid, an error otherwise.
func ValidateCookies(cookiesFile string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--cookies", cookiesFile,
		"--skip-download",
		"--flat-playlist",
		"-J",
		"--playlist-items", "1",
		"https://www.youtube.com/feed/history",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("yt-dlp validation failed: %w", err)
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}
