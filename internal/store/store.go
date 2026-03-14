package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ytvault/internal/models"
	"ytvault/internal/yt"
)

type Store struct {
	mu       sync.RWMutex
	path     string
	state    models.State
	nextID   int
	nextTask int
}

func New(path string) (*Store, error) {
	s := &Store{path: path, nextID: 1, nextTask: 1}
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

func (s *Store) ListSources() []models.Source {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Source, len(s.state.Sources))
	copy(out, s.state.Sources)
	return out
}

func (s *Store) CreateSource(kind models.SourceKind, profile yt.SourceProfile) (models.Source, models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
		ID:         fmt.Sprintf("task_%d", s.nextTask),
		Type:       "index",
		EntityType: "source",
		EntityID:   src.ID,
		Status:     models.TaskStatusQueued,
		CreatedAt:  now,
	}
	s.nextTask++
	s.state.Sources = append(s.state.Sources, src)
	s.state.Tasks = append([]models.Task{task}, s.state.Tasks...)
	if err := s.saveLocked(); err != nil {
		return models.Source{}, models.Task{}, err
	}
	return src, task, nil
}

func (s *Store) ListTasks() []models.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Task, len(s.state.Tasks))
	copy(out, s.state.Tasks)
	return out
}

func (s *Store) RunTask(id string) (models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.state.Tasks {
		task := &s.state.Tasks[i]
		if task.ID != id {
			continue
		}
		if task.Status != models.TaskStatusQueued {
			return models.Task{}, fmt.Errorf("task %s is not runnable from status %s", id, task.Status)
		}

		task.Status = models.TaskStatusRunning
		if err := s.saveLocked(); err != nil {
			return models.Task{}, err
		}

		now := time.Now().UTC()
		task.Status = models.TaskStatusSucceeded
		task.CompletedAt = &now
		if err := s.saveLocked(); err != nil {
			return models.Task{}, err
		}
		return *task, nil
	}

	return models.Task{}, fmt.Errorf("task %s not found", id)
}
