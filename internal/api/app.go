package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"ytvault/internal/models"
	"ytvault/internal/scheduler"
	"ytvault/internal/store"
	"ytvault/internal/worker"
	"ytvault/internal/yt"
)

type App struct {
	store     *store.Store
	worker    *worker.Worker
	scheduler *scheduler.Scheduler
}

func NewApp(dataPath, downloadDir string) (*App, error) {
	s, err := store.New(dataPath)
	if err != nil {
		return nil, err
	}
	w := worker.New(s, downloadDir)
	sc := scheduler.New(s, w)
	sc.Start()
	return &App{store: s, worker: w, scheduler: sc}, nil
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
	mux.HandleFunc("POST /api/v1/tasks/{id}/execute", a.handleExecuteTask)
	mux.HandleFunc("GET /", a.handleIndex)
	return withJSONLogging(mux)
}

// ── Source handlers ───────────────────────────────────────────────────────────

type createSourceRequest struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
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
	src, task, err := a.store.CreateSource(kind, profile)
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
	task, err := a.store.CreateIndexTask(id, false)
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
	IntervalHours int  `json:"intervalHours"`
	Enabled       bool `json:"enabled"`
	AutoDownload  bool `json:"autoDownload"`
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
	sched, err := a.store.UpsertSchedule(id, req.IntervalHours, req.Enabled, req.AutoDownload)
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
