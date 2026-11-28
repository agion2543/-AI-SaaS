package service

import (
	"testing"
	"time"
)

// dashboardRange must resolve the day window in Beijing time
// (Asia/Shanghai, fixed UTC+8), never in time.Local: on a UTC container
// the day boundary would shift by 8 hours and trades near midnight would
// be attributed to the wrong day.
func TestDashboardRangeUsesBeijingTime(t *testing.T) {
	start, end := dashboardRange("2026-11-28", "2026-11-28")
	if start.Location() != beijingTimeZone || end.Location() != beijingTimeZone {
		t.Fatalf("dashboardRange must return Beijing-time bounds, got %v / %v", start.Location(), end.Location())
	}
	if got, want := start.Format("2006-01-02 15:04:05 -0700"), "2026-11-28 00:00:00 +0800"; got != want {
		t.Fatalf("range start=%q, want %q", got, want)
	}
	if got, want := end.Format("2006-01-02 15:04:05 -0700"), "2026-11-28 23:59:59 +0800"; got != want {
		t.Fatalf("range end=%q, want %q", got, want)
	}

	// The default (no-arg) window is "today" in Beijing time, even when
	// the container runs in UTC.
	nowBeijing := time.Now().In(beijingTimeZone)
	start, end = dashboardRange("", "")
	if start.Year() != nowBeijing.Year() || start.Month() != nowBeijing.Month() || start.Day() != nowBeijing.Day() {
		t.Fatalf("default range start=%v, want Beijing today %v", start, nowBeijing.Format("2006-01-02"))
	}
	if end.Year() != nowBeijing.Year() || end.Month() != nowBeijing.Month() || end.Day() != nowBeijing.Day() {
		t.Fatalf("default range end=%v, want Beijing today %v", end, nowBeijing.Format("2006-01-02"))
	}
	// Reversed input is swapped, not silently kept.
	reversedStart, reversedEnd := dashboardRange("2026-11-29", "2026-11-28")
	if reversedStart.After(reversedEnd) {
		t.Fatalf("reversed range must be swapped, got %v > %v", reversedStart, reversedEnd)
	}
}
