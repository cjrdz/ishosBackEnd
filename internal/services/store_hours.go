// store_hours.go — Weekly opening hours and the open/closed status derived from them.
package services

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
)

// StoreHoursKey stores the weekly schedule: {"days": {"mon": [{"open": "14:00", "close": "21:00"}], ...}}.
const StoreHoursKey = "store.hours"

// MaxRangesPerDay allows split shifts (e.g. lunch and evening) without
// turning the schedule into a calendar.
const MaxRangesPerDay = 3

// WeekDays lists schedule keys in display order (Monday first, as in El Salvador).
var WeekDays = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

var weekdayKey = map[time.Weekday]string{
	time.Monday: "mon", time.Tuesday: "tue", time.Wednesday: "wed", time.Thursday: "thu",
	time.Friday: "fri", time.Saturday: "sat", time.Sunday: "sun",
}

// TimeRange is an opening window within one day, in store-local "HH:MM".
// Close must be after Open; windows past midnight are not supported.
type TimeRange struct {
	Open  string `json:"open"`
	Close string `json:"close"`
}

// WeeklyHours maps a day key (mon…sun) to its opening windows. A missing or
// empty day means closed all day.
type WeeklyHours map[string][]TimeRange

// StoreOpenStatus is the schedule-derived status shown to customers.
type StoreOpenStatus struct {
	// Configured is false when no schedule was saved: the store then behaves
	// as always open (only the manual orders switch applies).
	Configured bool        `json:"configured"`
	IsOpen     bool        `json:"is_open"`
	ClosesAt   *time.Time  `json:"closes_at,omitempty"` // end of the current window when open
	OpensAt    *time.Time  `json:"opens_at,omitempty"`  // start of the next window when closed
	Today      []TimeRange `json:"today"`
	Weekly     WeeklyHours `json:"weekly"`
}

func parseClock(value string) (int, error) {
	t, err := time.Parse("15:04", value)
	if err != nil || len(value) != 5 {
		return 0, fmt.Errorf("%q is not a HH:MM time", value)
	}
	return t.Hour()*60 + t.Minute(), nil
}

// ValidateWeeklyHours checks day keys, HH:MM format, open < close, the
// per-day limit and overlaps. It returns an *apperror.ValidationError.
func ValidateWeeklyHours(hours WeeklyHours) error {
	fields := map[string]string{}
	valid := map[string]bool{}
	for _, d := range WeekDays {
		valid[d] = true
	}
	for day, ranges := range hours {
		if !valid[day] {
			fields[day] = "unknown day (use mon…sun)"
			continue
		}
		if len(ranges) > MaxRangesPerDay {
			fields[day] = fmt.Sprintf("at most %d time ranges per day", MaxRangesPerDay)
			continue
		}
		type span struct{ open, close int }
		spans := make([]span, 0, len(ranges))
		for i, r := range ranges {
			open, errOpen := parseClock(r.Open)
			closeAt, errClose := parseClock(r.Close)
			switch {
			case errOpen != nil:
				fields[fmt.Sprintf("%s[%d].open", day, i)] = errOpen.Error()
			case errClose != nil:
				fields[fmt.Sprintf("%s[%d].close", day, i)] = errClose.Error()
			case closeAt <= open:
				fields[fmt.Sprintf("%s[%d]", day, i)] = "close must be after open"
			default:
				spans = append(spans, span{open, closeAt})
			}
		}
		sort.Slice(spans, func(a, b int) bool { return spans[a].open < spans[b].open })
		for i := 1; i < len(spans); i++ {
			if spans[i].open < spans[i-1].close {
				fields[day] = "time ranges overlap"
			}
		}
	}
	if len(fields) > 0 {
		return &apperror.ValidationError{Fields: fields, Cause: fmt.Errorf("invalid store hours")}
	}
	return nil
}

// NormalizeWeeklyHours returns a copy with every day present and ranges
// sorted by opening time, so clients can render it without special cases.
func NormalizeWeeklyHours(hours WeeklyHours) WeeklyHours {
	out := make(WeeklyHours, len(WeekDays))
	for _, d := range WeekDays {
		ranges := append([]TimeRange{}, hours[d]...) // [] not null in JSON for closed days
		sort.Slice(ranges, func(a, b int) bool { return ranges[a].Open < ranges[b].Open })
		out[d] = ranges
	}
	return out
}

func atClock(day time.Time, clock string) time.Time {
	minutes, _ := parseClock(clock)
	return time.Date(day.Year(), day.Month(), day.Day(), minutes/60, minutes%60, 0, 0, day.Location())
}

// ComputeOpenStatus derives the open/closed status at now in loc (the store's
// time zone). An empty schedule means "not configured": always open.
func ComputeOpenStatus(hours WeeklyHours, now time.Time, loc *time.Location) StoreOpenStatus {
	weekly := NormalizeWeeklyHours(hours)
	local := now.In(loc)
	status := StoreOpenStatus{Weekly: weekly, Today: weekly[weekdayKey[local.Weekday()]]}

	for _, ranges := range weekly {
		if len(ranges) > 0 {
			status.Configured = true
			break
		}
	}
	if !status.Configured {
		status.IsOpen = true
		return status
	}

	// Look at today and the next 7 days for the current or next window.
	for offset := 0; offset <= 7; offset++ {
		day := time.Date(local.Year(), local.Month(), local.Day()+offset, 0, 0, 0, 0, loc)
		for _, r := range weekly[weekdayKey[day.Weekday()]] {
			open, closeAt := atClock(day, r.Open), atClock(day, r.Close)
			if offset == 0 && !local.Before(open) && local.Before(closeAt) {
				status.IsOpen = true
				status.ClosesAt = &closeAt
				return status
			}
			if open.After(local) {
				status.OpensAt = &open
				return status
			}
		}
	}
	return status
}

var spanishWeekday = map[time.Weekday]string{
	time.Monday: "el lunes", time.Tuesday: "el martes", time.Wednesday: "el miércoles",
	time.Thursday: "el jueves", time.Friday: "el viernes", time.Saturday: "el sábado", time.Sunday: "el domingo",
}

// FormatClock renders a time like "2:00 pm" (as written in El Salvador).
func FormatClock(t time.Time) string {
	return strings.ToLower(t.Format("3:04 PM"))
}

// ClosedMessage explains to a customer when the store opens next, e.g.
// "Estamos cerrados. Abrimos mañana a las 2:00 pm."
func ClosedMessage(status StoreOpenStatus, now time.Time, loc *time.Location) string {
	if status.OpensAt == nil {
		return "Estamos cerrados por ahora."
	}
	local, opens := now.In(loc), status.OpensAt.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	days := int(time.Date(opens.Year(), opens.Month(), opens.Day(), 0, 0, 0, 0, loc).Sub(today).Hours() / 24)
	when := spanishWeekday[opens.Weekday()]
	switch days {
	case 0:
		when = "hoy"
	case 1:
		when = "mañana"
	}
	return fmt.Sprintf("Estamos cerrados. Abrimos %s a las %s.", when, FormatClock(opens))
}
