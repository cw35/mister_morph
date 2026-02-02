package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/quailyquaily/mistermorph/db/models"
	"gorm.io/gorm"
)

type UnscheduleJobTool struct {
	db *ScheduleJobTool
}

func NewUnscheduleJobTool(dsn string) *UnscheduleJobTool {
	return &UnscheduleJobTool{db: NewScheduleJobTool(dsn)}
}

func (t *UnscheduleJobTool) Name() string { return "unschedule_job" }
func (t *UnscheduleJobTool) Description() string {
	return "Disable or delete a scheduled job by id or exact name. Prefer disabling (enabled=false) to preserve run history."
}

func (t *UnscheduleJobTool) ParameterSchema() string {
	return `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "job_id": { "type": "string", "description": "Job id (preferred)." },
    "name": { "type": "string", "description": "Exact job name (must match exactly)." },
    "mode": { "type": "string", "description": "disable|delete (default disable)." }
  }
}`
}

func (t *UnscheduleJobTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	gdb, err := t.db.db(ctx)
	if err != nil {
		return "", err
	}

	jobID := strings.TrimSpace(getString(params, "job_id"))
	name := strings.TrimSpace(getString(params, "name"))
	if jobID == "" && name == "" {
		return "", fmt.Errorf("missing job_id or name")
	}

	mode := strings.ToLower(strings.TrimSpace(getString(params, "mode")))
	if mode == "" {
		mode = "disable"
	}
	if mode != "disable" && mode != "delete" {
		return "", fmt.Errorf("invalid mode %q (use disable|delete)", mode)
	}

	var job models.CronJob
	q := gdb.WithContext(ctx)
	switch {
	case jobID != "":
		err = q.Where("id = ?", jobID).First(&job).Error
	default:
		err = q.Where("name = ?", name).First(&job).Error
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", fmt.Errorf("job not found")
		}
		return "", err
	}

	if mode == "delete" {
		if err := q.Delete(&models.CronJob{}, "id = ?", job.ID).Error; err != nil {
			return "", err
		}
	} else {
		if err := q.Model(&models.CronJob{}).Where("id = ?", job.ID).Updates(map[string]any{
			"enabled":     false,
			"next_run_at": nil,
		}).Error; err != nil {
			return "", err
		}
	}

	out := map[string]any{
		"ok":     true,
		"job_id": job.ID,
		"name":   job.Name,
		"mode":   mode,
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}
