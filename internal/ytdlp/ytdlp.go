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

// baseArgs returns common yt-dlp arguments (JS runtime, cookies).
func baseArgs() []string {
	args := []string{"--js-runtimes", "node"}
	if cp := cookiesPath(); cp != "" {
		args = append(args, "--cookies", cp)
	}
	return args
}

// downloadArgs returns baseArgs plus download-specific flags.
// Forces web player client for age-restricted video support.
func downloadArgs() []string {
	args := baseArgs()
	args = append(args, "--extractor-args", "youtube:player_client=web")
	return args
}

// VideoInfo represents a single video returned by yt-dlp.
type VideoInfo struct {
	ID         string
	Title      string
	URL        string
	Duration   int    // seconds
	UploadDate string // YYYYMMDD or empty
}

// probeResult matches the JSON output of `yt-dlp --flat-playlist -J`.
type probeResult struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Channel      string        `json:"channel"`
	Uploader     string        `json:"uploader"`
	WebpageURL   string        `json:"webpage_url"`
	URL          string        `json:"url"`
	Duration     *float64      `json:"duration"`    // null or number
	RawUploadDate string       `json:"upload_date"` // YYYYMMDD or empty
	Entries      []probeResult `json:"entries"`
}

func (p probeResult) channelName() string {
	if p.Channel != "" {
		return p.Channel
	}
	return p.Uploader
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
	// For channel URLs, target the /videos tab to avoid Shorts/Live
	indexURL := url
	if strings.Contains(url, "/@") || strings.Contains(url, "/channel/") || strings.Contains(url, "/c/") || strings.Contains(url, "/user/") {
		indexURL = strings.TrimRight(url, "/") + "/videos"
	}

	if dateAfter != "" {
		return indexWithDateFilter(indexURL, dateAfter)
	}
	return indexFlat(indexURL)
}

// indexFlat uses --flat-playlist -J (fast, no upload_date).
func indexFlat(url string) (string, []VideoInfo, error) {
	args := append(baseArgs(), "--flat-playlist", "-J", "--no-warnings", url)
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

	if len(root.Entries) == 0 {
		return root.Title, []VideoInfo{{
			ID: root.ID, Title: root.Title, URL: root.videoURL(), Duration: root.duration(), UploadDate: root.RawUploadDate,
		}}, nil
	}

	// Use channel/uploader name (clean) instead of root.Title which may have
	// suffixes like " - Videos" for channel tabs
	name := root.channelName()
	if name == "" {
		name = root.Title
	}

	videos := make([]VideoInfo, 0, len(root.Entries))
	var flatten func(entries []probeResult)
	flatten = func(entries []probeResult) {
		for _, e := range entries {
			if e.ID == "" {
				continue
			}
			if len(e.Entries) > 0 {
				flatten(e.Entries)
				continue
			}
			videos = append(videos, VideoInfo{
				ID: e.ID, Title: e.Title, URL: e.videoURL(), Duration: e.duration(), UploadDate: e.RawUploadDate,
			})
		}
	}
	flatten(root.Entries)
	return name, videos, nil
}

// SearchResult represents a single YouTube search result.
type SearchResult struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Channel  string `json:"channel"`
	Duration int    `json:"duration"`
}

// Search searches YouTube for the given query and returns up to limit results.
func Search(query string, limit int) ([]SearchResult, error) {
	if limit < 1 || limit > 20 {
		limit = 10
	}
	searchURL := fmt.Sprintf("ytsearch%d:%s", limit, query)
	args := append(baseArgs(), "--flat-playlist", "-J", "--no-warnings", searchURL)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("yt-dlp: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("yt-dlp exec: %w", err)
	}

	var root probeResult
	if err := json.Unmarshal(out, &root); err != nil {
		return nil, fmt.Errorf("parse yt-dlp output: %w", err)
	}

	var results []SearchResult
	for _, e := range root.Entries {
		if e.ID == "" {
			continue
		}
		results = append(results, SearchResult{
			ID:       e.ID,
			Title:    e.Title,
			URL:      e.videoURL(),
			Channel:  e.channelName(),
			Duration: e.duration(),
		})
	}
	return results, nil
}

// indexWithDateFilter uses --dump-json (per-line JSON, full extraction) with
// --dateafter and --break-on-reject so yt-dlp stops at the first video older
// than the cutoff. This is slower but required because --flat-playlist doesn't
// provide upload_date for --dateafter to filter.
func indexWithDateFilter(url, dateAfter string) (string, []VideoInfo, error) {
	args := append(baseArgs(),
		"--no-warnings",
		"--skip-download",
		"--print-json",
		"--ignore-errors",
		"--dateafter", dateAfter,
		"--break-on-reject",
		url,
	)
	cmd := exec.Command("yt-dlp", args...)
	out, err := cmd.CombinedOutput()
	// --break-on-reject causes a non-zero exit code; that's expected.
	// Only fail if there's no output at all.
	outStr := strings.TrimSpace(string(out))
	if outStr == "" && err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				return "", nil, fmt.Errorf("yt-dlp: %s", stderr)
			}
		}
		return "", nil, fmt.Errorf("yt-dlp exec: %w", err)
	}

	var videos []VideoInfo
	channelTitle := ""
	for _, line := range strings.Split(outStr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var entry probeResult
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.ID == "" {
			continue
		}
		if channelTitle == "" {
			// Use channel/uploader name (clean) from the first result
			channelTitle = entry.channelName()
			if channelTitle == "" {
				channelTitle = entry.Title
			}
		}
		videos = append(videos, VideoInfo{
			ID: entry.ID, Title: entry.Title, URL: entry.videoURL(), Duration: entry.duration(), UploadDate: entry.RawUploadDate,
		})
	}
	return channelTitle, videos, nil
}

// Download downloads a single video to outputDir using yt-dlp.
// filenameTemplate and videoFormat can be empty to use defaults.
// Returns the path of the downloaded file.
func Download(ctx context.Context, videoURL, outputDir, filenameTemplate, videoFormat string) (string, error) {
	if filenameTemplate == "" {
		filenameTemplate = "%(uploader,channel,id)s/%(title)s [%(id)s].%(ext)s"
	}
	if videoFormat == "" {
		videoFormat = "bestvideo[ext=mp4]+bestaudio[ext=m4a]/bestvideo+bestaudio/best"
	}
	template := filepath.Join(outputDir, filenameTemplate)
	args := append(downloadArgs(),
		"-f", videoFormat,
		"--merge-output-format", "mp4",
		"--no-warnings",
		"--print", "after_move:filepath",
		"-o", template,
	)
	args = append(args, videoURL)
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
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

	args := append(baseArgs(),
		"--cookies", cookiesFile,
		"--skip-download",
		"--flat-playlist",
		"-J",
		"--playlist-items", "1",
		"https://www.youtube.com/feed/history",
	)
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
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
