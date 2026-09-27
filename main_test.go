package main

import (
	"testing"
	"time"
)

func TestCalculateWeek_PopulatesDayDates(t *testing.T) {
	setupTestDB(t)

	// 2026-09-26 is a Saturday; the week containing it starts Sunday 2026-09-20.
	aTime := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	week := calculateWeek(&aTime).Data

	wantDates := []string{
		"2026-09-20", "2026-09-21", "2026-09-22", "2026-09-23",
		"2026-09-24", "2026-09-25", "2026-09-26",
	}
	for i, day := range week.Days {
		if day.Date != wantDates[i] {
			t.Errorf("day %d: got Date %q, want %q", i, day.Date, wantDates[i])
		}
	}
}

func TestCalculateWeek_PopulatesCompletedFromDB(t *testing.T) {
	setupTestDB(t)

	if _, err := toggleCompletion("kitchen_cleaner", "2026-09-26"); err != nil {
		t.Fatalf("seeding completion failed: %v", err)
	}

	aTime := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	week := calculateWeek(&aTime).Data

	saturday := week.Days[6] // index 6 = the 7th day = Saturday, 2026-09-26
	if saturday.Date != "2026-09-26" {
		t.Fatalf("test assumption wrong: expected index 6 to be 2026-09-26, got %s", saturday.Date)
	}

	foundCompleted := false
	for _, a := range saturday.Assignments {
		if a.Chore.ID == "kitchen_cleaner" {
			if !a.Completed {
				t.Errorf("expected kitchen_cleaner assignment on 2026-09-26 to be Completed=true")
			}
			foundCompleted = true
		}
	}
	if !foundCompleted {
		t.Fatalf("expected a kitchen_cleaner assignment on 2026-09-26")
	}
}
