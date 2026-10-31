package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ytvault/internal/models"
	"ytvault/internal/scheduler"
	"ytvault/internal/store"
	"ytvault/internal/worker"
	"ytvault/internal/yt"
	"ytvault/internal/ytdlp"
)

type App struct {
	store       *store.Store
	worker      *worker.Worker
	scheduler   *scheduler.Scheduler
	cookiesPath string
	downloadDir string
	version     string
	commit      string

	// Cached cookie validation result
	cookiesMu         sync.Mutex
	cookiesValid      *bool // nil = not tested
	cookiesValidMsg   string
	cookiesValidAt    time.Time
	cookiesValidMtime time.Time
}

func NewApp(dataPath, downloadDir, cookiesPath, version, commit string) (*App, error) {
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return nil, fmt.Errorf("create download dir %s: %w", downloadDir, err)
	}
	s, err := store.New(dataPath)
	if err != nil {
		return nil, err
	}
	w := worker.New(s, downloadDir)
	sc := scheduler.New(s, w)
	sc.Start()
	return &App{
		store:       s,
		worker:      w,
		scheduler:   sc,
		cookiesPath: cookiesPath,
		downloadDir: downloadDir,
		version:     version,
		commit:      commit,
	}, nil
}

// Close stops background goroutines. Call on server shutdown.
func (a *App) Close() {
	a.scheduler.Stop()
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.handleHealth)
	mux.HandleFunc("GET /api/v1/sources", a.handleListSources)
	mux.HandleFunc("POST /api/v1/sources", a.handleCreateSource)
	mux.HandleFunc("GET /api/v1/sources/{id}/media", a.handleListSourceMedia)
	mux.HandleFunc("POST /api/v1/sources/{id}/download-all", a.handleDownloadAll)
	mux.HandleFunc("GET /api/v1/sources/{id}/poster", a.handleGetPoster)
	mux.HandleFunc("POST /api/v1/sources/{id}/poster", a.handleSetPoster)
	mux.HandleFunc("DELETE /api/v1/sources/{id}/poster", a.handleDeletePoster)
	mux.HandleFunc("POST /api/v1/sources/{id}/fetch-poster", a.handleFetchPoster)
	mux.HandleFunc("POST /api/v1/sources/{id}/reindex", a.handleReindex)
	mux.HandleFunc("GET /api/v1/images/search", a.handleImageSearch)
	mux.HandleFunc("GET /api/v1/sources/{id}/schedule", a.handleGetSchedule)
	mux.HandleFunc("POST /api/v1/sources/{id}/schedule", a.handleSetSchedule)
	mux.HandleFunc("DELETE /api/v1/sources/{id}/schedule", a.handleDeleteSchedule)
	mux.HandleFunc("GET /api/v1/schedules", a.handleListSchedules)
	mux.HandleFunc("GET /api/v1/tasks", a.handleListTasks)
	mux.HandleFunc("POST /api/v1/tasks/{id}/run", a.handleRunTask)
	mux.HandleFunc("POST /api/v1/tasks/{id}/start", a.handleStartTask)
	mux.HandleFunc("DELETE /api/v1/sources/{id}", a.handleDeleteSource)
	mux.HandleFunc("DELETE /api/v1/media/{id}", a.handleDeleteMedia)
	mux.HandleFunc("POST /api/v1/media/{id}/download", a.handleDownloadMedia)
	mux.HandleFunc("POST /api/v1/media/{id}/pin", a.handleTogglePin)
	mux.HandleFunc("POST /api/v1/media/{id}/retry", a.handleRetryMedia)
	mux.HandleFunc("GET /api/v1/sources/{id}/cleanup-preview", a.handleCleanupPreview)
	mux.HandleFunc("POST /api/v1/sources/{id}/cleanup", a.handleCleanup)
	mux.HandleFunc("PUT /api/v1/sources/{id}/retention", a.handleSetRetention)
	mux.HandleFunc("POST /api/v1/media/download-all-pending", a.handleDownloadAllPending)
	mux.HandleFunc("POST /api/v1/tasks/{id}/execute", a.handleExecuteTask)
	mux.HandleFunc("POST /api/v1/tasks/{id}/cancel", a.handleCancelTask)
	mux.HandleFunc("POST /api/v1/tasks/{id}/retry", a.handleRetryTask)
	mux.HandleFunc("DELETE /api/v1/tasks/{id}", a.handleDeleteTask)
	mux.HandleFunc("DELETE /api/v1/tasks", a.handleDeleteCompletedTasks)
	mux.HandleFunc("POST /api/v1/tasks/stop-all", a.handleStopAllRunning)
	mux.HandleFunc("POST /api/v1/tasks/retry-failed", a.handleRetryAllFailed)
	mux.HandleFunc("GET /api/v1/config", a.handleGetConfig)
	mux.HandleFunc("PUT /api/v1/config", a.handleUpdateConfig)
	mux.HandleFunc("GET /api/v1/health-summary", a.handleHealthSummary)
	mux.HandleFunc("GET /api/v1/version", a.handleVersion)
	mux.HandleFunc("GET /api/v1/youtube/search", a.handleYouTubeSearch)
	mux.HandleFunc("GET /api/v1/youtube/probe", a.handleYouTubeProbe)
	mux.HandleFunc("GET /api/v1/collections", a.handleListCollections)
	mux.HandleFunc("POST /api/v1/collections", a.handleCreateCollection)
	mux.HandleFunc("POST /api/v1/collections/{id}/items", a.handleAddToCollection)
	mux.HandleFunc("GET /api/v1/cookies/status", a.handleCookiesStatus)
	mux.HandleFunc("POST /api/v1/cookies", a.handleCookiesUpload)
	mux.HandleFunc("DELETE /api/v1/cookies", a.handleCookiesDelete)
	mux.HandleFunc("POST /api/v1/cookies/validate", a.handleCookiesValidate)
	mux.HandleFunc("GET /", a.handleIndex)
	return withSecurityHeaders(withJSONLogging(mux))
}

