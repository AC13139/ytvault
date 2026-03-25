package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ytvault/internal/models"
	"ytvault/internal/yt"
)

type Store struct {
	mu        sync.RWMutex
	path      string
	state     models.State
	nextID    int
	nextTask  int
	nextMedia int
}

type TaskNotRunnableError struct {
	ID     string
	Status models.TaskStatus
}

func (e *TaskNotRunnableError) Error() string {
	return fmt.Sprintf("task %s is not runnable from status %s", e.ID, e.Status)
}

func IsTaskNotRunnable(err error) bool {
	var target *TaskNotRunnableError
	return errors.As(err, &target)
}

func New(path string) (*Store, error) {
	s := &Store{path: path, nextID: 1, nextTask: 1, nextMedia: 1}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.state = models.State{}
			return nil
		}
		return err
	}
	if err := json.Unmarshal(data, &s.state); err != nil {
		return err
	}
	for _, src := range s.state.Sources {
		var n int
		fmt.Sscanf(src.ID, "src_%d", &n)
		if n >= s.nextID {
			s.nextID = n + 1
		}
	}
	for _, task := range s.state.Tasks {
		var n int
		fmt.Sscanf(task.ID, "task_%d", &n)
		if n >= s.nextTask {
			s.nextTask = n + 1
		}
	}
	for _, item := range s.state.MediaItems {
		var n int
		fmt.Sscanf(item.ID, "media_%d", &n)
		if n >= s.nextMedia {
			s.nextMedia = n + 1
		}
	}
	return nil
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// ── Sources ──────────────────────────────────────────────────────────────────

func (s *Store) ListSources() []models.Source {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Source, len(s.state.Sources))
	copy(out, s.state.Sources)
	return out
}

func (s *Store) GetSource(id string) (models.Source, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, src := range s.state.Sources {
		if src.ID == id {
			return src, true
		}
	}
	return models.Source{}, false
}

func (s *Store) FindSourceByURL(normalizedURL string) (models.Source, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, src := range s.state.Sources {
		if src.URL == normalizedURL {
			return src, true
		}
	}
	return models.Source{}, false
}

