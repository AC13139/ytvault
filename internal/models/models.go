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
)

type MediaItem struct {
	ID        string          `json:"id"`
	SourceID  string          `json:"sourceId"`
	VideoID   string          `json:"videoId"`
	Title     string          `json:"title"`
	URL       string          `json:"url"`
	Duration  int             `json:"duration,omitempty"`
	Status    MediaItemStatus `json:"status"`
	FilePath  string          `json:"filePath,omitempty"`
	ErrorMsg  string          `json:"errorMessage,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
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

type State struct {
	Sources    []Source    `json:"sources"`
	Tasks      []Task      `json:"tasks"`
	MediaItems []MediaItem `json:"mediaItems"`
	Schedules  []Schedule  `json:"schedules"`
}