// ── Source handlers ───────────────────────────────────────────────────────────

type createSourceRequest struct {
	Kind           string `json:"kind"`
	URL            string `json:"url"`
	DateAfter      string `json:"dateAfter,omitempty"`
	AutoDownload   bool   `json:"autoDownload,omitempty"`
	CollectionID   string `json:"collectionId,omitempty"`   // assign video to existing collection
	CollectionName string `json:"collectionName,omitempty"` // create new collection (when collectionId == "__new__")
}

type createSourceResponse struct {
	Source models.Source `json:"source"`
	Task   models.Task   `json:"task"`
}

func (a *App) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"service":   "ytvault-api",
		"timestamp": time.Now().UTC(),
	})
}

func (a *App) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version": a.version,
		"commit":  a.commit,
	})
}

func (a *App) handleListSources(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": a.store.ListSources()})
}

func (a *App) handleCreateSource(w http.ResponseWriter, r *http.Request) {
	var req createSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	// If a video is being added to a collection, route to collection flow
	kind := models.SourceKind(req.Kind)
	if kind == models.SourceKindVideo && req.CollectionID != "" {
		a.addVideoToCollectionFromCreate(w, req)
		return
	}

	profile, err := yt.NormalizeSource(req.URL, kind)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_source", err.Error())
		return
	}
	src, task, err := a.store.CreateSource(kind, profile, req.DateAfter, req.AutoDownload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, createSourceResponse{Source: src, Task: task})
}

