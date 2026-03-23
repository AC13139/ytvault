package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	app, err := NewApp(filepath.Join(t.TempDir(), "state.json"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func TestCreateSourceReturnsNormalizedChannelMetadata(t *testing.T) {
	app := newTestApp(t)
	body := []byte(`{"kind":"channel","url":"http://www.youtube.com/@SpeechLab-m7o"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", bytes.NewReader(body))
	w := httptest.NewRecorder()

	app.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Source struct {
			Kind              string `json:"kind"`
			URL               string `json:"url"`
			InputURL          string `json:"inputUrl"`
			Title             string `json:"title"`
			Locator           string `json:"locator"`
			PathKind          string `json:"pathKind"`
			PathLabel         string `json:"pathLabel"`
			IntakeStage       string `json:"intakeStage"`
			NextAction        string `json:"nextAction"`
			ValidationSummary string `json:"validationSummary"`
		} `json:"source"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Source.Kind != "channel" {
		t.Fatalf("kind = %q", resp.Source.Kind)
	}
	if resp.Source.URL != "https://www.youtube.com/@SpeechLab-m7o" {
		t.Fatalf("url = %q", resp.Source.URL)
	}
	if resp.Source.InputURL != "http://www.youtube.com/@SpeechLab-m7o" {
		t.Fatalf("inputUrl = %q", resp.Source.InputURL)
	}
	if resp.Source.Title != "Channel @SpeechLab-m7o" {
		t.Fatalf("title = %q", resp.Source.Title)
	}
	if resp.Source.Locator != "@SpeechLab-m7o" {
		t.Fatalf("locator = %q", resp.Source.Locator)
	}
	if resp.Source.PathKind != "handle" || resp.Source.PathLabel != "@handle" {
		t.Fatalf("path = %q / %q", resp.Source.PathKind, resp.Source.PathLabel)
	}
	if resp.Source.IntakeStage != "validated" {
		t.Fatalf("intakeStage = %q", resp.Source.IntakeStage)
	}
	if !strings.Contains(resp.Source.NextAction, "Live indexing/download") {
		t.Fatalf("nextAction = %q", resp.Source.NextAction)
	}
	if !strings.Contains(resp.Source.ValidationSummary, "channel source") {
		t.Fatalf("validationSummary = %q", resp.Source.ValidationSummary)
	}
}

func TestIndexShowsSourceProfileDetails(t *testing.T) {
	app := newTestApp(t)
	body := []byte(`{"kind":"channel","url":"http://www.youtube.com/@SpeechLab-m7o"}`)
	postReq := httptest.NewRequest(http.MethodPost, "/api/v1/sources", bytes.NewReader(body))
	postW := httptest.NewRecorder()
	app.Routes().ServeHTTP(postW, postReq)
	if postW.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", postW.Code, postW.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/", nil)
	getW := httptest.NewRecorder()
	app.Routes().ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("index status = %d", getW.Code)
	}
	page := getW.Body.String()
	for _, want := range []string{
		"Channel @SpeechLab-m7o",
		"https://www.youtube.com/@SpeechLab-m7o",
		"http://www.youtube.com/@SpeechLab-m7o",
		"@handle",
		"validated",
		"Live indexing/download",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("index missing %q", want)
		}
	}
}

