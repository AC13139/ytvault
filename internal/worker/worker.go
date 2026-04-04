// Package worker executes index and download tasks against real YouTube sources.
package worker

import (
	"context"
	"fmt"
	"log"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ytvault/internal/models"
	"ytvault/internal/store"
	"ytvault/internal/ytdlp"
)

// Worker executes tasks using yt-dlp.
type Worker struct {
	store       *store.Store
	downloadDir string

	mu      sync.Mutex
	cancels map[string]context.CancelFunc // taskID → cancel
	dlSem   chan struct{}                 // download concurrency limiter
}

// New creates a Worker. downloadDir is where video files are saved.
func New(s *store.Store, downloadDir string) *Worker {
	return &Worker{store: s, downloadDir: downloadDir, cancels: make(map[string]context.CancelFunc)}
}

// CancelTask stops a running task.
func (w *Worker) CancelTask(taskID string) error {
	w.mu.Lock()
	cancel, ok := w.cancels[taskID]
	w.mu.Unlock()
	if !ok {
		return fmt.Errorf("task %s is not running or already finished", taskID)
	}
	cancel()
	return nil
}

// StartTask atomically marks the task as running then executes it in a
// goroutine. Returns the task in running state. Callers get a 202-style
// response immediately; the goroutine updates the task on completion.
func (w *Worker) StartTask(taskID string) (models.Task, error) {
	// Verify task is queued before starting
	task, ok := w.store.GetTask(taskID)
	if !ok {
		return models.Task{}, fmt.Errorf("task %s not found", taskID)
	}
	if task.Status != models.TaskStatusQueued {
		return models.Task{}, fmt.Errorf("task %s is %s, not queued", taskID, task.Status)
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.mu.Lock()
	w.cancels[taskID] = cancel
	w.mu.Unlock()
	go func() {
		defer func() {
			cancel()
			w.mu.Lock()
			delete(w.cancels, taskID)
			w.mu.Unlock()
		}()
		w.execute(ctx, task)
	}()
	return task, nil
}

func (w *Worker) execute(ctx context.Context, task models.Task) {
	var execErr error
	switch task.Type {
	case "index":
		// Mark running immediately for index tasks
		updated, err := w.store.MarkTaskRunning(task.ID)
		if err != nil {
			log.Printf("worker: mark running %s: %v", task.ID, err)
			execErr = err
		} else {
			execErr = w.runIndex(updated)
		}
	case "download":
		// Respect max concurrent downloads — stay queued until slot available
		maxC := w.store.GetConfig().MaxConcurrent
		if maxC < 1 {
			maxC = 1
		}
		w.mu.Lock()
		if w.dlSem == nil {
			w.dlSem = make(chan struct{}, maxC)
		} else if cap(w.dlSem) != maxC {
			w.dlSem = make(chan struct{}, maxC)
		}
		sem := w.dlSem
		w.mu.Unlock()
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		case <-ctx.Done():
			execErr = fmt.Errorf("cancelled while waiting for download slot")
		}
		if execErr == nil {
			// Now we have the slot — mark as running
			if _, err := w.store.MarkTaskRunning(task.ID); err != nil {
				log.Printf("worker: mark running %s: %v", task.ID, err)
			}
			execErr = w.runDownload(ctx, task)
		}
	default:
		execErr = fmt.Errorf("unknown task type: %s", task.Type)
	}

	if execErr != nil {
		code := "execution_failed"
		if ctx.Err() != nil {
			code = "cancelled"
			execErr = fmt.Errorf("task cancelled by user")
		}
		log.Printf("worker: task %s (%s) failed: %v", task.ID, task.Type, execErr)
		if err := w.store.FailTask(task.ID, code, execErr.Error()); err != nil {
			log.Printf("worker: fail task %s: %v", task.ID, err)
		}
		return
	}
	log.Printf("worker: task %s (%s) succeeded", task.ID, task.Type)
	if err := w.store.SucceedTask(task.ID); err != nil {
		log.Printf("worker: succeed task %s: %v", task.ID, err)
	}
}