// addVideoToCollectionFromCreate handles the "add video to collection" path
// from the create source endpoint.
func (a *App) addVideoToCollectionFromCreate(w http.ResponseWriter, req createSourceRequest) {
	// Create new collection if requested
	collectionID := req.CollectionID
	if collectionID == "__new__" {
		if req.CollectionName == "" {
			writeError(w, http.StatusBadRequest, "missing_name", "collection name is required")
			return
		}
		col, err := a.store.CreateCollection(req.CollectionName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
			return
		}
		collectionID = col.ID
	}

	// Normalize the video URL to get canonical video ID
	profile, err := yt.NormalizeSource(req.URL, models.SourceKindVideo)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_url", err.Error())
		return
	}

	// Synchronous probe: index the single video to get metadata
	_, _, videos, err := ytdlp.Index(profile.NormalizedURL, "")
	if err != nil {
		writeError(w, http.StatusBadGateway, "index_failed", "failed to fetch video info: "+err.Error())
		return
	}
	if len(videos) == 0 {
		writeError(w, http.StatusBadGateway, "index_failed", "no video info returned")
		return
	}
	v := videos[0]

	// Parse upload date
	var uploadDate *time.Time
	if v.UploadDate != "" {
		if t, err := time.Parse("20060102", v.UploadDate); err == nil {
			uploadDate = &t
		}
	}

	item, task, err := a.store.AddVideoToCollection(collectionID, v.ID, v.Title, v.URL, v.Channel, v.Duration, uploadDate, req.AutoDownload)
	if err != nil {
		writeError(w, http.StatusConflict, "add_failed", err.Error())
		return
	}

	// Auto-start download task if created
	if task != nil {
		a.worker.StartTask(task.ID)
	}

	col, _ := a.store.GetSource(collectionID)
	resp := map[string]any{
		"source": col,
		"media":  item,
	}
	if task != nil {
		resp["task"] = *task
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (a *App) handleListSourceMedia(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := a.store.GetSource(id); !ok {
		writeError(w, http.StatusNotFound, "source_not_found", fmt.Sprintf("source %s not found", id))
		return
	}
	items := a.store.ListMediaItemsBySource(id)
	if items == nil {
		items = []models.MediaItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *App) handleDownloadAll(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tasks, err := a.store.CreateDownloadTasksForSource(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "create_tasks_failed", err.Error())
		return
	}
	if len(tasks) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"tasks": []any{}, "count": 0, "started": 0})
		return
	}

	started := 0
	var startErrs []string
	for _, t := range tasks {
		if _, err := a.worker.StartTask(t.ID); err != nil {
			startErrs = append(startErrs, fmt.Sprintf("task %s: %v", t.ID, err))
		} else {
			started++
		}
	}

	resp := map[string]any{"tasks": tasks, "count": len(tasks), "started": started}
	if len(startErrs) > 0 {
		resp["startErrors"] = startErrs
	}
	writeJSON(w, http.StatusAccepted, resp)
}

func (a *App) handleReindex(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		DateAfter string `json:"dateAfter"`
	}
	// Body is optional; ignore parse errors
	_ = json.NewDecoder(r.Body).Decode(&req)

	task, err := a.store.CreateIndexTask(id, false, req.DateAfter)
	if err != nil {
		writeError(w, http.StatusConflict, "create_failed", err.Error())
		return
	}
	started, err := a.worker.StartTask(task.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "start_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"task": started})
}

// ── Collection handlers ──────────────────────────────────────────────────────

func (a *App) handleListCollections(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": a.store.ListCollections()})
}

func (a *App) handleCreateCollection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "name is required")
		return
	}
	col, err := a.store.CreateCollection(req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"source": col})
}