func TestExecuteTaskEndpointCanFailStub(t *testing.T) {
	app := newTestApp(t)
	createBody := []byte(`{"kind":"video","url":"https://www.youtube.com/watch?v=abc123"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sources", bytes.NewReader(createBody))
	createW := httptest.NewRecorder()
	app.Routes().ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createW.Code, createW.Body.String())
	}

	var createResp struct {
		Task struct {
			ID string `json:"id"`
		} `json:"task"`
	}
	if err := json.Unmarshal(createW.Body.Bytes(), &createResp); err != nil {
		t.Fatal(err)
	}

	executeReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+createResp.Task.ID+"/execute", bytes.NewReader([]byte(`{"mode":"fail_stub"}`)))
	executeW := httptest.NewRecorder()
	app.Routes().ServeHTTP(executeW, executeReq)
	if executeW.Code != http.StatusOK {
		t.Fatalf("execute status = %d body=%s", executeW.Code, executeW.Body.String())
	}

	var executeResp struct {
		Task struct {
			ID           string `json:"id"`
			Status       string `json:"status"`
			ErrorCode    string `json:"errorCode"`
			ErrorMessage string `json:"errorMessage"`
			CompletedAt  string `json:"completedAt"`
		} `json:"task"`
		Execution struct {
			Mode string `json:"mode"`
		} `json:"execution"`
	}
	if err := json.Unmarshal(executeW.Body.Bytes(), &executeResp); err != nil {
		t.Fatal(err)
	}
	if executeResp.Task.ID != createResp.Task.ID {
		t.Fatalf("task id = %q want %q", executeResp.Task.ID, createResp.Task.ID)
	}
	if executeResp.Task.Status != "failed" {
		t.Fatalf("status = %q", executeResp.Task.Status)
	}
	if executeResp.Task.ErrorCode != "FAIL_STUB" {
		t.Fatalf("errorCode = %q", executeResp.Task.ErrorCode)
	}
	if !strings.Contains(executeResp.Task.ErrorMessage, "fail_stub") {
		t.Fatalf("errorMessage = %q", executeResp.Task.ErrorMessage)
	}
	if executeResp.Task.CompletedAt == "" {
		t.Fatal("expected completedAt to be set")
	}
	if executeResp.Execution.Mode != "fail_stub" {
		t.Fatalf("execution mode = %q", executeResp.Execution.Mode)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	listW := httptest.NewRecorder()
	app.Routes().ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listW.Code, listW.Body.String())
	}

	var listResp struct {
		Items []struct {
			ID           string `json:"id"`
			Status       string `json:"status"`
			ErrorCode    string `json:"errorCode"`
			ErrorMessage string `json:"errorMessage"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Items) != 1 {
		t.Fatalf("task count = %d", len(listResp.Items))
	}
	if listResp.Items[0].ID != createResp.Task.ID || listResp.Items[0].Status != "failed" {
		t.Fatalf("persisted task = %+v", listResp.Items[0])
	}
	if listResp.Items[0].ErrorCode != "FAIL_STUB" {
		t.Fatalf("persisted errorCode = %q", listResp.Items[0].ErrorCode)
	}
}

func TestRunTaskEndpointTransitionsQueuedToSucceeded(t *testing.T) {
	app := newTestApp(t)
	createBody := []byte(`{"kind":"video","url":"https://www.youtube.com/watch?v=abc123"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sources", bytes.NewReader(createBody))
	createW := httptest.NewRecorder()
	app.Routes().ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createW.Code, createW.Body.String())
	}

	var createResp struct {
		Task struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"task"`
	}
	if err := json.Unmarshal(createW.Body.Bytes(), &createResp); err != nil {
		t.Fatal(err)
	}
	if createResp.Task.Status != "queued" {
		t.Fatalf("queued status = %q", createResp.Task.Status)
	}

	runReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+createResp.Task.ID+"/run", bytes.NewReader([]byte(`{}`)))
	runW := httptest.NewRecorder()
	app.Routes().ServeHTTP(runW, runReq)
	if runW.Code != http.StatusOK {
		t.Fatalf("run status = %d body=%s", runW.Code, runW.Body.String())
	}

	var runResp struct {
		Task struct {
			ID          string `json:"id"`
			Status      string `json:"status"`
			CompletedAt string `json:"completedAt"`
		} `json:"task"`
	}
	if err := json.Unmarshal(runW.Body.Bytes(), &runResp); err != nil {
		t.Fatal(err)
	}
	if runResp.Task.ID != createResp.Task.ID {
		t.Fatalf("task id = %q want %q", runResp.Task.ID, createResp.Task.ID)
	}
	if runResp.Task.Status != "succeeded" {
		t.Fatalf("run status = %q", runResp.Task.Status)
	}
	if runResp.Task.CompletedAt == "" {
		t.Fatal("expected completedAt to be set")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	listW := httptest.NewRecorder()
	app.Routes().ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listW.Code, listW.Body.String())
	}

	var listResp struct {
		Items []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Items) != 1 {
		t.Fatalf("task count = %d", len(listResp.Items))
	}
	if listResp.Items[0].ID != createResp.Task.ID || listResp.Items[0].Status != "succeeded" {
		t.Fatalf("persisted task = %+v", listResp.Items[0])
	}
}

