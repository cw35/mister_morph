package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/quailyquaily/mistermorph/db/models"
)

type SearchJobsTool struct {
	db *ScheduleJobTool
}

func NewSearchJobsTool(dsn string) *SearchJobsTool {
	return &SearchJobsTool{db: NewScheduleJobTool(dsn)}
}

func (t *SearchJobsTool) Name() string { return "search_jobs" }
func (t *SearchJobsTool) Description() string {
	return "List candidate scheduled cron jobs (UTC) for the agent to choose from. This tool is retrieval-only; selection is done by the LLM. Supports optional simple substring filters and time filters."
}

func (t *SearchJobsTool) ParameterSchema() string {
	return `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "q": { "type": "string", "description": "Search string. Matches name/task (substring). Can include space-separated keywords." },
    "enabled": { "type": "boolean", "description": "Filter by enabled/disabled." },
    "schedule": { "type": "string", "description": "Exact cron expression filter (5-field, UTC)." },
    "interval_seconds": { "type": "integer", "description": "Exact interval filter in seconds." },
    "last_run_from_utc": { "type": "string", "description": "RFC3339 timestamp (UTC) lower bound for last_run_at." },
    "last_run_to_utc": { "type": "string", "description": "RFC3339 timestamp (UTC) upper bound for last_run_at." },
    "next_run_from_utc": { "type": "string", "description": "RFC3339 timestamp (UTC) lower bound for next_run_at." },
    "next_run_to_utc": { "type": "string", "description": "RFC3339 timestamp (UTC) upper bound for next_run_at." },
    "order_by": { "type": "string", "description": "Sort order: updated_at_desc|last_run_at_desc|next_run_at_asc (default updated_at_desc)." },
    "limit": { "type": "integer", "description": "Max results (default 10, max 50)." }
  }
}`
}

func (t *SearchJobsTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	gdb, err := t.db.db(ctx)
	if err != nil {
		return "", err
	}

	limit := int(getInt64(params, "limit"))
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	q := strings.TrimSpace(getString(params, "q"))
	schedule := strings.TrimSpace(getString(params, "schedule"))
	interval := getInt64(params, "interval_seconds")
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

	lastFrom, err := parseOptionalRFC3339UTC(getString(params, "last_run_from_utc"))
	if err != nil {
		return "", fmt.Errorf("last_run_from_utc: %w", err)
	}
	lastTo, err := parseOptionalRFC3339UTC(getString(params, "last_run_to_utc"))
	if err != nil {
		return "", fmt.Errorf("last_run_to_utc: %w", err)
	}
	nextFrom, err := parseOptionalRFC3339UTC(getString(params, "next_run_from_utc"))
	if err != nil {
		return "", fmt.Errorf("next_run_from_utc: %w", err)
	}
	nextTo, err := parseOptionalRFC3339UTC(getString(params, "next_run_to_utc"))
	if err != nil {
		return "", fmt.Errorf("next_run_to_utc: %w", err)
	}

	query := gdb.WithContext(ctx).Model(&models.CronJob{})
	if enabledFilter != nil {
		query = query.Where("enabled = ?", *enabledFilter)
	}
	if schedule != "" {
		query = query.Where("schedule = ?", schedule)
	}
	if interval > 0 {
		query = query.Where("interval_seconds = ?", interval)
	}

	if lastFrom != nil {
		query = query.Where("last_run_at IS NOT NULL AND last_run_at >= ?", lastFrom.Unix())
	}
	if lastTo != nil {
		query = query.Where("last_run_at IS NOT NULL AND last_run_at <= ?", lastTo.Unix())
	}
	if nextFrom != nil {
		query = query.Where("next_run_at IS NOT NULL AND next_run_at >= ?", nextFrom.Unix())
	}
	if nextTo != nil {
		query = query.Where("next_run_at IS NOT NULL AND next_run_at <= ?", nextTo.Unix())
	}

	if q != "" {
		terms := strings.Fields(q)
		if len(terms) == 0 {
			terms = []string{q}
		}
		for _, term := range terms {
			term = strings.TrimSpace(term)
			if term == "" {
				continue
			}
			like := "%" + term + "%"
			query = query.Where("(name LIKE ? OR task LIKE ?)", like, like)
		}
	}

	var jobs []models.CronJob
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

	b, _ := json.Marshal(map[string]any{
		"ok":    true,
		"count": len(out),
		"jobs":  out,
	})
	return string(b), nil
}

func parseOptionalRFC3339UTC(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	tt := t.UTC()
	return &tt, nil
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}
