package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/quailyquaily/mistermorph/db/models"
	"gorm.io/gorm"
)

const (
	StatusQueued   = "queued"
	StatusRunning  = "running"
	StatusSuccess  = "succeeded"
	StatusFailed   = "failed"
	StatusCanceled = "canceled"
	StatusTimedOut = "timed_out"
	StatusSkipped  = "skipped"

	overlapForbid = "forbid"

	defaultTimeout = 10 * time.Minute
)

type Config struct {
	Enabled     bool
	Concurrency int
	Tick        time.Duration

	// Max characters stored in cron_runs.error/result_summary (bounded metadata only).
	MaxErrorChars   int
	MaxSummaryChars int

	// Optional callback invoked after a run is finished and persisted.
	// This can be used to deliver notifications (e.g., Telegram) in higher-level runtimes.
	OnRunFinished func(ctx context.Context, job models.CronJob, run models.CronRun, status string, errStr *string, summary *string) error
}

func DefaultConfig() Config {
	return Config{
		Enabled:         false,
		Concurrency:     1,
		Tick:            1 * time.Second,
		MaxErrorChars:   2000,
		MaxSummaryChars: 1000,
		OnRunFinished:   nil,
	}
}

type TaskRunner func(ctx context.Context, task string, model string, meta map[string]any) (resultSummary *string, err error)

type Scheduler struct {
	db           *gorm.DB
	log          *slog.Logger
	cfg          Config
	defaultModel string
	runner       TaskRunner

	wg sync.WaitGroup

	wakeCh chan struct{}
}

func New(db *gorm.DB, defaultModel string, runner TaskRunner, cfg Config, log *slog.Logger) (*Scheduler, error) {
	if db == nil {
		return nil, fmt.Errorf("nil db")
	}
	if runner == nil {
		return nil, fmt.Errorf("nil runner")
	}
	if strings.TrimSpace(defaultModel) == "" {
		return nil, fmt.Errorf("missing default model")
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.Tick <= 0 {
		cfg.Tick = 1 * time.Second
	}
	if cfg.MaxErrorChars <= 0 {
		cfg.MaxErrorChars = 2000
	}
	if cfg.MaxSummaryChars <= 0 {
		cfg.MaxSummaryChars = 1000
	}
	if log == nil {
		log = slog.Default()
	}
	return &Scheduler{
		db:           db,
		log:          log,
		cfg:          cfg,
		defaultModel: defaultModel,
		runner:       runner,
		wakeCh:       make(chan struct{}, 1),
	}, nil
}

func (s *Scheduler) Start(ctx context.Context) error {
	if !s.cfg.Enabled {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.recoverOrphanedRuns(ctx); err != nil {
		return err
	}
	if err := s.reconcileNextRunAt(ctx, time.Now().UTC().Unix()); err != nil {
		return err
	}

	s.log.Info("scheduler_start", "concurrency", s.cfg.Concurrency, "tick_ms", s.cfg.Tick.Milliseconds())

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.scheduleLoop(ctx)
	}()

	for i := 0; i < s.cfg.Concurrency; i++ {
		s.wg.Add(1)
		go func(workerID int) {
			defer s.wg.Done()
			s.workerLoop(ctx, workerID)
		}(i + 1)
	}

	// Kick workers to process any pre-existing queued runs on startup.
	s.wakeWorkers()
	return nil
}

func (s *Scheduler) Wait() {
	s.wg.Wait()
}

func (s *Scheduler) wakeWorkers() {
	select {
	case s.wakeCh <- struct{}{}:
	default:
	}
}

func (s *Scheduler) recoverOrphanedRuns(ctx context.Context) error {
	now := time.Now().UTC().Unix()
	msg := "process restarted"
	res := s.db.WithContext(ctx).
		Model(&models.CronRun{}).
		Where("status = ?", StatusRunning).
		Updates(map[string]any{
			"status":      StatusFailed,
			"finished_at": now,
			"error":       msg,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		s.log.Warn("scheduler_recovered_orphaned_runs", "count", res.RowsAffected)
	}
	return nil
}

func (s *Scheduler) scheduleLoop(ctx context.Context) {
	t := time.NewTicker(s.cfg.Tick)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("scheduler_stop", "reason", ctx.Err().Error())
			return
		case <-t.C:
			now := time.Now().UTC().Unix()
			if err := s.tick(ctx, now); err != nil {
				s.log.Warn("scheduler_tick_error", "error", err.Error())
			}
		}
	}
}

