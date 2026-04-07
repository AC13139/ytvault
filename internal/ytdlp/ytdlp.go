// Package ytdlp wraps the yt-dlp CLI for indexing and downloading YouTube content.
package ytdlp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

// indexArgs returns baseArgs plus skip=authcheck for public content indexing.
// This prevents false "requires authentication" errors when cookies are
// visitor-only or expired. NOT used for ValidateCookies which needs authcheck.
func indexArgs() []string {
	args := baseArgs()
	args = append(args, "--extractor-args", "youtubetab:skip=authcheck")
	return args
}

// HasLoginCookies checks whether a cookie file contains YouTube login
// session cookies (SID, __Secure-1PSID, etc.) vs just visitor cookies.
func HasLoginCookies(cookiesFile string) bool {
	data, err := os.ReadFile(cookiesFile)
	if err != nil {
		return false
	}
	text := string(data)
	// These cookies are only set for logged-in YouTube sessions
	for _, name := range []string{"SID", "HSID", "SSID", "LOGIN_INFO", "__Secure-1PSID", "__Secure-3PSID"} {
		// Match tab-separated cookie name (Netscape format: domain\tTRUE\t...\tNAME\tVALUE)
		if strings.Contains(text, "\t"+name+"\t") {
			return true
		}
	}
	return false
}

// downloadArgs returns baseArgs plus download-specific flags.
// Forces web player client for age-restricted video support.
// Trims trailing whitespace from metadata fields to prevent filesystem issues.
func downloadArgs() []string {
	args := baseArgs()
	args = append(args,
		"--extractor-args", "youtube:player_client=web",
		// Trim trailing spaces from uploader/channel names (e.g. "王志安 " → "王志安")
		"--replace-in-metadata", "uploader,channel", "\\s+$", "",
	)
	return args
}

// VideoInfo represents a single video returned by yt-dlp.
type VideoInfo struct {
	ID         string
	Title      string
	URL        string
	Channel    string // channel/uploader name
	Duration   int    // seconds
	UploadDate string // YYYYMMDD or empty
}

// probeResult matches the JSON output of `yt-dlp --flat-playlist -J`.
type thumbnailEntry struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	ID     string `json:"id,omitempty"`
}

type probeResult struct {
	ID            string           `json:"id"`
	Title         string           `json:"title"`
	Channel       string           `json:"channel"`
	Uploader      string           `json:"uploader"`
	WebpageURL    string           `json:"webpage_url"`
	URL           string           `json:"url"`
	Duration      *float64         `json:"duration"`      // null or number
	RawUploadDate string           `json:"upload_date"`   // YYYYMMDD or empty
	Thumbnail     string           `json:"thumbnail"`     // best thumbnail URL
	Thumbnails    []thumbnailEntry `json:"thumbnails"`    // all available thumbnails
	Entries       []probeResult    `json:"entries"`
}

// bestPosterURL selects the best image for a source poster.
// For channels: avatar (square, ~900px). For playlists: playlist thumbnail.
// Falls back to the top-level thumbnail field.
func (p probeResult) bestPosterURL() string {
	// Look for avatar in thumbnails array (channels have "avatar_uncropped")
	for _, t := range p.Thumbnails {
		if strings.Contains(t.ID, "avatar") && t.URL != "" {
			return t.URL
		}
	}
	// Look for a square thumbnail (likely avatar, not banner)
	for _, t := range p.Thumbnails {
		if t.Width > 0 && t.Height > 0 && t.Width == t.Height && t.URL != "" {
			return t.URL
		}
	}
	// Fallback to top-level thumbnail
	if p.Thumbnail != "" {
		return p.Thumbnail
	}
	return ""
}

