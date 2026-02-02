package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/quailyquaily/mistermorph/db/models"
)

type ListJobsTool struct {
	db *ScheduleJobTool
}

func NewListJobsTool(dsn string) *ListJobsTool {
	return &ListJobsTool{db: NewScheduleJobTool(dsn)}
}

func (t *ListJobsTool) Name() string { return "list_jobs" }
func (t *ListJobsTool) Description() string {
	return "List recent scheduled cron jobs (UTC) so the agent can choose one to modify/cancel. No matching is performed."
}

func (t *ListJobsTool) ParameterSchema() string {
	return `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "enabled": { "type": "boolean", "description": "Filter by enabled/disabled." },
    "order_by": { "type": "string", "description": "updated_at_desc|last_run_at_desc|next_run_at_asc (default updated_at_desc)." },
    "limit": { "type": "integer", "description": "Max results (default 20, max 200)." }
  }
}`
}

func (t *ListJobsTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	gdb, err := t.db.db(ctx)
	if err != nil {
		return "", err
	}

	limit := int(getInt64(params, "limit"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	orderBy := strings.ToLower(strings.TrimSpace(getString(params, "order_by")))
	if orderBy == "" {
		orderBy = "updated_at_desc"
	}

	var enabledFilter *bool
	if v, ok := params["enabled"]; ok {
		if b, ok := v.(bool); ok {
			enabledFilter = &b
		}
	}

	query := gdb.WithContext(ctx).Model(&models.CronJob{})
	if enabledFilter != nil {
		query = query.Where("enabled = ?", *enabledFilter)
	}
	switch orderBy {
	case "updated_at_desc":
		query = query.Order("updated_at desc")
	case "last_run_at_desc":
		query = query.Order("last_run_at desc nulls last").Order("updated_at desc")
	case "next_run_at_asc":
		query = query.Order("next_run_at asc nulls last").Order("updated_at desc")
	default:
		return "", fmt.Errorf("invalid order_by %q", orderBy)
	}

	var jobs []models.CronJob
	if err := query.Limit(limit).Find(&jobs).Error; err != nil {
		return "", err
	}

	out := make([]map[string]any, 0, len(jobs))
	for _, j := range jobs {
		item := map[string]any{
			"id":       j.ID,
			"name":     j.Name,
			"enabled":  j.Enabled,
			"run_once": j.RunOnce,
		}
		if j.Schedule != nil {
			item["schedule"] = *j.Schedule
		}
		if j.IntervalSeconds != nil {
			item["interval_seconds"] = *j.IntervalSeconds
		}
		if j.Model != nil {
			item["model"] = *j.Model
		}
		if j.TimeoutSeconds != nil {
			item["timeout_seconds"] = *j.TimeoutSeconds
		}
		if strings.TrimSpace(j.OverlapPolicy) != "" {
			item["overlap_policy"] = j.OverlapPolicy
		}
		if j.LastRunAt != nil {
			item["last_run_at_utc"] = time.Unix(*j.LastRunAt, 0).UTC().Format(time.RFC3339)
		}
		if j.NextRunAt != nil {
			item["next_run_at_utc"] = time.Unix(*j.NextRunAt, 0).UTC().Format(time.RFC3339)
		}
		if j.NotifyTelegramChatID != nil {
			item["notify_telegram_chat_id"] = *j.NotifyTelegramChatID
		}
		item["updated_at_utc"] = time.Unix(j.UpdatedAt, 0).UTC().Format(time.RFC3339)
		item["task_preview"] = truncate(j.Task, 200)
		out = append(out, item)
	}

	b, _ := json.Marshal(map[string]any{"ok": true, "count": len(out), "jobs": out})
	return string(b), nil
}