func (s *Scheduler) tick(ctx context.Context, now int64) error {
	// Set NextRunAt for any enabled jobs missing it.
	if err := s.reconcileMissingNextRunAt(ctx, now); err != nil {
		return err
	}

	var due []models.CronJob
	if err := s.db.WithContext(ctx).
		Where("enabled = ?", true).
		Where("next_run_at IS NOT NULL AND next_run_at <= ?", now).
		Find(&due).Error; err != nil {
		return err
	}
	for _, job := range due {
		if _, err := s.enqueueJobIfDue(ctx, job.ID, now); err != nil {
			s.log.Warn("scheduler_enqueue_error", "job_id", job.ID, "error", err.Error())
		}
	}
	return nil
}

func (s *Scheduler) reconcileMissingNextRunAt(ctx context.Context, now int64) error {
	var jobs []models.CronJob
	if err := s.db.WithContext(ctx).
		Where("enabled = ?", true).
		Where("next_run_at IS NULL").
		Find(&jobs).Error; err != nil {
		return err
	}
	for _, job := range jobs {
		next, err := nextRunAt(job, now)
		if err != nil {
			s.log.Warn("scheduler_job_invalid", "job_id", job.ID, "error", err.Error())
			_ = s.db.WithContext(ctx).Model(&models.CronJob{}).Where("id = ?", job.ID).Update("enabled", false).Error
			continue
		}
		_ = s.db.WithContext(ctx).Model(&models.CronJob{}).Where("id = ?", job.ID).Update("next_run_at", next).Error
	}
	return nil
}

func (s *Scheduler) reconcileNextRunAt(ctx context.Context, now int64) error {
	var jobs []models.CronJob
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Find(&jobs).Error; err != nil {
		return err
	}

	for _, job := range jobs {
		next, err := nextRunAt(job, now)
		if err != nil {
			s.log.Warn("scheduler_job_invalid", "job_id", job.ID, "error", err.Error())
			_ = s.db.WithContext(ctx).Model(&models.CronJob{}).Where("id = ?", job.ID).Update("enabled", false).Error
			continue
		}
		// Hardcoded misfire=skip: if next_run_at is in the past, advance it to the next future time.
		if job.NextRunAt == nil || *job.NextRunAt < now {
			_ = s.db.WithContext(ctx).Model(&models.CronJob{}).Where("id = ?", job.ID).Update("next_run_at", next).Error
		}
	}
	return nil
}

func (s *Scheduler) enqueueJobIfDue(ctx context.Context, jobID string, now int64) (bool, error) {
	queued := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job models.CronJob
		if err := tx.Where("id = ?", jobID).First(&job).Error; err != nil {
			return err
		}
		if !job.Enabled || job.NextRunAt == nil {
			return nil
		}
		scheduledFor := *job.NextRunAt
		if scheduledFor > now {
			return nil
		}

		var updates map[string]any
		if job.RunOnce {
			updates = map[string]any{
				"enabled":     false,
				"last_run_at": scheduledFor,
				"next_run_at": nil,
			}
		} else {
			next, err := nextRunAt(job, scheduledFor)
			if err != nil {
				_ = tx.Model(&models.CronJob{}).Where("id = ?", job.ID).Update("enabled", false).Error
				return fmt.Errorf("compute next: %w", err)
			}
			updates = map[string]any{
				"last_run_at": scheduledFor,
				"next_run_at": next,
			}
		}

		var runningCount int64
		if err := tx.Model(&models.CronRun{}).Where("job_id = ? AND status = ?", job.ID, StatusRunning).Count(&runningCount).Error; err != nil {
			return err
		}

		policy := strings.ToLower(strings.TrimSpace(job.OverlapPolicy))
		if policy == "" {
			policy = overlapForbid
		}

		if runningCount > 0 && policy == overlapForbid {
			msg := "overlap_forbid: prior run still running"
			s.log.Info("scheduler_overlap_forbid", "job_id", job.ID, "scheduled_for", scheduledFor)
			run := models.CronRun{
				JobID:        job.ID,
				JobUpdatedAt: job.UpdatedAt,
				Status:       StatusSkipped,
				ScheduledFor: scheduledFor,
				Attempt:      1,
				Error:        &msg,
			}
			if err := tx.Create(&run).Error; err != nil {
				return err
			}
			return tx.Model(&models.CronJob{}).Where("id = ?", job.ID).Updates(updates).Error
		}

		run := models.CronRun{
			JobID:        job.ID,
			JobUpdatedAt: job.UpdatedAt,
			Status:       StatusQueued,
			ScheduledFor: scheduledFor,
			Attempt:      1,
		}
		if err := tx.Create(&run).Error; err != nil {
			return err
		}
		queued = true
		return tx.Model(&models.CronJob{}).Where("id = ?", job.ID).Updates(updates).Error
	})
	if err != nil {
		return false, err
	}
	if queued {
		s.wakeWorkers()
	}
	return queued, nil
}

