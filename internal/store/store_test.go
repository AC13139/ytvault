package store

import (
	"path/filepath"
	"testing"
	"time"

	"ytvault/internal/models"
	"ytvault/internal/yt"
)

// helpers

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func createTestSource(t *testing.T, s *Store, rawURL string) (models.Source, models.Task) {
	t.Helper()
	profile, err := yt.NormalizeSource(rawURL, models.SourceKindVideo)
	if err != nil {
		t.Fatal(err)
	}
	src, task, err := s.CreateSource(models.SourceKindVideo, profile)
	if err != nil {
		t.Fatal(err)
	}
	return src, task
}

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

func TestRunTaskRejectsNonQueuedTerminalTasks(t *testing.T) {
	tmp := t.TempDir()
	s, err := New(filepath.Join(tmp, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	profile, err := yt.NormalizeSource("https://www.youtube.com/watch?v=abc123", models.SourceKindVideo)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("succeeded task cannot run again", func(t *testing.T) {
		_, task, err := s.CreateSource(models.SourceKindVideo, profile)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.RunTask(task.ID); err != nil {
			t.Fatal(err)
		}
		_, err = s.RunTask(task.ID)
		if err == nil {
			t.Fatal("expected rerun to fail")
		}
		notRunnable, ok := err.(*TaskNotRunnableError)
		if !ok {
			t.Fatalf("expected TaskNotRunnableError, got %T", err)
		}
		if notRunnable.Status != models.TaskStatusSucceeded {
			t.Fatalf("status = %q", notRunnable.Status)
		}
	})

	t.Run("failed task cannot run again", func(t *testing.T) {
		_, task, err := s.CreateSource(models.SourceKindVideo, profile)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.FailTaskStub(task.ID, "FAIL_STUB", "boom"); err != nil {
			t.Fatal(err)
		}
		_, err = s.RunTask(task.ID)
		if err == nil {
			t.Fatal("expected rerun to fail")
		}
		notRunnable, ok := err.(*TaskNotRunnableError)
		if !ok {
			t.Fatalf("expected TaskNotRunnableError, got %T", err)
		}
		if notRunnable.Status != models.TaskStatusFailed {
			t.Fatalf("status = %q", notRunnable.Status)
		}
	})
}

func TestCreateMediaItems_Dedup(t *testing.T) {
	s := newTestStore(t)
	src, _ := createTestSource(t, s, "https://www.youtube.com/watch?v=abc123")

	items := []models.MediaItem{
		{SourceID: src.ID, VideoID: "vid1", Title: "Video 1", URL: "https://www.youtube.com/watch?v=vid1", Status: models.MediaItemStatusPending},
		{SourceID: src.ID, VideoID: "vid2", Title: "Video 2", URL: "https://www.youtube.com/watch?v=vid2", Status: models.MediaItemStatusPending},
	}
	created, err := s.CreateMediaItems(items)
	if err != nil {
		t.Fatal(err)
	}
	if created != 2 {
		t.Fatalf("expected 2 created, got %d", created)
	}

	// Re-inserting the same items should skip all.
	created, err = s.CreateMediaItems(items)
	if err != nil {
		t.Fatal(err)
	}
	if created != 0 {
		t.Fatalf("expected 0 created on dedup, got %d", created)
	}
	if got := len(s.ListMediaItemsBySource(src.ID)); got != 2 {
		t.Fatalf("expected 2 media items, got %d", got)
	}

	// A new video in the same batch should be inserted; duplicates skipped.
	created, err = s.CreateMediaItems(append(items, models.MediaItem{
		SourceID: src.ID, VideoID: "vid3", Title: "Video 3",
		URL: "https://www.youtube.com/watch?v=vid3", Status: models.MediaItemStatusPending,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if created != 1 {
		t.Fatalf("expected 1 created, got %d", created)
	}
}

func TestCreateDownloadTask_Validation(t *testing.T) {
	s := newTestStore(t)
	src, _ := createTestSource(t, s, "https://www.youtube.com/watch?v=abc123")

	_, err := s.CreateDownloadTask("nonexistent_media")
	if err == nil {
		t.Fatal("expected error for nonexistent media item")
	}

	// Create a media item.
	created, err := s.CreateMediaItems([]models.MediaItem{
		{SourceID: src.ID, VideoID: "vid1", Status: models.MediaItemStatusPending},
	})
	if err != nil || created != 1 {
		t.Fatalf("create media items: err=%v created=%d", err, created)
	}
	item := s.ListMediaItemsBySource(src.ID)[0]

	task, err := s.CreateDownloadTask(item.ID)
	if err != nil {
		t.Fatalf("first CreateDownloadTask: %v", err)
	}
	if task.EntityID != item.ID {
		t.Fatalf("task.EntityID = %q", task.EntityID)
	}

	// Duplicate call must be rejected while first task is queued.
	_, err = s.CreateDownloadTask(item.ID)
	if err == nil {
		t.Fatal("expected error creating duplicate active download task")
	}
}

func TestCreateDownloadTasksForSource_Validation(t *testing.T) {
	s := newTestStore(t)

	// Unknown source returns error.
	_, err := s.CreateDownloadTasksForSource("src_999")
	if err == nil {
		t.Fatal("expected error for unknown source")
	}

	src, _ := createTestSource(t, s, "https://www.youtube.com/watch?v=abc123")
	_, _ = s.CreateMediaItems([]models.MediaItem{
		{SourceID: src.ID, VideoID: "vid1", Status: models.MediaItemStatusPending},
		{SourceID: src.ID, VideoID: "vid2", Status: models.MediaItemStatusPending},
	})

	tasks, err := s.CreateDownloadTasksForSource(src.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	// Second call must not produce duplicates (tasks are still queued).
	tasks2, err := s.CreateDownloadTasksForSource(src.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks2) != 0 {
		t.Fatalf("expected 0 duplicate tasks, got %d", len(tasks2))
	}
}

func TestCreateIndexTask(t *testing.T) {
	s := newTestStore(t)
	src, initial := createTestSource(t, s, "https://www.youtube.com/watch?v=abc123")

	// Unknown source returns error.
	if _, err := s.CreateIndexTask("src_999", false); err == nil {
		t.Fatal("expected error for unknown source")
	}

	// Blocked while the initial queued index task is still active.
	if _, err := s.CreateIndexTask(src.ID, true); err == nil {
		t.Fatal("expected error while initial task is queued")
	}

	// Complete the initial task so it's no longer active.
	if _, err := s.RunTask(initial.ID); err != nil {
		t.Fatal(err)
	}

	// Creates an index task.
	task, err := s.CreateIndexTask(src.ID, true)
	if err != nil {
		t.Fatalf("CreateIndexTask: %v", err)
	}
	if task.Type != "index" || task.EntityID != src.ID || !task.AutoDownload {
		t.Fatalf("unexpected task: %+v", task)
	}

	// Duplicate is rejected while first task is active.
	if _, err := s.CreateIndexTask(src.ID, false); err == nil {
		t.Fatal("expected error for duplicate active index task")
	}
}

func TestUpsertAndDeleteSchedule(t *testing.T) {
	s := newTestStore(t)
	src, _ := createTestSource(t, s, "https://www.youtube.com/watch?v=abc123")

	// Unknown source returns error.
	if _, err := s.UpsertSchedule("src_999", 24, true, false); err == nil {
		t.Fatal("expected error for unknown source")
	}
	// Invalid interval.
	if _, err := s.UpsertSchedule(src.ID, 0, true, false); err == nil {
		t.Fatal("expected error for intervalHours=0")
	}

	sched, err := s.UpsertSchedule(src.ID, 24, true, true)
	if err != nil {
		t.Fatalf("UpsertSchedule: %v", err)
	}
	if sched.SourceID != src.ID || sched.IntervalHours != 24 || !sched.Enabled || !sched.AutoDownload {
		t.Fatalf("unexpected schedule: %+v", sched)
	}

	// Update: change interval and disable.
	sched2, err := s.UpsertSchedule(src.ID, 6, false, false)
	if err != nil {
		t.Fatalf("UpsertSchedule update: %v", err)
	}
	if sched2.IntervalHours != 6 || sched2.Enabled || sched2.AutoDownload {
		t.Fatalf("unexpected updated schedule: %+v", sched2)
	}
	if got := len(s.ListSchedules()); got != 1 {
		t.Fatalf("expected 1 schedule, got %d", got)
	}

	// Delete.
	if err := s.DeleteSchedule(src.ID); err != nil {
		t.Fatalf("DeleteSchedule: %v", err)
	}
	if got := len(s.ListSchedules()); got != 0 {
		t.Fatalf("expected 0 schedules, got %d", got)
	}

	// Delete nonexistent returns error.
	if err := s.DeleteSchedule(src.ID); err == nil {
		t.Fatal("expected error deleting nonexistent schedule")
	}
}

func TestListDueSchedules(t *testing.T) {
	s := newTestStore(t)
	src1, _ := createTestSource(t, s, "https://www.youtube.com/watch?v=abc123")
	src2, _ := createTestSource(t, s, "https://www.youtube.com/watch?v=def456")

	// src1: enabled, due now (very short interval so NextRunAt is already past)
	if _, err := s.UpsertSchedule(src1.ID, 24, true, false); err != nil {
		t.Fatal(err)
	}
	// src2: disabled — should not be due
	if _, err := s.UpsertSchedule(src2.ID, 1, false, false); err != nil {
		t.Fatal(err)
	}

	far := s.ListSchedules()[0].NextRunAt.Add(48 * 3600e9) // 48h in the future from NextRunAt
	due := s.ListDueSchedules(far)
	if len(due) != 1 || due[0].SourceID != src1.ID {
		t.Fatalf("expected 1 due schedule for src1, got %+v", due)
	}

	// Neither is due yet if we check right now (NextRunAt is in the future).
	pastDue := s.ListDueSchedules(s.ListSchedules()[0].NextRunAt.Add(-time.Hour))
	if len(pastDue) != 0 {
		t.Fatalf("expected 0 due schedules, got %d", len(pastDue))
	}
}

func TestTouchSchedule(t *testing.T) {
	s := newTestStore(t)
	src, _ := createTestSource(t, s, "https://www.youtube.com/watch?v=abc123")
	if _, err := s.UpsertSchedule(src.ID, 6, true, false); err != nil {
		t.Fatal(err)
	}

	now := s.ListSchedules()[0].NextRunAt
	if err := s.TouchSchedule(src.ID, now); err != nil {
		t.Fatalf("TouchSchedule: %v", err)
	}

	sched, _ := s.GetSchedule(src.ID)
	if sched.LastRunAt == nil || !sched.LastRunAt.Equal(now) {
		t.Fatalf("LastRunAt not set correctly: %v", sched.LastRunAt)
	}
	expectedNext := now.Add(6 * time.Hour)
	if !sched.NextRunAt.Equal(expectedNext) {
		t.Fatalf("NextRunAt = %v, want %v", sched.NextRunAt, expectedNext)
	}
}