func (a *App) handleAddToCollection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		URL          string `json:"url"`
		AutoDownload bool   `json:"autoDownload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "missing_url", "url is required")
		return
	}

	// Normalize URL to canonical video ID
	profile, err := yt.NormalizeSource(req.URL, models.SourceKindVideo)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_url", err.Error())
		return
	}

	// Synchronous probe
	_, _, videos, err := ytdlp.Index(profile.NormalizedURL, "")
	if err != nil {
		writeError(w, http.StatusBadGateway, "index_failed", "failed to fetch video info: "+err.Error())
		return
	}
	if len(videos) == 0 {
		writeError(w, http.StatusBadGateway, "index_failed", "no video info returned")
		return
	}
	v := videos[0]

	var uploadDate *time.Time
	if v.UploadDate != "" {
		if t, err := time.Parse("20060102", v.UploadDate); err == nil {
			uploadDate = &t
		}
	}

	item, task, err := a.store.AddVideoToCollection(id, v.ID, v.Title, v.URL, v.Channel, v.Duration, uploadDate, req.AutoDownload)
	if err != nil {
		writeError(w, http.StatusConflict, "add_failed", err.Error())
		return
	}

	if task != nil {
		a.worker.StartTask(task.ID)
	}

	resp := map[string]any{"media": item}
	if task != nil {
		resp["task"] = *task
	}
	writeJSON(w, http.StatusCreated, resp)
}

// ── Schedule handlers ─────────────────────────────────────────────────────────

type setScheduleRequest struct {
	IntervalHours int    `json:"intervalHours"`
	Enabled       bool   `json:"enabled"`
	AutoDownload  bool   `json:"autoDownload"`
	DateAfter     string `json:"dateAfter,omitempty"`
}

func (a *App) handleListSchedules(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": a.store.ListSchedules()})
}

func (a *App) handleGetSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sched, ok := a.store.GetSchedule(id)
	if !ok {
		writeError(w, http.StatusNotFound, "schedule_not_found", fmt.Sprintf("no schedule for source %s", id))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"schedule": sched})
}

func (a *App) handleSetSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Collections cannot be scheduled (manually curated)
	if src, ok := a.store.GetSource(id); ok && src.Kind == models.SourceKindCollection {
		writeError(w, http.StatusBadRequest, "not_schedulable", "collections cannot be scheduled — add videos manually")
		return
	}
	var req setScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if req.IntervalHours < 1 {
		writeError(w, http.StatusBadRequest, "invalid_interval", "intervalHours must be >= 1")
		return
	}
	sched, err := a.store.UpsertSchedule(id, req.IntervalHours, req.Enabled, req.AutoDownload, req.DateAfter)
	if err != nil {
		writeError(w, http.StatusBadRequest, "schedule_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"schedule": sched})
}

func (a *App) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.store.DeleteSchedule(id); err != nil {
		writeError(w, http.StatusNotFound, "schedule_not_found", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Task handlers ─────────────────────────────────────────────────────────────

func (a *App) handleListTasks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": a.store.ListTasks()})
}

// handleRunTask is the legacy stub endpoint kept for backward compatibility.
func (a *App) handleRunTask(w http.ResponseWriter, r *http.Request) {
	task, err := a.store.RunTask(r.PathValue("id"))
	if err != nil {
		if store.IsTaskNotRunnable(err) {
			writeError(w, http.StatusConflict, "task_not_runnable", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "run_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": task})
}

// handleStartTask starts real execution of a task via the worker (async).
func (a *App) handleStartTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, err := a.worker.StartTask(id)
	if err != nil {
		if store.IsTaskNotRunnable(err) {
			writeError(w, http.StatusConflict, "task_not_runnable", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "start_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"task": task})
}

type executeTaskRequest struct {
	Mode string `json:"mode"`
}

func (a *App) handleExecuteTask(w http.ResponseWriter, r *http.Request) {
	var req executeTaskRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
	}

	mode := req.Mode
	if mode == "" {
		mode = "stub"
	}

	var (
		task models.Task
		err  error
	)
	if mode == "fail_stub" {
		task, err = a.store.FailTaskStub(r.PathValue("id"), "FAIL_STUB", "stubbed execution failure triggered by request mode=fail_stub")
	} else {
		task, err = a.store.RunTask(r.PathValue("id"))
	}
	if err != nil {
		if store.IsTaskNotRunnable(err) {
			writeError(w, http.StatusConflict, "task_not_runnable", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "execute_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"task": task,
		"execution": map[string]any{
			"action":  "execute",
			"handler": "index-task-execute-skeleton",
			"mode":    mode,
			"note":    "Execution skeleton is wired, but real index work is not connected yet.",
		},
	})
}

func (a *App) handleYouTubeSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing_query", "q parameter is required")
		return
	}
	limit := 10
	results, err := ytdlp.Search(q, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (a *App) handleHealthSummary(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.store.HealthSummary())
}

func (a *App) handleYouTubeProbe(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		writeError(w, http.StatusBadRequest, "missing_url", "url parameter required")
		return
	}
	dateAfter := r.URL.Query().Get("dateAfter")
	title, _, videos, err := ytdlp.Index(url, dateAfter)
	resp := map[string]any{
		"url":       url,
		"dateAfter": dateAfter,
		"title":     title,
		"count":     len(videos),
	}
	if err != nil {
		resp["error"] = err.Error()
	}
	if len(videos) > 0 && len(videos) <= 10 {
		resp["videos"] = videos
	} else if len(videos) > 10 {
		resp["videos"] = videos[:10]
		resp["truncated"] = true
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := a.store.GetConfig()
	writeJSON(w, http.StatusOK, map[string]any{
		"config":      cfg,
		"downloadDir": a.downloadDir,
	})
}

func (a *App) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var cfg models.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := a.store.UpdateConfig(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "save_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": a.store.GetConfig()})
}

func (a *App) handleTogglePin(w http.ResponseWriter, r *http.Request) {
	pinned, err := a.store.TogglePin(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "pin_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pinned": pinned})
}

// resolveRetention returns the effective retention days for a source.
// 0 = use global default, -1 = keep forever (override), >0 = specific days.
func (a *App) resolveRetention(src models.Source) int {
	if src.RetentionDays == -1 {
		return 0 // keep forever — no cleanup
	}
	if src.RetentionDays > 0 {
		return src.RetentionDays
	}
	return a.store.GetConfig().DefaultRetentionDays
}

func (a *App) handleCleanupPreview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	src, ok := a.store.GetSource(id)
	if !ok {
		writeError(w, http.StatusNotFound, "source_not_found", "source not found")
		return
	}
	days := a.resolveRetention(src)
	if days == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"eligible": 0, "pinned": 0, "retentionDays": 0})
		return
	}
	eligible, pinned := a.store.CleanupPreview(id, days)
	writeJSON(w, http.StatusOK, map[string]any{"eligible": eligible, "pinned": pinned, "retentionDays": days})
}

func (a *App) handleCleanup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	src, ok := a.store.GetSource(id)
	if !ok {
		writeError(w, http.StatusNotFound, "source_not_found", "source not found")
		return
	}
	days := a.resolveRetention(src)
	if days == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"removed": 0, "message": "no retention policy set"})
		return
	}
	filePaths, removed := a.store.CleanupSource(id, days)
	for _, fp := range filePaths {
		os.Remove(fp)
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
}

func (a *App) handleSetRetention(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		RetentionDays int `json:"retentionDays"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := a.store.UpdateSourceRetention(id, req.RetentionDays); err != nil {
		writeError(w, http.StatusNotFound, "update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"retentionDays": req.RetentionDays})
}

func (a *App) handleDownloadMedia(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Reset missing items to pending before creating download task
	if item, ok := a.store.GetMediaItem(id); ok && item.Status == models.MediaItemStatusMissing {
		_ = a.store.RetryMediaItem(id)
	}
	task, err := a.store.CreateDownloadTask(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "download_failed", err.Error())
		return
	}
	started, err := a.worker.StartTask(task.ID)
	if err != nil {
		writeJSON(w, http.StatusAccepted, map[string]any{"task": task})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"task": started})
}