func (s *Store) DeleteSource(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	for i, src := range s.state.Sources {
		if src.ID == id {
			s.state.Sources = append(s.state.Sources[:i], s.state.Sources[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("source %s not found", id)
	}

	// Cascade: remove related media items
	var media []models.MediaItem
	for _, m := range s.state.MediaItems {
		if m.SourceID != id {
			media = append(media, m)
		}
	}
	s.state.MediaItems = media

	// Cascade: remove related tasks (only completed ones; keep running/queued)
	var tasks []models.Task
	for _, t := range s.state.Tasks {
		if t.EntityID == id && (t.Status == models.TaskStatusSucceeded || t.Status == models.TaskStatusFailed) {
			continue
		}
		tasks = append(tasks, t)
	}
	s.state.Tasks = tasks

	// Cascade: remove schedule
	var scheds []models.Schedule
	for _, sc := range s.state.Schedules {
		if sc.SourceID != id {
			scheds = append(scheds, sc)
		}
	}
	s.state.Schedules = scheds

	return s.saveLocked()
}

func (s *Store) CreateSource(kind models.SourceKind, profile yt.SourceProfile, dateAfter string, autoDownload bool) (models.Source, models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Prevent duplicate sources with the same normalized URL
	for _, existing := range s.state.Sources {
		if existing.URL == profile.NormalizedURL {
			return models.Source{}, models.Task{}, fmt.Errorf("source already exists: %s (%s)", existing.Title, existing.URL)
		}
	}

	now := time.Now().UTC()
	src := models.Source{
		ID:                 fmt.Sprintf("src_%d", s.nextID),
		Kind:               kind,
		URL:                profile.NormalizedURL,
		InputURL:           profile.InputURL,
		Title:              profile.DisplayName,
		Locator:            profile.Locator,
		Host:               profile.Host,
		PathKind:           profile.PathKind,
		PathLabel:          profile.PathLabel,
		IntakeStage:        profile.IntakeStage,
		NextAction:         profile.NextAction,
		ValidationSummary:  profile.ValidationSummary,
		PrototypeReadiness: profile.PrototypeReadiness,
		Status:             models.SourceStatusActive,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	s.nextID++
	task := models.Task{
		ID:           fmt.Sprintf("task_%d", s.nextTask),
		Type:         "index",
		EntityType:   "source",
		EntityID:     src.ID,
		Status:       models.TaskStatusQueued,
		AutoDownload: autoDownload,
		DateAfter:    dateAfter,
		CreatedAt:    now,
	}
	s.nextTask++
	s.state.Sources = append(s.state.Sources, src)
	s.state.Tasks = append([]models.Task{task}, s.state.Tasks...)
	if err := s.saveLocked(); err != nil {
		return models.Source{}, models.Task{}, err
	}
	return src, task, nil
}

// UpdateSourceTitle updates a source's display title with the real name
// returned by yt-dlp after indexing (e.g. the actual channel or playlist name).
func (s *Store) UpdateSourceTitle(id, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Sources {
		if s.state.Sources[i].ID == id {
			s.state.Sources[i].Title = title
			s.state.Sources[i].UpdatedAt = time.Now().UTC()
			return s.saveLocked()
		}
	}
	return fmt.Errorf("source %s not found", id)
}

// ── Tasks ────────────────────────────────────────────────────────────────────

func (s *Store) ListTasks() []models.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Task, len(s.state.Tasks))
	copy(out, s.state.Tasks)
	return out
}

func (s *Store) GetTask(id string) (models.Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, task := range s.state.Tasks {
		if task.ID == id {
			return task, true
		}
	}
	return models.Task{}, false
}

// MarkTaskRunning atomically transitions a task from queued → running.
func (s *Store) MarkTaskRunning(id string) (models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Tasks {
		task := &s.state.Tasks[i]
		if task.ID != id {
			continue
		}
		if task.Status != models.TaskStatusQueued {
			return models.Task{}, &TaskNotRunnableError{ID: id, Status: task.Status}
		}
		task.Status = models.TaskStatusRunning
		if err := s.saveLocked(); err != nil {
			return models.Task{}, err
		}
		return *task, nil
	}
	return models.Task{}, fmt.Errorf("task %s not found", id)
}

// SucceedTask transitions a task from running → succeeded.
func (s *Store) SucceedTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Tasks {
		task := &s.state.Tasks[i]
		if task.ID != id {
			continue
		}
		now := time.Now().UTC()
		task.Status = models.TaskStatusSucceeded
		task.CompletedAt = &now
		return s.saveLocked()
	}
	return fmt.Errorf("task %s not found", id)
}

// FailTask transitions a task from running → failed.
func (s *Store) FailTask(id, code, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Tasks {
		task := &s.state.Tasks[i]
		if task.ID != id {
			continue
		}
		now := time.Now().UTC()
		task.Status = models.TaskStatusFailed
		task.CompletedAt = &now
		task.ErrorCode = code
		task.ErrorMessage = message
		return s.saveLocked()
	}
	return fmt.Errorf("task %s not found", id)
}

// RunTask is a stub that immediately transitions queued → succeeded.
// Used by tests and the /run endpoint for backward compatibility.
func (s *Store) RunTask(id string) (models.Task, error) {
	return s.executeTask(id, false)
}

// FailTaskStub is a stub that immediately transitions queued → failed.
func (s *Store) FailTaskStub(id string, code string, message string) (models.Task, error) {
	return s.executeTask(id, true, code, message)
}

func (s *Store) executeTask(id string, fail bool, failMeta ...string) (models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.state.Tasks {
		task := &s.state.Tasks[i]
		if task.ID != id {
			continue
		}
		if task.Status != models.TaskStatusQueued {
			return models.Task{}, &TaskNotRunnableError{ID: id, Status: task.Status}
		}

		task.Status = models.TaskStatusRunning
		task.ErrorCode = ""
		task.ErrorMessage = ""
		if err := s.saveLocked(); err != nil {
			return models.Task{}, err
		}

		now := time.Now().UTC()
		task.CompletedAt = &now
		if fail {
			task.Status = models.TaskStatusFailed
			if len(failMeta) > 0 {
				task.ErrorCode = failMeta[0]
			}
			if len(failMeta) > 1 {
				task.ErrorMessage = failMeta[1]
			}
		} else {
			task.Status = models.TaskStatusSucceeded
		}
		if err := s.saveLocked(); err != nil {
			return models.Task{}, err
		}
		return *task, nil
	}

	return models.Task{}, fmt.Errorf("task %s not found", id)
}

// ── Media Items ───────────────────────────────────────────────────────────────

func (s *Store) ListMediaItemsBySource(sourceID string) []models.MediaItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.MediaItem
	for _, item := range s.state.MediaItems {
		if item.SourceID == sourceID {
			out = append(out, item)
		}
	}
	return out
}