func TestExecuteTaskEndpointRejectsTerminalTaskRerun(t *testing.T) {
	app := newTestApp(t)
	createBody := []byte(`{"kind":"video","url":"https://www.youtube.com/watch?v=abc123"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sources", bytes.NewReader(createBody))
	createW := httptest.NewRecorder()
	app.Routes().ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createW.Code, createW.Body.String())
	}

	var createResp struct {
		Task struct {
			ID string `json:"id"`
		} `json:"task"`
	}
	if err := json.Unmarshal(createW.Body.Bytes(), &createResp); err != nil {
		t.Fatal(err)
	}

	t.Run("succeeded task returns 409", func(t *testing.T) {
		runReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+createResp.Task.ID+"/execute", bytes.NewReader([]byte(`{"mode":"stub"}`)))
		runW := httptest.NewRecorder()
		app.Routes().ServeHTTP(runW, runReq)
		if runW.Code != http.StatusOK {
			t.Fatalf("first execute status = %d body=%s", runW.Code, runW.Body.String())
		}

		rerunReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+createResp.Task.ID+"/execute", bytes.NewReader([]byte(`{"mode":"stub"}`)))
		rerunW := httptest.NewRecorder()
		app.Routes().ServeHTTP(rerunW, rerunReq)
		if rerunW.Code != http.StatusConflict {
			t.Fatalf("rerun status = %d body=%s", rerunW.Code, rerunW.Body.String())
		}
		if !strings.Contains(rerunW.Body.String(), `"code":"task_not_runnable"`) {
			t.Fatalf("rerun body=%s", rerunW.Body.String())
		}
		if !strings.Contains(rerunW.Body.String(), "status succeeded") {
			t.Fatalf("rerun body=%s", rerunW.Body.String())
		}
	})

	t.Run("failed task returns 409", func(t *testing.T) {
		app := newTestApp(t)
		createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sources", bytes.NewReader(createBody))
		createW := httptest.NewRecorder()
		app.Routes().ServeHTTP(createW, createReq)
		if createW.Code != http.StatusCreated {
			t.Fatalf("create status = %d body=%s", createW.Code, createW.Body.String())
		}
		var failedCreateResp struct {
			Task struct {
				ID string `json:"id"`
			} `json:"task"`
		}
		if err := json.Unmarshal(createW.Body.Bytes(), &failedCreateResp); err != nil {
			t.Fatal(err)
		}

		failReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+failedCreateResp.Task.ID+"/execute", bytes.NewReader([]byte(`{"mode":"fail_stub"}`)))
		failW := httptest.NewRecorder()
		app.Routes().ServeHTTP(failW, failReq)
		if failW.Code != http.StatusOK {
			t.Fatalf("first fail execute status = %d body=%s", failW.Code, failW.Body.String())
		}

		rerunReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+failedCreateResp.Task.ID+"/execute", bytes.NewReader([]byte(`{"mode":"stub"}`)))
		rerunW := httptest.NewRecorder()
		app.Routes().ServeHTTP(rerunW, rerunReq)
		if rerunW.Code != http.StatusConflict {
			t.Fatalf("rerun status = %d body=%s", rerunW.Code, rerunW.Body.String())
		}
		if !strings.Contains(rerunW.Body.String(), `"code":"task_not_runnable"`) {
			t.Fatalf("rerun body=%s", rerunW.Body.String())
		}
		if !strings.Contains(rerunW.Body.String(), "status failed") {
			t.Fatalf("rerun body=%s", rerunW.Body.String())
		}
	})
}

func TestListSourceMediaReturnsNotFoundForUnknownSource(t *testing.T) {
	app := newTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/src_999/media", nil)
	w := httptest.NewRecorder()
	app.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"code":"source_not_found"`) {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestListSourceMediaReturnsEmptyArrayForNewSource(t *testing.T) {
	app := newTestApp(t)
	createBody := []byte(`{"kind":"video","url":"https://www.youtube.com/watch?v=abc123"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sources", bytes.NewReader(createBody))
	createW := httptest.NewRecorder()
	app.Routes().ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d", createW.Code)
	}
	var createResp struct {
		Source struct{ ID string `json:"id"` } `json:"source"`
	}
	if err := json.Unmarshal(createW.Body.Bytes(), &createResp); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/"+createResp.Source.ID+"/media", nil)
	w := httptest.NewRecorder()
	app.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 0 {
		t.Fatalf("expected empty items, got %d", len(resp.Items))
	}
}

func TestDownloadAllReturnsErrorForUnknownSource(t *testing.T) {
	app := newTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/src_999/download-all", nil)
	w := httptest.NewRecorder()
	app.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"code":"create_tasks_failed"`) {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestDownloadAllReturnsZeroCountForSourceWithNoMedia(t *testing.T) {
	app := newTestApp(t)
	createBody := []byte(`{"kind":"video","url":"https://www.youtube.com/watch?v=abc123"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sources", bytes.NewReader(createBody))
	createW := httptest.NewRecorder()
	app.Routes().ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d", createW.Code)
	}
	var createResp struct {
		Source struct{ ID string `json:"id"` } `json:"source"`
	}
	if err := json.Unmarshal(createW.Body.Bytes(), &createResp); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/"+createResp.Source.ID+"/download-all", nil)
	w := httptest.NewRecorder()
	app.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 0 {
		t.Fatalf("expected count=0, got %d", resp.Count)
	}
}
