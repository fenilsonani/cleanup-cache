package daemon

import (
	"fmt"
	"sync"
	"time"

	"github.com/fenilsonani/cleanup-cache/internal/config"
	"github.com/robfig/cron/v3"
)

// CleanupJob represents a scheduled cleanup job
type CleanupJob struct {
	Name       string
	Schedule   string
	Categories map[string]bool
	DryRun     bool
	SkipIfBusy bool
	NextRun    time.Time
	LastRun    time.Time
}

// Scheduler manages scheduled cleanup jobs
type Scheduler struct {
	daemon    *Daemon
	cron      *cron.Cron
	jobs      map[string]cron.EntryID
	jobsMu    sync.RWMutex
	running   bool
	schedules []config.CleanupSchedule
}

// NewScheduler creates a new scheduler
func NewScheduler(daemon *Daemon, schedules []config.CleanupSchedule) *Scheduler {
	// Create cron with seconds support
	parser := cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)

	c := cron.New(cron.WithParser(parser), cron.WithChain(
		cron.Recover(cron.DefaultLogger),
	))

	return &Scheduler{
		daemon:    daemon,
		cron:      c,
		jobs:      make(map[string]cron.EntryID),
		schedules: schedules,
	}
}

// Start starts the scheduler
func (s *Scheduler) Start() error {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

	if s.running {
		return fmt.Errorf("scheduler already running")
	}

	// Add all configured schedules
	for _, schedule := range s.schedules {
		if err := s.addJobInternal(schedule); err != nil {
			return fmt.Errorf("failed to add schedule %s: %w", schedule.Name, err)
		}
	}

	// Start cron
	s.cron.Start()
	s.running = true

	s.daemon.logger.Info("Scheduler started with %d jobs", len(s.jobs))
	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

	if !s.running {
		return
	}

	ctx := s.cron.Stop()
	select {
	case <-ctx.Done():
		// Clean shutdown
	case <-time.After(10 * time.Second):
		s.daemon.logger.Warn("Scheduler stop timed out")
	}

	s.running = false
	s.daemon.logger.Info("Scheduler stopped")
}

// addJobInternal adds a job (internal, no lock)
func (s *Scheduler) addJobInternal(schedule config.CleanupSchedule) error {
	if _, exists := s.jobs[schedule.Name]; exists {
		return fmt.Errorf("job %s already exists", schedule.Name)
	}

	// Create job
	job := &CleanupJob{
		Name:       schedule.Name,
		Schedule:   schedule.Schedule,
		Categories: schedule.Categories,
		DryRun:     schedule.DryRun,
		SkipIfBusy: schedule.SkipIfBusy,
	}

	// Create job function
	jobFunc := func() {
		s.daemon.logger.Info("Executing scheduled job: %s", job.Name)
		job.LastRun = time.Now()

		if err := s.daemon.RunCleanupJob(job); err != nil {
			s.daemon.logger.Error("Job %s failed: %v", job.Name, err)
		}
	}

	// Add to cron
	id, err := s.cron.AddFunc(schedule.Schedule, jobFunc)
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	s.jobs[schedule.Name] = id

	// Get next run time
	entry := s.cron.Entry(id)
	job.NextRun = entry.Next

	s.daemon.logger.Info("Added job: %s, next run: %v", schedule.Name, job.NextRun)
	return nil
}

// AddJob adds a new job to the scheduler
func (s *Scheduler) AddJob(schedule config.CleanupSchedule) error {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	return s.addJobInternal(schedule)
}

// RemoveJob removes a job from the scheduler
func (s *Scheduler) RemoveJob(name string) error {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

	id, exists := s.jobs[name]
	if !exists {
		return fmt.Errorf("job %s not found", name)
	}

	s.cron.Remove(id)
	delete(s.jobs, name)

	s.daemon.logger.Info("Removed job: %s", name)
	return nil
}

// GetNextRun returns the next run time for a job
func (s *Scheduler) GetNextRun(name string) (time.Time, error) {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()

	id, exists := s.jobs[name]
	if !exists {
		return time.Time{}, fmt.Errorf("job %s not found", name)
	}

	entry := s.cron.Entry(id)
	return entry.Next, nil
}

// ListJobs returns information about all jobs
func (s *Scheduler) ListJobs() []JobInfo {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()

	jobs := make([]JobInfo, 0, len(s.jobs))
	entries := s.cron.Entries()

	for _, entry := range entries {
		// Find job name by ID
		var name string
		for n, id := range s.jobs {
			if id == entry.ID {
				name = n
				break
			}
		}

		if name != "" {
			jobs = append(jobs, JobInfo{
				Name:    name,
				NextRun: entry.Next,
				PrevRun: entry.Prev,
			})
		}
	}

	return jobs
}

// TriggerJob manually triggers a job
func (s *Scheduler) TriggerJob(name string) error {
	s.jobsMu.RLock()
	id, exists := s.jobs[name]
	s.jobsMu.RUnlock()

	if !exists {
		return fmt.Errorf("job %s not found", name)
	}

	// Find the schedule config
	var schedule config.CleanupSchedule
	for _, sched := range s.schedules {
		if sched.Name == name {
			schedule = sched
			break
		}
	}

	// Create and run job
	job := &CleanupJob{
		Name:       schedule.Name,
		Schedule:   schedule.Schedule,
		Categories: schedule.Categories,
		DryRun:     schedule.DryRun,
		SkipIfBusy: schedule.SkipIfBusy,
	}

	s.daemon.logger.Info("Manually triggering job: %s (entry ID: %d)", name, id)
	return s.daemon.RunCleanupJob(job)
}

// JobInfo contains information about a scheduled job
type JobInfo struct {
	Name    string
	NextRun time.Time
	PrevRun time.Time
}