func (s *Store) GetMediaItem(id string) (models.MediaItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.state.MediaItems {
		if item.ID == id {
			return item, true
		}
	}
	return models.MediaItem{}, false
}

// CreateMediaItems inserts new media items, skipping any (sourceID, videoID)
// pairs that already exist. Returns the number of newly created items.
func (s *Store) CreateMediaItems(items []models.MediaItem) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	type key struct{ source, video string }
	existing := make(map[key]bool, len(s.state.MediaItems))
	for _, m := range s.state.MediaItems {
		existing[key{m.SourceID, m.VideoID}] = true
	}

	now := time.Now().UTC()
	created := 0
	for _, item := range items {
		k := key{item.SourceID, item.VideoID}
		if existing[k] {
			continue
		}
		item.ID = fmt.Sprintf("media_%d", s.nextMedia)
		item.CreatedAt = now
		item.UpdatedAt = now
		s.nextMedia++
		s.state.MediaItems = append(s.state.MediaItems, item)
		existing[k] = true
		created++
	}
	if created == 0 {
		return 0, nil
	}
	return created, s.saveLocked()
}

// UpdateMediaItemStatus updates the status, optional filePath, and optional
// errorMsg of a media item.
func (s *Store) UpdateMediaItemStatus(id string, status models.MediaItemStatus, filePath, errorMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.MediaItems {
		item := &s.state.MediaItems[i]
		if item.ID != id {
			continue
		}
		// Always overwrite both fields so retries never carry stale state.
		item.Status = status
		item.FilePath = filePath
		item.ErrorMsg = errorMsg
		item.UpdatedAt = time.Now().UTC()
		return s.saveLocked()
	}
	return fmt.Errorf("media item %s not found", id)
}

// CreateDownloadTask creates a queued download task for a single media item.
// Returns an error if the item does not exist or already has an active
// (queued or running) download task.
func (s *Store) CreateDownloadTask(mediaItemID string) (models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	itemFound := false
	for _, item := range s.state.MediaItems {
		if item.ID == mediaItemID {
			itemFound = true
			break
		}
	}
	if !itemFound {
		return models.Task{}, fmt.Errorf("media item %s not found", mediaItemID)
	}
	for _, task := range s.state.Tasks {
		if task.Type == "download" && task.EntityID == mediaItemID &&
			(task.Status == models.TaskStatusQueued || task.Status == models.TaskStatusRunning) {
			return models.Task{}, fmt.Errorf("media item %s already has an active download task", mediaItemID)
		}
	}

	now := time.Now().UTC()
	task := models.Task{
		ID:         fmt.Sprintf("task_%d", s.nextTask),
		Type:       "download",
		EntityType: "media",
		EntityID:   mediaItemID,
		Status:     models.TaskStatusQueued,
		CreatedAt:  now,
	}
	s.nextTask++
	s.state.Tasks = append([]models.Task{task}, s.state.Tasks...)
	if err := s.saveLocked(); err != nil {
		return models.Task{}, err
	}
	return task, nil
}