func (p probeResult) channelName() string {
	if p.Channel != "" {
		return strings.TrimSpace(p.Channel)
	}
	return strings.TrimSpace(p.Uploader)
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
// Returns the channel/playlist title, best poster URL, and the list of videos found.
func Index(url, dateAfter string) (string, string, []VideoInfo, error) {
	// For channel URLs, target the /videos tab to avoid Shorts/Live
	indexURL := url
	if strings.Contains(url, "/@") || strings.Contains(url, "/channel/") || strings.Contains(url, "/c/") || strings.Contains(url, "/user/") {
		indexURL = strings.TrimRight(url, "/") + "/videos"
	}

	if dateAfter != "" {
		title, posterURL, videos, err := indexWithDateFilter(indexURL, dateAfter)
		if err == nil && len(videos) > 0 {
			return title, posterURL, videos, nil
		}
		if err != nil {
			log.Printf("ytdlp: full extraction failed (%v), falling back to flat mode", err)
		} else {
			log.Printf("ytdlp: full extraction returned 0 videos, falling back to flat mode")
		}
		return indexFlatWithLimit(indexURL, dateAfter)
	}
	return indexFlat(indexURL)
}

// indexFlat uses --flat-playlist -J (fast, no upload_date).
func indexFlat(url string) (string, string, []VideoInfo, error) {
	args := append(indexArgs(), "--flat-playlist", "-J", "--no-warnings", url)
	cmd := exec.Command("yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", "", nil, fmt.Errorf("yt-dlp: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", "", nil, fmt.Errorf("yt-dlp exec: %w", err)
	}

	var root probeResult
	if err := json.Unmarshal(out, &root); err != nil {
		return "", "", nil, fmt.Errorf("parse yt-dlp output: %w", err)
	}

	posterURL := root.bestPosterURL()

	if len(root.Entries) == 0 {
		return root.Title, posterURL, []VideoInfo{{
			ID: root.ID, Title: root.Title, URL: root.videoURL(), Channel: root.channelName(), Duration: root.duration(), UploadDate: root.RawUploadDate,
		}}, nil
	}

	// Use channel/uploader name (clean) instead of root.Title which may have
	// suffixes like " - Videos" for channel tabs
	// For playlists, root.Title is the playlist name (what we want).
	// For channels, root.channelName() is cleaner (no " - Videos" suffix).
	// Detect playlists by checking if Title doesn't end with " - Videos"/etc.
	name := strings.TrimSpace(root.Title)
	if ch := root.channelName(); ch != "" && (name == "" || strings.HasSuffix(name, " - Videos") || strings.HasSuffix(name, " - Shorts") || strings.HasSuffix(name, " - Live")) {
		name = ch
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
				ID: e.ID, Title: e.Title, URL: e.videoURL(), Channel: e.channelName(), Duration: e.duration(), UploadDate: e.RawUploadDate,
			})
		}
	}
	flatten(root.Entries)
	return name, posterURL, videos, nil
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

// dateFilterToLimit maps a yt-dlp dateAfter value to a max video count.
// YouTube lists newest first, so limiting to N videos approximates "last X days".
// dateFilterToLimit maps date ranges to video counts for flat mode fallback.
// Assumes ~1-2 videos per week for most channels.
func dateFilterToLimit(dateAfter string) int {
	switch dateAfter {
	case "now-1days":
		return 3
	case "now-3days":
		return 5
	case "now-7days":
		return 10
	case "now-14days":
		return 15
	case "now-30days":
		return 20
	case "now-90days":
		return 50
	case "now-365days":
		return 120
	case "now-730days":
		return 250
	default:
		return 50
	}
}

