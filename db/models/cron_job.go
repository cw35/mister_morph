package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CronJob struct {
	ID string `gorm:"primaryKey;type:text"`

	Name    string `gorm:"type:text;not null;uniqueIndex"`
	Enabled bool   `gorm:"not null;default:1"`

	// Exactly one of Schedule (cron expr) or IntervalSeconds should be set.
	Schedule        *string `gorm:"type:text"`
	IntervalSeconds *int64  `gorm:""`

	// Agent input
	Task string `gorm:"type:text;not null"`

	// If true, disable the job after its next scheduled enqueue (one-shot execution).
	RunOnce bool `gorm:"not null;default:0"`

	// Optional notification target (best-effort; depends on runtime wiring).
	NotifyTelegramChatID *int64 `gorm:"index"`

	// Optional overrides (best-effort; depends on runtime wiring).
	Provider *string `gorm:"type:text"`
	Model    *string `gorm:"type:text"`

	// Per-run timeout override (seconds). If nil/<=0, use scheduler default (hardcoded 10m).
	TimeoutSeconds *int64 `gorm:""`

	// forbid|queue|replace (queue/replace may be unsupported initially).
	OverlapPolicy string `gorm:"type:text;not null;default:'forbid'"`

	// Derived schedule state (UTC unix seconds).
	LastRunAt *int64 `gorm:""`
	NextRunAt *int64 `gorm:"index"`

	CreatedAt int64 `gorm:"autoCreateTime"`
	UpdatedAt int64 `gorm:"autoUpdateTime"`
}

func (j *CronJob) BeforeCreate(_ *gorm.DB) error {
	if j.ID == "" {
		j.ID = uuid.NewString()
	}
	return nil
}
