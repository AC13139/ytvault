package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	apiapp "ytvault/internal/api"
)

func TestCLIChannelIntakeAndList(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	addOut := runCLI(t, server.URL, "source", "add", "channel", "http://www.youtube.com/@SpeechLab-m7o")
	for _, want := range []string{
		"Source src_1",
		"Title: Channel (pending index)",
		"Path: @handle",
		"Normalized: https://www.youtube.com/@SpeechLab-m7o",
		"Original: http://www.youtube.com/@SpeechLab-m7o",
		"Intake: validated",
		"Task task_1",
	} {
		if !strings.Contains(addOut, want) {
			t.Fatalf("add output missing %q\n%s", want, addOut)
		}
	}

	listOut := runCLI(t, server.URL, "source", "list")
	for _, want := range []string{
		"Source src_1",
		"Title: Channel (pending index)",
		"Path: @handle",
		"Prototype: validated-only: source accepted and queued, live indexing/download not wired yet",
	} {
		if !strings.Contains(listOut, want) {
			t.Fatalf("list output missing %q\n%s", want, listOut)
		}
	}
}

func TestCLITaskListShowsFailureDetails(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	_ = runCLI(t, server.URL, "source", "add", "channel", "http://www.youtube.com/@SpeechLab-m7o")

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/tasks/task_1/execute", bytes.NewReader([]byte(`{"mode":"fail_stub"}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("execute status = %d", resp.StatusCode)
	}

	listOut := runCLI(t, server.URL, "task", "list")
	for _, want := range []string{
		"Task task_1",
		"Status: failed",
		"ErrorCode: FAIL_STUB",
		"ErrorMessage: stubbed execution failure triggered by request mode=fail_stub",
	} {
		if !strings.Contains(listOut, want) {
			t.Fatalf("task list output missing %q\n%s", want, listOut)
		}
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	app, err := apiapp.NewApp(filepath.Join(t.TempDir(), "state.json"), t.TempDir(), filepath.Join(t.TempDir(), "cookies.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(app.Routes())
}

func runCLI(t *testing.T, base string, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run", "./apps/cli"}, args...)...)
	cmd.Dir = filepath.Join("..", "..")
	cmd.Env = append(cmd.Environ(), "YTVAULT_API="+base)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cli %v failed: %v\n%s", args, err, string(out))
	}
	return string(out)
}
