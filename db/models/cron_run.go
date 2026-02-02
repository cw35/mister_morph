package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CronRun struct {
	ID string `gorm:"primaryKey;type:text"`

	JobID string  `gorm:"type:text;not null;index"`
	Job   CronJob `gorm:"foreignKey:JobID;references:ID;constraint:OnDelete:CASCADE"`

	// Snapshot job version at enqueue time (UTC unix seconds from cron_jobs.updated_at).
	JobUpdatedAt int64 `gorm:"not null;default:0"`

	// queued|running|succeeded|failed|canceled|timed_out|skipped
	Status string `gorm:"type:text;not null;index"`

	// UTC unix seconds
	ScheduledFor int64  `gorm:"not null;index"`
	StartedAt    *int64 `gorm:""`
	FinishedAt   *int64 `gorm:""`

	Attempt int `gorm:"not null;default:1"`

	Error         *string `gorm:"type:text"`
	ResultSummary *string `gorm:"type:text"`

	CreatedAt int64 `gorm:"autoCreateTime"`
	UpdatedAt int64 `gorm:"autoUpdateTime"`
}

func (r *CronRun) BeforeCreate(_ *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	return nil
}
