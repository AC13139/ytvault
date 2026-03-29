// Package scheduler runs periodic re-index (and optional auto-download) jobs
// for sources that have an enabled Schedule.
package scheduler

import (
	"log"
	"sync"
	"time"

	"ytvault/internal/store"
	"ytvault/internal/worker"
)

// Scheduler checks due schedules on a regular tick and starts index tasks.
type Scheduler struct {
	store  *store.Store
	worker *worker.Worker
	done   chan struct{}
	once   sync.Once
	wg     sync.WaitGroup
}

// New creates a Scheduler. Call Start to begin background processing.
func New(s *store.Store, w *worker.Worker) *Scheduler {
	return &Scheduler{store: s, worker: w, done: make(chan struct{})}
}

// Start launches the background goroutine. Safe to call only once.
func (sc *Scheduler) Start() {
	sc.wg.Add(1)
	go func() {
		defer sc.wg.Done()
		sc.run()
	}()
}

// Stop signals the scheduler to exit and waits for the goroutine to return.
// Idempotent — safe to call multiple times.
func (sc *Scheduler) Stop() {
	sc.once.Do(func() { close(sc.done) })
	sc.wg.Wait()
}

func (sc *Scheduler) run() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	// Check immediately on startup so a missed schedule fires right away.
	log.Printf("scheduler: started, checking for due schedules")
	sc.tick(time.Now().UTC())

	for {
		select {
		case <-sc.done:
			return
		case t := <-ticker.C:
			sc.tick(t.UTC())
		}
	}
}

func (sc *Scheduler) tick(now time.Time) {
	due := sc.store.ListDueSchedules(now)
	if len(due) > 0 {
		log.Printf("scheduler: %d due schedule(s) at %s", len(due), now.Format("15:04:05"))
	}
	for _, sched := range due {
		// Attempt to create the index task first. This check is atomic
		// (inside the store lock), so it prevents duplicate active tasks.
		task, err := sc.store.CreateIndexTask(sched.SourceID, sched.AutoDownload, sched.DateAfter)
		if err != nil {
			// Task creation failed (e.g. already an active index task running).
			// Still advance NextRunAt to prevent a tight retry loop on the next tick.
			if touchErr := sc.store.TouchSchedule(sched.SourceID, now); touchErr != nil {
				log.Printf("scheduler: touch schedule for source %s: %v", sched.SourceID, touchErr)
			}
			log.Printf("scheduler: source %s skipped: %v", sched.SourceID, err)
			continue
		}

		// Task created — advance the schedule so the next run fires after the
		// full interval. TouchSchedule re-checks Enabled inside the lock; if
		// the user disabled the schedule between ListDueSchedules and here,
		// it is a no-op (the created task still runs this once, which is
		// acceptable).
		if err := sc.store.TouchSchedule(sched.SourceID, now); err != nil {
			log.Printf("scheduler: touch schedule for source %s: %v", sched.SourceID, err)
			// Non-fatal: the task exists and will run; the schedule counter may be stale.
		}

		if _, err := sc.worker.StartTask(task.ID); err != nil {
			log.Printf("scheduler: start task %s for source %s: %v", task.ID, sched.SourceID, err)
		} else {
			log.Printf("scheduler: started index task %s for source %s", task.ID, sched.SourceID)
		}
	}
}
