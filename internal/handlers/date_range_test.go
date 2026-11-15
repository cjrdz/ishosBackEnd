package handlers

import (
	"testing"
	"time"
)

func TestParseDateRange_UsesStoreTimeZone(t *testing.T) {
	loc, err := time.LoadLocation("America/El_Salvador") // UTC-6, no DST
	if err != nil {
		t.Fatal(err)
	}

	start, end, err := parseDateRange("2026-10-25", "2026-10-25", loc)
	if err != nil {
		t.Fatal(err)
	}

	// The store's day 2026-10-25 runs 06:00 UTC on the 25th to 05:59:59 UTC on the 26th.
	if want := time.Date(2026, 10, 25, 6, 0, 0, 0, time.UTC); !start.Equal(want) {
		t.Fatalf("start = %s, want %s", start.UTC(), want)
	}
	if want := time.Date(2026, 10, 26, 5, 59, 59, 999999999, time.UTC); !end.Equal(want) {
		t.Fatalf("end = %s, want %s", end.UTC(), want)
	}
}

func TestCapDateRange_CountsCalendarDays(t *testing.T) {
	loc, _ := time.LoadLocation("America/El_Salvador")

	if _, _, err := capDateRange("2026-10-01", "2026-10-07", 7, loc); err != nil {
		t.Fatalf("7 inclusive days should be allowed: %v", err)
	}
	if _, _, err := capDateRange("2026-10-01", "2026-10-08", 7, loc); err == nil {
		t.Fatal("8 inclusive days should exceed a 7-day cap")
	}
	if _, _, err := capDateRange("2026-10-08", "2026-10-01", 7, loc); err == nil {
		t.Fatal("end before start should be rejected")
	}
}