// CreateDownloadTasksForSource creates queued download tasks for all pending
// media items belonging to the given source that do not already have an active
// (queued or running) download task. Returns an error if the source does not exist.
func (s *Store) CreateDownloadTasksForSource(sourceID string) ([]models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sourceFound := false
	for _, src := range s.state.Sources {
		if src.ID == sourceID {
			sourceFound = true
			break
		}
	}
	if !sourceFound {
		return nil, fmt.Errorf("source %s not found", sourceID)
	}

	// Collect media item IDs that already have an active download task.
	alreadyQueued := make(map[string]bool)
	for _, task := range s.state.Tasks {
		if task.Type == "download" &&
			(task.Status == models.TaskStatusQueued || task.Status == models.TaskStatusRunning) {
			alreadyQueued[task.EntityID] = true
		}
	}

	now := time.Now().UTC()
	var tasks []models.Task
	for _, item := range s.state.MediaItems {
		if item.SourceID == sourceID &&
			item.Status == models.MediaItemStatusPending &&
			!alreadyQueued[item.ID] {
			task := models.Task{
				ID:         fmt.Sprintf("task_%d", s.nextTask),
				Type:       "download",
				EntityType: "media",
				EntityID:   item.ID,
				Status:     models.TaskStatusQueued,
				CreatedAt:  now,
			}
			s.nextTask++
			tasks = append(tasks, task)
			alreadyQueued[item.ID] = true // prevent duplicate within this call
		}
	}
	if len(tasks) == 0 {
		return nil, nil
	}
	s.state.Tasks = append(tasks, s.state.Tasks...)
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return tasks, nil
}

// CreateIndexTask creates a queued index task for a source. Returns an error
// if the source does not exist or already has a queued/running index task.
func (s *Store) CreateIndexTask(sourceID string, autoDownload bool, dateAfter string) (models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sourceFound := false
	for _, src := range s.state.Sources {
		if src.ID == sourceID {
			sourceFound = true
			break
		}
	}
	if !sourceFound {
		return models.Task{}, fmt.Errorf("source %s not found", sourceID)
	}
	for _, task := range s.state.Tasks {
		if task.Type == "index" && task.EntityID == sourceID &&
			(task.Status == models.TaskStatusQueued || task.Status == models.TaskStatusRunning) {
			return models.Task{}, fmt.Errorf("source %s already has an active index task", sourceID)
		}
	}

	now := time.Now().UTC()
	task := models.Task{
		ID:           fmt.Sprintf("task_%d", s.nextTask),
		Type:         "index",
		EntityType:   "source",
		EntityID:     sourceID,
		Status:       models.TaskStatusQueued,
		AutoDownload: autoDownload,
		DateAfter:    dateAfter,
		CreatedAt:    now,
	}
	s.nextTask++
	s.state.Tasks = append([]models.Task{task}, s.state.Tasks...)
	if err := s.saveLocked(); err != nil {
		return models.Task{}, err
	}
	return task, nil
}

// DeleteTask removes a single completed task.
func (s *Store) DeleteTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.state.Tasks {
		if t.ID == id {
			if t.Status == models.TaskStatusQueued || t.Status == models.TaskStatusRunning {
				return fmt.Errorf("cannot delete %s task %s", t.Status, id)
			}
			s.state.Tasks = append(s.state.Tasks[:i], s.state.Tasks[i+1:]...)
			return s.saveLocked()
		}
	}
	return fmt.Errorf("task %s not found", id)
}

