package store

import (
	"path/filepath"
	"testing"

	"ytvault/internal/models"
	"ytvault/internal/yt"
)

func TestCreateSource(t *testing.T) {
	tmp := t.TempDir()
	s, err := New(filepath.Join(tmp, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	profile, err := yt.NormalizeSource("https://www.youtube.com/watch?v=abc123", models.SourceKindVideo)
	if err != nil {
		t.Fatal(err)
	}
	src, task, err := s.CreateSource(models.SourceKindVideo, profile)
	if err != nil {
		t.Fatal(err)
	}
	if src.ID == "" || task.ID == "" {
		t.Fatal("expected source and task IDs")
	}
	if src.URL != "https://www.youtube.com/watch?v=abc123" {
		t.Fatalf("normalized url = %q", src.URL)
	}
	if src.Title != "Video abc123" {
		t.Fatalf("title = %q", src.Title)
	}
	if src.Locator != "abc123" {
		t.Fatalf("locator = %q", src.Locator)
	}
	if src.PathKind != "watch" || src.PathLabel != "watch page" {
		t.Fatalf("path = %q / %q", src.PathKind, src.PathLabel)
	}
	if src.IntakeStage != "validated" {
		t.Fatalf("intake stage = %q", src.IntakeStage)
	}
	if got := len(s.ListSources()); got != 1 {
		t.Fatalf("expected 1 source, got %d", got)
	}
	if got := len(s.ListTasks()); got != 1 {
		t.Fatalf("expected 1 task, got %d", got)
	}
}

func TestRunTaskTransitionsQueuedToSucceeded(t *testing.T) {
	tmp := t.TempDir()
	s, err := New(filepath.Join(tmp, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	profile, err := yt.NormalizeSource("https://www.youtube.com/watch?v=abc123", models.SourceKindVideo)
	if err != nil {
		t.Fatal(err)
	}
	_, task, err := s.CreateSource(models.SourceKindVideo, profile)
	if err != nil {
		t.Fatal(err)
	}

	runTask, err := s.RunTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if runTask.Status != models.TaskStatusSucceeded {
		t.Fatalf("status = %q", runTask.Status)
	}
	if runTask.CompletedAt == nil {
		t.Fatal("expected completedAt to be set")
	}

	tasks := s.ListTasks()
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Status != models.TaskStatusSucceeded {
		t.Fatalf("persisted status = %q", tasks[0].Status)
	}
}