func (s *Scheduler) workerLoop(ctx context.Context, workerID int) {
	idleWait := s.cfg.Tick
	if idleWait <= 0 {
		idleWait = 60 * time.Second
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.wakeCh:
		case <-time.After(idleWait):
		}

		for {
			run, ok, err := s.claimNextQueuedRun(ctx)
			if err != nil {
				s.log.Warn("scheduler_claim_error", "worker", workerID, "error", err.Error())
				break
			}
			if !ok {
				break
			}

			if err := s.executeRun(ctx, workerID, *run); err != nil {
				s.log.Warn("scheduler_run_error", "worker", workerID, "run_id", run.ID, "job_id", run.JobID, "error", err.Error())
			}
		}
	}
}

func (s *Scheduler) claimNextQueuedRun(ctx context.Context) (*models.CronRun, bool, error) {
	var r models.CronRun
	res := s.db.WithContext(ctx).
		Where("status = ?", StatusQueued).
		Order("scheduled_for asc").
		Limit(1).
		Find(&r)
	if res.Error != nil {
		return nil, false, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, false, nil
	}
	now := time.Now().UTC().Unix()
	res2 := s.db.WithContext(ctx).
		Model(&models.CronRun{}).
		Where("id = ? AND status = ?", r.ID, StatusQueued).
		Updates(map[string]any{"status": StatusRunning, "started_at": now})
	if res2.Error != nil {
		return nil, false, res2.Error
	}
	if res2.RowsAffected == 0 {
		return nil, false, nil
	}
	r.Status = StatusRunning
	r.StartedAt = &now
	return &r, true, nil
}

func (s *Scheduler) executeRun(ctx context.Context, workerID int, run models.CronRun) error {
	var job models.CronJob
	if err := s.db.WithContext(ctx).Where("id = ?", run.JobID).First(&job).Error; err != nil {
		msg := truncateString(err.Error(), s.cfg.MaxErrorChars)
		return s.finishRun(run.ID, StatusFailed, &msg, nil)
	}

	timeout := defaultTimeout
	if job.TimeoutSeconds != nil && *job.TimeoutSeconds > 0 {
		timeout = time.Duration(*job.TimeoutSeconds) * time.Second
	}

	model := s.defaultModel
	if job.Model != nil && strings.TrimSpace(*job.Model) != "" {
		model = strings.TrimSpace(*job.Model)
	}

	scheduledFor := time.Unix(run.ScheduledFor, 0).UTC().Format(time.RFC3339)
	meta := map[string]any{
		"trigger":           "cron",
		"cron_job_id":       run.JobID,
		"cron_run_id":       run.ID,
		"scheduled_for_utc": scheduledFor,
	}
	if job.NotifyTelegramChatID != nil && *job.NotifyTelegramChatID != 0 {
		meta["telegram_chat_id"] = *job.NotifyTelegramChatID
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	s.log.Info("scheduler_run_start", "worker", workerID, "run_id", run.ID, "job_id", run.JobID, "scheduled_for", run.ScheduledFor)
	summary, runErr := s.runner(runCtx, job.Task, model, meta)

	status := StatusFailed
	var errStr *string
	if runErr == nil {
		status = StatusSuccess
	} else if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		status = StatusTimedOut
		msg := fmt.Sprintf("timeout: run exceeded %s deadline", timeout.String())
		errStr = &msg
	} else if errors.Is(runCtx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		status = StatusCanceled
		msg := "canceled"
		errStr = &msg
	} else {
		msg := truncateString(runErr.Error(), s.cfg.MaxErrorChars)
		errStr = &msg
	}

	if summary != nil {
		s := truncateString(*summary, s.cfg.MaxSummaryChars)
		summary = &s
	}

	if errStr != nil {
		msg := truncateString(*errStr, s.cfg.MaxErrorChars)
		errStr = &msg
	}

	if err := s.finishRun(run.ID, status, errStr, summary); err != nil {
		return err
	}

	if s.cfg.OnRunFinished != nil {
		notifyCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.cfg.OnRunFinished(notifyCtx, job, run, status, errStr, summary); err != nil {
			s.log.Warn("scheduler_notify_error", "worker", workerID, "run_id", run.ID, "job_id", run.JobID, "error", err.Error())
		}
	}
	return nil
}

func (s *Scheduler) finishRun(runID string, status string, errStr *string, summary *string) error {
	now := time.Now().UTC().Unix()
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.db.WithContext(dbCtx).
		Model(&models.CronRun{}).
		Where("id = ?", runID).
		Updates(map[string]any{
			"status":         status,
			"finished_at":    now,
			"error":          errStr,
			"result_summary": summary,
		}).Error
}

func truncateString(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}
