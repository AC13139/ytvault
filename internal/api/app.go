package api

import (
	"encoding/json"
	"net/http"
	"time"

	"ytvault/internal/models"
	"ytvault/internal/store"
	"ytvault/internal/yt"
)

type App struct {
	store *store.Store
}

func NewApp(dataPath string) (*App, error) {
	s, err := store.New(dataPath)
	if err != nil {
		return nil, err
	}
	return &App{store: s}, nil
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.handleHealth)
	mux.HandleFunc("GET /api/v1/sources", a.handleListSources)
	mux.HandleFunc("POST /api/v1/sources", a.handleCreateSource)
	mux.HandleFunc("GET /api/v1/tasks", a.handleListTasks)
	mux.HandleFunc("POST /api/v1/tasks/{id}/run", a.handleRunTask)
	mux.HandleFunc("POST /api/v1/tasks/{id}/execute", a.handleExecuteTask)
	mux.HandleFunc("GET /", a.handleIndex)
	return withJSONLogging(mux)
}

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

func (a *App) handleListTasks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": a.store.ListTasks()})
}

func (a *App) handleRunTask(w http.ResponseWriter, r *http.Request) {
	task, err := a.store.RunTask(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "run_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": task})
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
