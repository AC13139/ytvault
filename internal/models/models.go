package models

import "time"

type SourceKind string

const (
	SourceKindChannel  SourceKind = "channel"
	SourceKindPlaylist SourceKind = "playlist"
	SourceKindVideo    SourceKind = "video"
)

type SourceStatus string

const (
	SourceStatusActive SourceStatus = "active"
	SourceStatusPaused SourceStatus = "paused"
)

type Source struct {
	ID                 string       `json:"id"`
	Kind               SourceKind   `json:"kind"`
	URL                string       `json:"url"`
	InputURL           string       `json:"inputUrl,omitempty"`
	Title              string       `json:"title"`
	Locator            string       `json:"locator,omitempty"`
	Host               string       `json:"host,omitempty"`
	PathKind           string       `json:"pathKind,omitempty"`
	PathLabel          string       `json:"pathLabel,omitempty"`
	IntakeStage        string       `json:"intakeStage,omitempty"`
	NextAction         string       `json:"nextAction,omitempty"`
	ValidationSummary  string       `json:"validationSummary,omitempty"`
	PrototypeReadiness string       `json:"prototypeReadiness,omitempty"`
	RetentionDays      int          `json:"retentionDays,omitempty"` // 0 = keep forever
	Status             SourceStatus `json:"status"`
	CreatedAt          time.Time    `json:"createdAt"`
	UpdatedAt          time.Time    `json:"updatedAt"`
}

type TaskStatus string

const (
	TaskStatusQueued    TaskStatus = "queued"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusSucceeded TaskStatus = "succeeded"
	TaskStatusFailed    TaskStatus = "failed"
)

type Task struct {
	ID           string     `json:"id"`
	Type         string     `json:"type"`
	EntityType   string     `json:"entityType"`
	EntityID     string     `json:"entityId"`
	Status       TaskStatus `json:"status"`
	AutoDownload bool       `json:"autoDownload,omitempty"` // if true, worker auto-downloads after index
	DateAfter    string     `json:"dateAfter,omitempty"`    // yt-dlp --dateafter value, e.g. "now-30days"
	ErrorCode    string     `json:"errorCode,omitempty"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
}

type MediaItemStatus string

const (
	MediaItemStatusPending     MediaItemStatus = "pending"
	MediaItemStatusDownloading MediaItemStatus = "downloading"
	MediaItemStatusDone        MediaItemStatus = "done"
	MediaItemStatusFailed      MediaItemStatus = "failed"
	MediaItemStatusMissing     MediaItemStatus = "missing"
)

type MediaItem struct {
	ID            string          `json:"id"`
	SourceID      string          `json:"sourceId"`
	VideoID       string          `json:"videoId"`
	Title         string          `json:"title"`
	URL           string          `json:"url"`
	Duration      int             `json:"duration,omitempty"`
	PlaylistIndex int             `json:"playlistIndex,omitempty"` // 1-based index within playlist
	UploadDate    *time.Time      `json:"uploadDate,omitempty"`    // YouTube upload date (nil = unknown)
	Pinned        bool            `json:"pinned,omitempty"`        // pinned items are never auto-cleaned
	Status     MediaItemStatus `json:"status"`
	FilePath   string          `json:"filePath,omitempty"`
	FileSize   int64           `json:"fileSize,omitempty"` // bytes
	ErrorMsg   string          `json:"errorMessage,omitempty"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
}

// Schedule defines a recurring re-index (and optional auto-download) for a source.
type Schedule struct {
	SourceID      string     `json:"sourceId"`
	IntervalHours int        `json:"intervalHours"` // 1, 6, 12, 24, 168
	Enabled       bool       `json:"enabled"`
	AutoDownload  bool       `json:"autoDownload"`          // if true, new videos are downloaded automatically
	DateAfter     string     `json:"dateAfter,omitempty"`   // yt-dlp --dateafter value, e.g. "now-30days"
	LastRunAt     *time.Time `json:"lastRunAt,omitempty"`
	NextRunAt     time.Time  `json:"nextRunAt"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// Config stores user-configurable settings with sensible defaults.
type Config struct {
	FolderFormat          string `json:"folderFormat,omitempty"`          // folder naming preset key
	FileFormat            string `json:"fileFormat,omitempty"`            // file naming preset key
	PlaylistFolderOverride bool   `json:"playlistFolderOverride,omitempty"` // use playlist name as folder for playlist sources
	PlaylistFileOverride   bool   `json:"playlistFileOverride,omitempty"`   // use playlist index+title for playlist sources
	FilenameTemplate      string `json:"filenameTemplate,omitempty"`      // resolved yt-dlp output template
	PlaylistTemplate      string `json:"playlistTemplate,omitempty"`      // resolved template for playlist sources
	VideoFormat           string `json:"videoFormat,omitempty"`           // yt-dlp -f value
	MaxConcurrent         int    `json:"maxConcurrent,omitempty"`         // max parallel downloads
	DefaultRetentionDays  int    `json:"defaultRetentionDays,omitempty"`  // global default, 0 = keep forever
}

type State struct {
	Sources    []Source    `json:"sources"`
	Tasks      []Task      `json:"tasks"`
	MediaItems []MediaItem `json:"mediaItems"`
	Schedules  []Schedule  `json:"schedules"`
	Config     *Config     `json:"config,omitempty"`
}