// DeleteCompletedTasks removes all succeeded and failed tasks.
func (s *Store) DeleteCompletedTasks() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	var kept []models.Task
	removed := 0
	for _, t := range s.state.Tasks {
		if t.Status == models.TaskStatusSucceeded || t.Status == models.TaskStatusFailed {
			removed++
		} else {
			kept = append(kept, t)
		}
	}
	if removed > 0 {
		s.state.Tasks = kept
		_ = s.saveLocked()
	}
	return removed
}

// ── Schedules ─────────────────────────────────────────────────────────────────

func (s *Store) ListSchedules() []models.Schedule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Schedule, len(s.state.Schedules))
	copy(out, s.state.Schedules)
	return out
}

func (s *Store) GetSchedule(sourceID string) (models.Schedule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sched := range s.state.Schedules {
		if sched.SourceID == sourceID {
			return sched, true
		}
	}
	return models.Schedule{}, false
}

// UpsertSchedule creates or replaces the schedule for a source.
func (s *Store) UpsertSchedule(sourceID string, intervalHours int, enabled, autoDownload bool, dateAfter string) (models.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sourceFound := false
	for _, src := range s.state.Sources {
		if src.ID == sourceID {
			sourceFound = true
			break
		}
	}
	if !sourceFound {
		return models.Schedule{}, fmt.Errorf("source %s not found", sourceID)
	}
	if intervalHours < 1 {
		return models.Schedule{}, fmt.Errorf("intervalHours must be >= 1")
	}

	now := time.Now().UTC()
	for i := range s.state.Schedules {
		sched := &s.state.Schedules[i]
		if sched.SourceID != sourceID {
			continue
		}
		sched.IntervalHours = intervalHours
		sched.Enabled = enabled
		sched.AutoDownload = autoDownload
		sched.DateAfter = dateAfter
		sched.NextRunAt = now.Add(time.Duration(intervalHours) * time.Hour)
		sched.UpdatedAt = now
		if err := s.saveLocked(); err != nil {
			return models.Schedule{}, err
		}
		return *sched, nil
	}

	sched := models.Schedule{
		SourceID:      sourceID,
		IntervalHours: intervalHours,
		Enabled:       enabled,
		AutoDownload:  autoDownload,
		DateAfter:     dateAfter,
		NextRunAt:     now.Add(time.Duration(intervalHours) * time.Hour),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	s.state.Schedules = append(s.state.Schedules, sched)
	if err := s.saveLocked(); err != nil {
		return models.Schedule{}, err
	}
	return sched, nil
}

// DeleteSchedule removes the schedule for a source.
func (s *Store) DeleteSchedule(sourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sched := range s.state.Schedules {
		if sched.SourceID == sourceID {
			s.state.Schedules = append(s.state.Schedules[:i], s.state.Schedules[i+1:]...)
			return s.saveLocked()
		}
	}
	return fmt.Errorf("schedule for source %s not found", sourceID)
}

// ListDueSchedules returns enabled schedules whose NextRunAt is on or before now.
func (s *Store) ListDueSchedules(now time.Time) []models.Schedule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.Schedule
	for _, sched := range s.state.Schedules {
		if sched.Enabled && !sched.NextRunAt.After(now) {
			out = append(out, sched)
		}
	}
	return out
}

// TouchSchedule records that a schedule fired: sets LastRunAt and advances NextRunAt.
func (s *Store) TouchSchedule(sourceID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Schedules {
		sched := &s.state.Schedules[i]
		if sched.SourceID != sourceID {
			continue
		}
		// Re-check Enabled inside the lock: if the user disabled the schedule
		// between ListDueSchedules (snapshot) and here, treat as a no-op so we
		// don't advance NextRunAt for a disabled schedule.
		if !sched.Enabled {
			return nil
		}
		sched.LastRunAt = &now
		sched.NextRunAt = now.Add(time.Duration(sched.IntervalHours) * time.Hour)
		sched.UpdatedAt = now
		return s.saveLocked()
	}
	return fmt.Errorf("schedule for source %s not found", sourceID)
}
