package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/quailyquaily/mistermorph/db"
	"github.com/quailyquaily/mistermorph/db/models"
	"gorm.io/gorm"
)

type ScheduleJobTool struct {
	DSN string

	once    sync.Once
	openErr error
	gdb     *gorm.DB
}

func NewScheduleJobTool(dsn string) *ScheduleJobTool {
	return &ScheduleJobTool{DSN: strings.TrimSpace(dsn)}
}

func (t *ScheduleJobTool) Name() string { return "schedule_job" }
func (t *ScheduleJobTool) Description() string {
	return "Create or update a persistent scheduled job (stored in SQLite cron_jobs). This is run-metadata aware scheduling for the resident scheduler."
}

func (t *ScheduleJobTool) ParameterSchema() string {
	return `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "name": { "type": "string", "description": "Job name (unique)." },
    "task": { "type": "string", "description": "Agent task string to execute." },
    "enabled": { "type": "boolean", "description": "Enable/disable job (default true)." },
    "schedule": { "type": "string", "description": "Cron expression (5-field, UTC). Example: \"0 9 * * *\"." },
    "interval_seconds": { "type": "integer", "description": "Fixed interval schedule in seconds (alternative to schedule). Note: repeats forever unless run_once=true." },
    "run_once": { "type": "boolean", "description": "If true, disable the job after its next scheduled enqueue (one-shot execution)." },
    "notify_telegram_chat_id": { "type": "integer", "description": "Optional Telegram chat_id to notify with the run result (best-effort; requires runtime support)." },
    "model": { "type": "string", "description": "Optional model override." },
    "timeout_seconds": { "type": "integer", "description": "Optional per-run timeout override (seconds)." },
    "overlap_policy": { "type": "string", "description": "Overlap policy: forbid|queue|replace (default forbid)." }
  },
  "required": ["name", "task"]
}`
}

func (t *ScheduleJobTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	gdb, err := t.db(ctx)
	if err != nil {
		return "", err
	}

	name := strings.TrimSpace(getString(params, "name"))
	if name == "" {
		return "", fmt.Errorf("missing name")
	}
	task := strings.TrimSpace(getString(params, "task"))
	if task == "" {
		return "", fmt.Errorf("missing task")
	}

	schedule := strings.TrimSpace(getString(params, "schedule"))
	intervalSeconds := getInt64(params, "interval_seconds")
	if schedule == "" && intervalSeconds <= 0 {
		return "", fmt.Errorf("missing schedule or interval_seconds")
	}
	if schedule != "" && intervalSeconds > 0 {
		return "", fmt.Errorf("provide only one of schedule or interval_seconds")
	}

	enabled := true
	if v, ok := params["enabled"]; ok {
		if b, ok := v.(bool); ok {
			enabled = b
		}
	}

	runOnce := false
	if v, ok := params["run_once"]; ok {
		if b, ok := v.(bool); ok {
			runOnce = b
		}
	}

	notifyTelegramChatID := getInt64(params, "notify_telegram_chat_id")

	model := strings.TrimSpace(getString(params, "model"))
	timeoutSeconds := getInt64(params, "timeout_seconds")
	overlapPolicy := strings.TrimSpace(getString(params, "overlap_policy"))
	if overlapPolicy == "" {
		overlapPolicy = "forbid"
	}

	var job models.CronJob
	err = gdb.WithContext(ctx).Where("name = ?", name).First(&job).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	set := func(j *models.CronJob) {
		j.Name = name
		j.Task = task
		j.Enabled = enabled
		j.RunOnce = runOnce
		j.OverlapPolicy = overlapPolicy

		if schedule != "" {
			j.Schedule = &schedule
			j.IntervalSeconds = nil
		} else {
			j.Schedule = nil
			j.IntervalSeconds = &intervalSeconds
		}

		if model != "" {
			j.Model = &model
		} else {
			j.Model = nil
		}
		if timeoutSeconds > 0 {
			j.TimeoutSeconds = &timeoutSeconds
		} else {
			j.TimeoutSeconds = nil
		}

		if notifyTelegramChatID != 0 {
			j.NotifyTelegramChatID = &notifyTelegramChatID
		} else {
			j.NotifyTelegramChatID = nil
		}
	}

	isCreate := errors.Is(err, gorm.ErrRecordNotFound)
	if isCreate {
		set(&job)
		// Let scheduler compute NextRunAt; it will reconcile NULL next_run_at on its next tick.
		if err := gdb.WithContext(ctx).Create(&job).Error; err != nil {
			return "", err
		}
	} else {
		set(&job)
		// Force scheduler to recompute next_run_at after updates (e.g. schedule changes).
		job.NextRunAt = nil
		if err := gdb.WithContext(ctx).Save(&job).Error; err != nil {
			return "", err
		}
	}

	out := map[string]any{
		"ok":       true,
		"job_id":   job.ID,
		"enabled":  job.Enabled,
		"run_once": job.RunOnce,
		"notify_telegram_chat_id": func() any {
			if job.NotifyTelegramChatID == nil {
				return nil
			}
			return *job.NotifyTelegramChatID
		}(),
		"updated_at_utc": func() string {
			if job.UpdatedAt == 0 {
				return ""
			}
			return time.Unix(job.UpdatedAt, 0).UTC().Format(time.RFC3339)
		}(),
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

func (t *ScheduleJobTool) db(ctx context.Context) (*gorm.DB, error) {
	t.once.Do(func() {
		cfg := db.DefaultConfig()
		cfg.DSN = t.DSN
		cfg.AutoMigrate = true

		gdb, err := db.Open(ctx, cfg)
		if err != nil {
			t.openErr = err
			return
		}
		if err := db.AutoMigrate(gdb); err != nil {
			t.openErr = err
			return
		}
		t.gdb = gdb
	})
	return t.gdb, t.openErr
}

func getString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	default:
		return ""
	}
}

func getInt64(m map[string]any, key string) int64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case int64:
		return x
	case float64:
		// JSON numbers decode as float64.
		return int64(x)
	default:
		return 0
	}
}