// indexFlatWithLimit uses --flat-playlist (fast, reliable) with --playlist-items
// to limit by recency. Since full extraction (--print-json) is unreliable in
// Docker environments, we use flat mode and approximate date filtering by
// video count (YouTube lists newest first).
func indexFlatWithLimit(url, dateAfter string) (string, string, []VideoInfo, error) {
	limit := dateFilterToLimit(dateAfter)
	args := append(indexArgs(),
		"--flat-playlist", "-J", "--no-warnings",
		"--playlist-items", fmt.Sprintf("1:%d", limit),
		url,
	)
	cmd := exec.Command("yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", "", nil, fmt.Errorf("yt-dlp: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", "", nil, fmt.Errorf("yt-dlp exec: %w", err)
	}

	var root probeResult
	if err := json.Unmarshal(out, &root); err != nil {
		return "", "", nil, fmt.Errorf("parse yt-dlp output: %w", err)
	}

	// For playlists, root.Title is the playlist name (what we want).
	// For channels, root.channelName() is cleaner (no " - Videos" suffix).
	// Detect playlists by checking if Title doesn't end with " - Videos"/etc.
	name := strings.TrimSpace(root.Title)
	if ch := root.channelName(); ch != "" && (name == "" || strings.HasSuffix(name, " - Videos") || strings.HasSuffix(name, " - Shorts") || strings.HasSuffix(name, " - Live")) {
		name = ch
	}
	posterURL := root.bestPosterURL()

	if len(root.Entries) == 0 {
		return name, posterURL, []VideoInfo{{
			ID: root.ID, Title: root.Title, URL: root.videoURL(), Channel: root.channelName(), Duration: root.duration(), UploadDate: root.RawUploadDate,
		}}, nil
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
				ID: e.ID, Title: e.Title, URL: e.videoURL(), Channel: e.channelName(), Duration: e.duration(), UploadDate: e.RawUploadDate,
			})
		}
	}
	flatten(root.Entries)
	return name, posterURL, videos, nil
}

// FetchPosterURL fetches only the poster/avatar URL for a source without
// indexing all videos. Much faster than Index() for poster-only fetches.
// FetchPosterURL fetches only the poster/avatar URL for a source without
// indexing all videos. Much faster than Index() for poster-only fetches.
func FetchPosterURL(url string) (string, error) {
	// For channels, target the /videos tab to get channel-level metadata
	if strings.Contains(url, "/@") || strings.Contains(url, "/channel/") || strings.Contains(url, "/c/") || strings.Contains(url, "/user/") {
		url = strings.TrimRight(url, "/") + "/videos"
	}
	args := append(indexArgs(),
		"--flat-playlist", "-J", "--no-warnings",
		"--playlist-items", "1:1", // just 1 entry — fast, but still returns root metadata
		url,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("yt-dlp: %w", err)
	}
	var root probeResult
	if err := json.Unmarshal(out, &root); err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	poster := root.bestPosterURL()
	// For playlists/channels without avatar, use the best available thumbnail
	if poster == "" && len(root.Thumbnails) > 0 {
		// Pick the largest thumbnail (last in the array, which is typically highest res)
		poster = root.Thumbnails[len(root.Thumbnails)-1].URL
	}
	return poster, nil
}

// indexWithDateFilter uses --dump-json (per-line JSON, full extraction) with
// --dateafter and --break-on-reject so yt-dlp stops at the first video older
// than the cutoff. This is slower but required because --flat-playlist doesn't
// provide upload_date for --dateafter to filter.
func indexWithDateFilter(url, dateAfter string) (string, string, []VideoInfo, error) {
	args := append(indexArgs(),
		"--no-warnings",
		"--skip-download",
		"--print-json",
		"--ignore-errors",
		"--dateafter", dateAfter,
		"--break-on-reject",
		"--playlist-items", "1:100", // cap at 100 videos to avoid 20+ min indexing
		url,
	)
	// 5 minute timeout — full extraction is slow but shouldn't take forever
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	out, err := cmd.CombinedOutput()
	// --break-on-reject causes a non-zero exit code; that's expected.
	// Only fail if there's no output at all.
	outStr := strings.TrimSpace(string(out))
	if outStr == "" && err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				return "", "", nil, fmt.Errorf("yt-dlp: %s", stderr)
			}
		}
		return "", "", nil, fmt.Errorf("yt-dlp exec: %w", err)
	}

	var videos []VideoInfo
	channelTitle := ""
	posterURL := ""
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
			channelTitle = entry.channelName()
			if channelTitle == "" {
				channelTitle = entry.Title
			}
			// Don't use per-video thumbnail as source poster — it's a video
			// frame, not the channel/playlist avatar. Poster URL stays empty
			// for the dateAfter path; the flat mode fallback will get it.
		}
		videos = append(videos, VideoInfo{
			ID: entry.ID, Title: entry.Title, URL: entry.videoURL(), Channel: entry.channelName(), Duration: entry.duration(), UploadDate: entry.RawUploadDate,
		})
	}
	return channelTitle, posterURL, videos, nil
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
