package entitlement

import (
	"testing"
	"time"
)

func TestPeriodWindowUsesAssignmentAnchor(t *testing.T) {
	anchor := time.Date(2026, time.June, 20, 15, 30, 0, 0, time.UTC)
	now := time.Date(2026, time.July, 21, 10, 0, 0, 0, time.UTC)

	from, to := periodWindow(now, anchor, PeriodMonth)

	wantFrom := time.Date(2026, time.July, 20, 15, 30, 0, 0, time.UTC)
	wantTo := time.Date(2026, time.August, 20, 15, 30, 0, 0, time.UTC)
	if !from.Equal(wantFrom) || !to.Equal(wantTo) {
		t.Fatalf("periodWindow() = %s - %s, want %s - %s", from, to, wantFrom, wantTo)
	}
}

func TestPeriodWindowBeforeAssignmentStartsAtAnchor(t *testing.T) {
	anchor := time.Date(2026, time.June, 20, 15, 30, 0, 0, time.UTC)
	now := time.Date(2026, time.June, 19, 15, 30, 0, 0, time.UTC)

	from, to := periodWindow(now, anchor, PeriodWeek)

	wantTo := time.Date(2026, time.June, 27, 15, 30, 0, 0, time.UTC)
	if !from.Equal(anchor) || !to.Equal(wantTo) {
		t.Fatalf("periodWindow() = %s - %s, want %s - %s", from, to, anchor, wantTo)
	}
}
