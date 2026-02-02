package scheduler

import (
	"testing"
	"time"

	"github.com/quailyquaily/mistermorph/db/models"
)

func TestNextRunAt_Interval(t *testing.T) {
	interval := int64(60)
	job := models.CronJob{
		IntervalSeconds: &interval,
	}
	after := time.Date(2026, 2, 3, 9, 0, 0, 0, time.UTC).Unix()
	next, err := nextRunAt(job, after)
	if err != nil {
		t.Fatalf("nextRunAt: %v", err)
	}
	if want := after + 60; next != want {
		t.Fatalf("want %d, got %d", want, next)
	}
}
