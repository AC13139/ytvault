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

	// Cached cookie validation result
	cookiesMu         sync.Mutex
	cookiesValid      *bool  // nil = not tested
	cookiesValidMsg   string
	cookiesValidAt    time.Time
	cookiesValidMtime time.Time
}

func NewApp(dataPath, downloadDir, cookiesPath string) (*App, error) {
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
	return &App{store: s, worker: w, scheduler: sc, cookiesPath: cookiesPath, downloadDir: downloadDir}, nil
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
	mux.HandleFunc("POST /api/v1/sources/{id}/reindex", a.handleReindex)
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
	mux.HandleFunc("POST /api/v1/media/{id}/retry", a.handleRetryMedia)
	mux.HandleFunc("POST /api/v1/media/download-all-pending", a.handleDownloadAllPending)
	mux.HandleFunc("POST /api/v1/tasks/{id}/execute", a.handleExecuteTask)
	mux.HandleFunc("POST /api/v1/tasks/{id}/cancel", a.handleCancelTask)
	mux.HandleFunc("DELETE /api/v1/tasks/{id}", a.handleDeleteTask)
	mux.HandleFunc("DELETE /api/v1/tasks", a.handleDeleteCompletedTasks)
	mux.HandleFunc("GET /api/v1/config", a.handleGetConfig)
	mux.HandleFunc("PUT /api/v1/config", a.handleUpdateConfig)
	mux.HandleFunc("GET /api/v1/youtube/search", a.handleYouTubeSearch)
	mux.HandleFunc("GET /api/v1/cookies/status", a.handleCookiesStatus)
	mux.HandleFunc("POST /api/v1/cookies", a.handleCookiesUpload)
	mux.HandleFunc("DELETE /api/v1/cookies", a.handleCookiesDelete)
	mux.HandleFunc("POST /api/v1/cookies/validate", a.handleCookiesValidate)
	mux.HandleFunc("GET /", a.handleIndex)
	return withJSONLogging(mux)
}

// ── Source handlers ───────────────────────────────────────────────────────────

type createSourceRequest struct {
	Kind         string `json:"kind"`
	URL          string `json:"url"`
	DateAfter    string `json:"dateAfter,omitempty"`
	AutoDownload bool   `json:"autoDownload,omitempty"`
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

func (a *App) handleListSources(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": a.store.ListSources()})
}

func (a *App) handleCreateSource(w http.ResponseWriter, r *http.Request) {
	var req createSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	kind := models.SourceKind(req.Kind)
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

func (a *App) handleDownloadMedia(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
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

func (a *App) handleDownloadAllPending(w http.ResponseWriter, _ *http.Request) {
	// Find all pending media items across all sources
	sources := a.store.ListSources()
	started := 0
	total := 0
	for _, src := range sources {
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
	if err := a.store.DeleteMediaItem(r.PathValue("id")); err != nil {
		writeError(w, http.StatusNotFound, "delete_failed", err.Error())
		return
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

func (a *App) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
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

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Cookies saved",
		"size":    len(content),
	})
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
