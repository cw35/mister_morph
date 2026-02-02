package scheduler

import (
	"fmt"
	"strings"
	"time"

	"github.com/quailyquaily/mistermorph/db/models"
)

func nextRunAt(job models.CronJob, afterUnix int64) (int64, error) {
	after := time.Unix(afterUnix, 0).UTC()

	if job.Schedule != nil && strings.TrimSpace(*job.Schedule) != "" {
		expr, err := parseCronExpr(*job.Schedule)
		if err != nil {
			return 0, err
		}
		next, err := expr.next(after)
		if err != nil {
			return 0, err
		}
		return next.Unix(), nil
	}
	if job.IntervalSeconds != nil && *job.IntervalSeconds > 0 {
		return after.Add(time.Duration(*job.IntervalSeconds) * time.Second).Unix(), nil
	}
	return 0, fmt.Errorf("job has neither schedule nor interval_seconds")
}