func (w *Worker) runIndex(task models.Task) error {
	src, ok := w.store.GetSource(task.EntityID)
	if !ok {
		return fmt.Errorf("source %s not found", task.EntityID)
	}

	log.Printf("worker: indexing source %s (%s) kind=%s dateAfter=%q", src.ID, src.URL, src.Kind, task.DateAfter)
	title, posterURL, videos, err := ytdlp.Index(src.URL, task.DateAfter)
	if err != nil {
		return fmt.Errorf("index %s: %w", src.URL, err)
	}
	log.Printf("worker: index result for %s: title=%q videos=%d", src.ID, title, len(videos))
	for i, v := range videos {
		if i < 5 {
			log.Printf("worker:   [%d] id=%s title=%q url=%s", i, v.ID, v.Title, v.URL)
		}
	}

	// Update the source title with the real name from yt-dlp
	if title != "" {
		if err := w.store.UpdateSourceTitle(src.ID, title); err != nil {
			log.Printf("worker: update title for source %s: %v", src.ID, err)
		}
	}

	// Auto-download poster if not already set
	if posterURL != "" && src.PosterPath == "" {
		posterDir := filepath.Join(w.downloadDir, ".posters")
		if err := os.MkdirAll(posterDir, 0o755); err == nil {
			posterPath := filepath.Join(posterDir, src.ID+".jpg")
			if err := downloadPoster(posterURL, posterPath); err != nil {
				log.Printf("worker: poster download for source %s: %v", src.ID, err)
			} else {
				if err := w.store.UpdateSourcePoster(src.ID, posterPath, posterURL); err != nil {
					log.Printf("worker: update poster for source %s: %v", src.ID, err)
				} else {
					log.Printf("worker: poster saved for source %s", src.ID)
				}
			}
		}
	}

	items := make([]models.MediaItem, len(videos))
	for i, v := range videos {
		items[i] = models.MediaItem{
			SourceID:      src.ID,
			VideoID:       v.ID,
			Title:         v.Title,
			Channel:       v.Channel,
			URL:           v.URL,
			Duration:      v.Duration,
			PlaylistIndex: i + 1, // 1-based
			Status:        models.MediaItemStatusPending,
		}
		if v.UploadDate != "" {
			if t, err := time.Parse("20060102", v.UploadDate); err == nil {
				items[i].UploadDate = &t
			}
		}
	}

	created, err := w.store.CreateMediaItems(items)
	if err != nil {
		return err
	}
	log.Printf("worker: created %d new media item(s) for source %s", created, src.ID)

	// Auto-download: immediately queue and start downloads for new videos.
	log.Printf("worker: source %s autoDownload=%v created=%d", src.ID, task.AutoDownload, created)
	if task.AutoDownload && created > 0 {
		dlTasks, err := w.store.CreateDownloadTasksForSource(src.ID)
		if err != nil {
			log.Printf("worker: auto-download setup for source %s: %v", src.ID, err)
			return nil // index itself succeeded
		}
		for _, dt := range dlTasks {
			if _, err := w.StartTask(dt.ID); err != nil {
				log.Printf("worker: auto-start download task %s: %v", dt.ID, err)
			}
		}
		log.Printf("worker: auto-started %d download task(s) for source %s", len(dlTasks), src.ID)
	}

	// Auto-cleanup: -1 = keep forever, 0 = use global default, >0 = specific days
	retDays := src.RetentionDays
	if retDays == -1 {
		retDays = 0
	} else if retDays == 0 {
		retDays = w.store.GetConfig().DefaultRetentionDays
	}
	if retDays > 0 {
		filePaths, removed := w.store.CleanupSource(src.ID, retDays)
		if removed > 0 {
			for _, fp := range filePaths {
				os.Remove(fp)
			}
			log.Printf("worker: auto-cleanup removed %d old item(s) for source %s", removed, src.ID)
		}
	}

	return nil
}

func (w *Worker) runDownload(ctx context.Context, task models.Task) error {
	item, ok := w.store.GetMediaItem(task.EntityID)
	if !ok {
		return fmt.Errorf("media item %s not found", task.EntityID)
	}

	log.Printf("worker: downloading %s — %s", item.VideoID, item.Title)
	if err := w.store.UpdateMediaItemStatus(item.ID, models.MediaItemStatusDownloading, "", "", 0); err != nil {
		return err
	}

	cfg := w.store.GetConfig()
	// Use playlist-specific template if the source is a playlist
	tmpl := cfg.FilenameTemplate
	if src, ok := w.store.GetSource(item.SourceID); ok && src.Kind == models.SourceKindPlaylist {
		tmpl = cfg.PlaylistTemplate
	}
	// Replace playlist-specific template vars — yt-dlp doesn't have playlist
	// context when downloading individual video URLs
	if src, ok := w.store.GetSource(item.SourceID); ok {
		// Replace playlist_title with the source name (playlist or channel name)
		tmpl = strings.Replace(tmpl, "%(playlist_title,uploader,channel,id)s", src.Title, 1)
	}
	if item.PlaylistIndex > 0 {
		tmpl = strings.Replace(tmpl, "%(playlist_index)03d", fmt.Sprintf("%03d", item.PlaylistIndex), 1)
	}
	filePath, err := ytdlp.Download(ctx, item.URL, w.downloadDir, tmpl, cfg.VideoFormat)
	if err != nil {
		dlErr := fmt.Errorf("download %s: %w", item.URL, err)
		_ = w.store.UpdateMediaItemStatus(item.ID, models.MediaItemStatusFailed, "", dlErr.Error(), 0)
		return dlErr
	}

	// Get file size
	var fileSize int64
	if fi, err := os.Stat(filePath); err == nil {
		fileSize = fi.Size()
	}
	return w.store.UpdateMediaItemStatus(item.ID, models.MediaItemStatusDone, filePath, "", fileSize)
}

// downloadPoster downloads an image from url and saves it to destPath.
// Validates content type and enforces a 2MB size limit.
func downloadPoster(url, destPath string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		return fmt.Errorf("not an image: %s", ct)
	}

	// Limit to 2MB
	limited := io.LimitReader(resp.Body, 2<<20)
	tmp := destPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, limited); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()
	return os.Rename(tmp, destPath)
}