func (a *App) handleDownloadAllPending(w http.ResponseWriter, r *http.Request) {
	sourceFilter := r.URL.Query().Get("source")
	sources := a.store.ListSources()
	started := 0
	total := 0

	// First: retry all failed/missing media items (reset to pending)
	for _, src := range sources {
		if sourceFilter != "" && src.ID != sourceFilter {
			continue
		}
		for _, m := range a.store.ListMediaItemsBySource(src.ID) {
			if m.Status == models.MediaItemStatusFailed || m.Status == models.MediaItemStatusMissing {
				_ = a.store.RetryMediaItem(m.ID)
			}
		}
	}

	// Then: create download tasks for all pending items
	for _, src := range sources {
		if sourceFilter != "" && src.ID != sourceFilter {
			continue
		}
		tasks, err := a.store.CreateDownloadTasksForSource(src.ID)
		if err != nil {
			continue
		}
		total += len(tasks)
		for _, t := range tasks {
			if _, err := a.worker.StartTask(t.ID); err == nil {
				started++
			}
		}
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"total": total, "started": started})
}

func (a *App) handleDeleteMedia(w http.ResponseWriter, r *http.Request) {
	deleted, err := a.store.DeleteMediaItem(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "delete_failed", err.Error())
		return
	}
	// Delete the physical file if it exists
	if deleted.FilePath != "" {
		os.Remove(deleted.FilePath)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleRetryMedia(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.store.RetryMediaItem(id); err != nil {
		writeError(w, http.StatusBadRequest, "retry_failed", err.Error())
		return
	}
	// Create and start a download task
	task, err := a.store.CreateDownloadTask(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "task_failed", err.Error())
		return
	}
	started, err := a.worker.StartTask(task.ID)
	if err != nil {
		writeJSON(w, http.StatusAccepted, map[string]any{"task": task, "message": "retrying but could not auto-start"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"task": started})
}

func (a *App) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.worker.CancelTask(id); err != nil {
		writeError(w, http.StatusBadRequest, "cancel_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "task cancellation requested"})
}

func (a *App) handleGetPoster(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	src, ok := a.store.GetSource(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if src.PosterPath == "" {
		// For videos without a poster, redirect to YouTube thumbnail
		if src.Kind == models.SourceKindVideo && src.Locator != "" {
			http.Redirect(w, r, "https://img.youtube.com/vi/"+src.Locator+"/mqdefault.jpg", http.StatusTemporaryRedirect)
			return
		}
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, src.PosterPath)
}

func (a *App) handleSetPoster(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	src, ok := a.store.GetSource(id)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "source not found")
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		writeError(w, http.StatusBadRequest, "invalid", "url is required")
		return
	}
	if !strings.HasPrefix(req.URL, "https://") {
		writeError(w, http.StatusBadRequest, "invalid", "url must be https")
		return
	}
	// SSRF protection: reject private/loopback/metadata URLs
	for _, blocked := range []string{"localhost", "127.", "10.", "172.16.", "192.168.", "169.254.", "[::1]", "metadata"} {
		if strings.Contains(strings.ToLower(req.URL), blocked) {
			writeError(w, http.StatusBadRequest, "invalid", "URL points to a private/internal address")
			return
		}
	}
	posterDir := filepath.Join(a.downloadDir, ".posters")
	if err := os.MkdirAll(posterDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "write_failed", err.Error())
		return
	}
	posterPath := filepath.Join(posterDir, src.ID+".jpg")
	tmp := posterPath + ".tmp"

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	resp, err := client.Get(req.URL)
	if err != nil {
		writeError(w, http.StatusBadGateway, "fetch_failed", err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "fetch_failed", fmt.Sprintf("HTTP %d", resp.StatusCode))
		return
	}

	// Read limited body and validate actual content type
	limited := io.LimitReader(resp.Body, 2<<20)
	data, err := io.ReadAll(limited)
	if err != nil {
		writeError(w, http.StatusBadGateway, "read_failed", err.Error())
		return
	}
	detected := http.DetectContentType(data)
	if !strings.HasPrefix(detected, "image/") {
		writeError(w, http.StatusBadRequest, "invalid", "content is not an image: "+detected)
		return
	}

	// Atomic write
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "write_failed", err.Error())
		return
	}
	if err := os.Rename(tmp, posterPath); err != nil {
		os.Remove(tmp)
		writeError(w, http.StatusInternalServerError, "write_failed", err.Error())
		return
	}
	if err := a.store.UpdateSourcePoster(id, posterPath, req.URL); err != nil {
		writeError(w, http.StatusInternalServerError, "store_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"posterPath": posterPath})
}

func (a *App) handleDeletePoster(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	src, ok := a.store.GetSource(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if src.PosterPath != "" {
		os.Remove(src.PosterPath)
	}
	a.store.UpdateSourcePoster(id, "", "")
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleFetchPoster(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	src, ok := a.store.GetSource(id)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "source not found")
		return
	}
	var posterURL string
	if src.Kind == models.SourceKindVideo && src.Locator != "" {
		// For videos, use YouTube thumbnail directly (no yt-dlp needed)
		posterURL = "https://img.youtube.com/vi/" + src.Locator + "/maxresdefault.jpg"
	} else {
		// For channels/playlists, fetch metadata only (no video listing)
		var err error
		posterURL, err = ytdlp.FetchPosterURL(src.URL)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "fetch_failed", err.Error())
			return
		}
	}
	if posterURL == "" {
		writeError(w, http.StatusNotFound, "no_poster", "no poster image found for this source")
		return
	}
	// Download and save
	posterDir := filepath.Join(a.downloadDir, ".posters")
	if err := os.MkdirAll(posterDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "write_failed", err.Error())
		return
	}
	posterPath := filepath.Join(posterDir, src.ID+".jpg")
	tmp := posterPath + ".tmp"

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(posterURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, "fetch_failed", err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "fetch_failed", fmt.Sprintf("HTTP %d", resp.StatusCode))
		return
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		writeError(w, http.StatusBadGateway, "read_failed", err.Error())
		return
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "write_failed", err.Error())
		return
	}
	if err := os.Rename(tmp, posterPath); err != nil {
		os.Remove(tmp)
		writeError(w, http.StatusInternalServerError, "write_failed", err.Error())
		return
	}
	if err := a.store.UpdateSourcePoster(id, posterPath, posterURL); err != nil {
		writeError(w, http.StatusInternalServerError, "store_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"posterPath": posterPath, "posterURL": posterURL})
}

func (a *App) handleImageSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing_query", "q parameter required")
		return
	}
	// DuckDuckGo image search (best-effort, no API key)
	client := &http.Client{Timeout: 10 * time.Second}
	// Step 1: get vqd token
	searchURL := "https://duckduckgo.com/?q=" + strings.ReplaceAll(q, " ", "+") + "&iax=images&ia=images"
	resp1, err := client.Get(searchURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, "search_failed", "DuckDuckGo unreachable")
		return
	}
	body1, _ := io.ReadAll(io.LimitReader(resp1.Body, 100<<10))
	resp1.Body.Close()
	// Extract vqd token
	vqd := ""
	if idx := strings.Index(string(body1), "vqd=\""); idx >= 0 {
		rest := string(body1)[idx+5:]
		if end := strings.Index(rest, "\""); end >= 0 {
			vqd = rest[:end]
		}
	}
	if vqd == "" {
		if idx := strings.Index(string(body1), "vqd='"); idx >= 0 {
			rest := string(body1)[idx+5:]
			if end := strings.Index(rest, "'"); end >= 0 {
				vqd = rest[:end]
			}
		}
	}
	if vqd == "" {
		writeError(w, http.StatusBadGateway, "search_failed", "could not extract search token")
		return
	}
	// Step 2: fetch image results
	imgURL := fmt.Sprintf("https://duckduckgo.com/i.js?q=%s&vqd=%s&o=json&p=1&f=,,,,,", strings.ReplaceAll(q, " ", "+"), vqd)
	req2, _ := http.NewRequest("GET", imgURL, nil)
	req2.Header.Set("User-Agent", "Mozilla/5.0")
	resp2, err := client.Do(req2)
	if err != nil {
		writeError(w, http.StatusBadGateway, "search_failed", err.Error())
		return
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(io.LimitReader(resp2.Body, 500<<10))
	var ddgResp struct {
		Results []struct {
			Image     string `json:"image"`
			Thumbnail string `json:"thumbnail"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Title     string `json:"title"`
		} `json:"results"`
	}
	json.Unmarshal(body2, &ddgResp)
	type imgResult struct {
		URL       string `json:"url"`
		Thumbnail string `json:"thumbnail"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		Title     string `json:"title"`
	}
	results := make([]imgResult, 0, 20)
	for _, r := range ddgResp.Results {
		if r.Image == "" || len(results) >= 20 {
			break
		}
		results = append(results, imgResult{URL: r.Image, Thumbnail: r.Thumbnail, Width: r.Width, Height: r.Height, Title: r.Title})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (a *App) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Clean up poster before deleting source
	if src, ok := a.store.GetSource(id); ok && src.PosterPath != "" {
		os.Remove(src.PosterPath)
	}
	if err := a.store.DeleteSource(id); err != nil {
		writeError(w, http.StatusNotFound, "delete_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.store.DeleteTask(id); err != nil {
		writeError(w, http.StatusBadRequest, "delete_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleStopAllRunning(w http.ResponseWriter, _ *http.Request) {
	tasks := a.store.ListTasks()
	stopped := 0
	for _, t := range tasks {
		if t.Status == models.TaskStatusRunning {
			if err := a.worker.CancelTask(t.ID); err == nil {
				stopped++
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"stopped": stopped})
}

func (a *App) handleRetryAllFailed(w http.ResponseWriter, _ *http.Request) {
	tasks := a.store.ListTasks()
	retried := 0
	for _, t := range tasks {
		if t.Status != models.TaskStatusFailed {
			continue
		}
		// Reset media item to pending if it's a download task
		if t.Type == "download" && t.EntityType == "media" {
			_ = a.store.RetryMediaItem(t.EntityID)
		}
		// Requeue the existing failed task and start it
		if _, err := a.store.RequeueTask(t.ID); err != nil {
			continue
		}
		if _, err := a.worker.StartTask(t.ID); err == nil {
			retried++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"retried": retried})
}

func (a *App) handleRetryTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, ok := a.store.GetTask(id)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "task not found")
		return
	}
	// Reset media item if it's a download task
	if task.Type == "download" && task.EntityType == "media" {
		_ = a.store.RetryMediaItem(task.EntityID)
	}
	requeued, err := a.store.RequeueTask(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "retry_failed", err.Error())
		return
	}
	started, err := a.worker.StartTask(requeued.ID)
	if err != nil {
		writeJSON(w, http.StatusAccepted, map[string]any{"task": requeued, "message": "requeued"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"task": started})
}

func (a *App) handleDeleteCompletedTasks(w http.ResponseWriter, _ *http.Request) {
	removed := a.store.DeleteCompletedTasks()
	writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
}

// ── Cookie handlers ──────────────────────────────────────────────────────────

const maxCookieSize = 1 << 20 // 1 MB

func (a *App) cookiesFileInfo() (exists bool, size int64, modTime time.Time) {
	fi, err := os.Stat(a.cookiesPath)
	if err != nil || fi.IsDir() {
		return false, 0, time.Time{}
	}
	return true, fi.Size(), fi.ModTime()
}

func (a *App) handleCookiesStatus(w http.ResponseWriter, _ *http.Request) {
	exists, size, modTime := a.cookiesFileInfo()

	resp := map[string]any{
		"path":   a.cookiesPath,
		"exists": exists,
	}
	if exists {
		resp["size"] = size
		resp["lastModified"] = modTime.UTC()
		resp["hasLoginCookies"] = ytdlp.HasLoginCookies(a.cookiesPath)
	}

	// Return cached validation if fresh (5 min and same mtime)
	a.cookiesMu.Lock()
	if a.cookiesValid != nil && time.Since(a.cookiesValidAt) < 5*time.Minute && a.cookiesValidMtime == modTime {
		resp["valid"] = *a.cookiesValid
		resp["validationMessage"] = a.cookiesValidMsg
		resp["validatedAt"] = a.cookiesValidAt.UTC()
	}
	a.cookiesMu.Unlock()

	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleCookiesUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCookieSize)

	var content []byte
	var err error

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		file, _, fErr := r.FormFile("file")
		if fErr != nil {
			writeError(w, http.StatusBadRequest, "upload_failed", "no file in upload: "+fErr.Error())
			return
		}
		defer file.Close()
		content, err = io.ReadAll(file)
	} else {
		content, err = io.ReadAll(r.Body)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_failed", "failed to read body: "+err.Error())
		return
	}

	// Reject empty
	if len(strings.TrimSpace(string(content))) == 0 {
		writeError(w, http.StatusBadRequest, "empty_cookies", "cookies file is empty")
		return
	}

	// Basic Netscape format validation
	text := string(content)
	if !strings.Contains(text, ".youtube.com") {
		writeError(w, http.StatusBadRequest, "invalid_format", "cookies file does not contain YouTube cookies — expected Netscape cookie format with .youtube.com entries")
		return
	}

	// Atomic write: temp file then rename
	dir := filepath.Dir(a.cookiesPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "write_failed", "cannot create directory: "+err.Error())
		return
	}
	tmp, err := os.CreateTemp(dir, "cookies-*.tmp")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "write_failed", err.Error())
		return
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		writeError(w, http.StatusInternalServerError, "write_failed", err.Error())
		return
	}
	tmp.Close()
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		os.Remove(tmpPath)
		writeError(w, http.StatusInternalServerError, "write_failed", err.Error())
		return
	}
	// Remove existing path if it's a stale directory (Docker bind-mount artifact)
	if fi, err := os.Stat(a.cookiesPath); err == nil && fi.IsDir() {
		os.RemoveAll(a.cookiesPath)
	}
	if err := os.Rename(tmpPath, a.cookiesPath); err != nil {
		os.Remove(tmpPath)
		writeError(w, http.StatusInternalServerError, "write_failed", err.Error())
		return
	}

	// Invalidate cached validation
	a.cookiesMu.Lock()
	a.cookiesValid = nil
	a.cookiesMu.Unlock()

	resp := map[string]any{
		"message": "Cookies saved",
		"size":    len(content),
	}

	// Check whether the cookies contain actual login session tokens
	if !ytdlp.HasLoginCookies(a.cookiesPath) {
		resp["warning"] = "These cookies do not contain YouTube login session tokens (SID, __Secure-1PSID, etc.). " +
			"They appear to be visitor-only cookies. Make sure you are logged in to YouTube in your browser before exporting cookies."
		resp["hasLoginCookies"] = false
	} else {
		resp["hasLoginCookies"] = true
	}

	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleCookiesDelete(w http.ResponseWriter, _ *http.Request) {
	os.Remove(a.cookiesPath) // Idempotent — ignore error if file missing
	a.cookiesMu.Lock()
	a.cookiesValid = nil
	a.cookiesMu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleCookiesValidate(w http.ResponseWriter, _ *http.Request) {
	exists, _, modTime := a.cookiesFileInfo()
	if !exists {
		writeJSON(w, http.StatusOK, map[string]any{
			"valid":   false,
			"message": "No cookies file found",
		})
		return
	}

	err := ytdlp.ValidateCookies(a.cookiesPath)
	valid := err == nil
	msg := "Cookies are valid — YouTube authentication successful"
	if !valid {
		msg = err.Error()
	}

	// Cache the result
	a.cookiesMu.Lock()
	a.cookiesValid = &valid
	a.cookiesValidMsg = msg
	a.cookiesValidAt = time.Now()
	a.cookiesValidMtime = modTime
	a.cookiesMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"valid":   valid,
		"message": msg,
	})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}
