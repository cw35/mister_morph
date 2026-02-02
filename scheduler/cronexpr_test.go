package scheduler

import (
	"testing"
	"time"
)

func TestCronExpr_Next_AllAny(t *testing.T) {
	e, err := parseCronExpr("* * * * *")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	after := time.Date(2026, 2, 3, 9, 0, 30, 0, time.UTC)
	next, err := e.next(after)
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if want := time.Date(2026, 2, 3, 9, 1, 0, 0, time.UTC); !next.Equal(want) {
		t.Fatalf("want %s, got %s", want.Format(time.RFC3339), next.Format(time.RFC3339))
	}
}

func TestCronExpr_Next_DailyAt0900(t *testing.T) {
	e, err := parseCronExpr("0 9 * * *")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	after := time.Date(2026, 2, 3, 8, 59, 59, 0, time.UTC)
	next, err := e.next(after)
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if want := time.Date(2026, 2, 3, 9, 0, 0, 0, time.UTC); !next.Equal(want) {
		t.Fatalf("want %s, got %s", want.Format(time.RFC3339), next.Format(time.RFC3339))
	}
}

func TestCronExpr_Invalid(t *testing.T) {
	_, err := parseCronExpr("0 0 * *")
	if err == nil {
		t.Fatalf("expected error")
	}
}
