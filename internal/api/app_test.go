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
	app, err := NewApp(filepath.Join(t.TempDir(), "state.json"))
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
