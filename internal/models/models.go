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
	ErrorCode    string     `json:"errorCode,omitempty"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
}

type State struct {
	Sources []Source `json:"sources"`
	Tasks   []Task   `json:"tasks"`
}
