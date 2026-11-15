package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

func elSalvador(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/El_Salvador")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// Example schedule: weekdays 2–9 pm, Saturday split shift, closed Sunday.
var sampleHours = WeeklyHours{
	"mon": {{"14:00", "21:00"}}, "tue": {{"14:00", "21:00"}}, "wed": {{"14:00", "21:00"}},
	"thu": {{"14:00", "21:00"}}, "fri": {{"14:00", "21:00"}},
	"sat": {{"10:00", "13:00"}, {"15:00", "22:00"}},
}

func TestComputeOpenStatus(t *testing.T) {
	loc := elSalvador(t)
	at := func(day, hour, minute int) time.Time { // October 2026: the 26th is a Monday
		return time.Date(2026, 10, day, hour, minute, 0, 0, loc)
	}

	cases := []struct {
		name       string
		now        time.Time
		open       bool
		closesAt   time.Time
		opensAt    time.Time
		wantClosed string
	}{
		{name: "open mid-shift", now: at(26, 16, 30), open: true, closesAt: at(26, 21, 0)},
		{name: "before opening today", now: at(26, 9, 0), opensAt: at(26, 14, 0), wantClosed: "Estamos cerrados. Abrimos hoy a las 2:00 pm."},
		{name: "exactly at closing is closed", now: at(26, 21, 0), opensAt: at(27, 14, 0), wantClosed: "Estamos cerrados. Abrimos mañana a las 2:00 pm."},
		{name: "between split shifts", now: at(31, 13, 30), opensAt: at(31, 15, 0)},
		{name: "after Saturday close skips closed Sunday", now: at(31, 23, 0), opensAt: at(26+7, 14, 0), wantClosed: "Estamos cerrados. Abrimos el lunes a las 2:00 pm."},
		{name: "UTC instant is judged in store time", now: time.Date(2026, 10, 26, 20, 30, 0, 0, time.UTC), open: true, closesAt: at(26, 21, 0)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeOpenStatus(sampleHours, tc.now, loc)
			if !got.Configured || got.IsOpen != tc.open {
				t.Fatalf("configured=%v open=%v, want configured open=%v", got.Configured, got.IsOpen, tc.open)
			}
			if tc.open && !got.ClosesAt.Equal(tc.closesAt) {
				t.Errorf("closes_at = %v, want %v", got.ClosesAt, tc.closesAt)
			}
			if !tc.open && !got.OpensAt.Equal(tc.opensAt) {
				t.Errorf("opens_at = %v, want %v", got.OpensAt, tc.opensAt)
			}
			if tc.wantClosed != "" {
				if msg := ClosedMessage(got, tc.now, loc); msg != tc.wantClosed {
					t.Errorf("message = %q, want %q", msg, tc.wantClosed)
				}
			}
		})
	}
}

func TestComputeOpenStatus_NotConfiguredIsAlwaysOpen(t *testing.T) {
	got := ComputeOpenStatus(nil, time.Now(), elSalvador(t))
	if got.Configured || !got.IsOpen || len(got.Weekly) != 7 {
		t.Fatalf("got %+v, want unconfigured, open, all 7 days listed", got)
	}
	if raw, _ := json.Marshal(got); strings.Contains(string(raw), "null") {
		t.Errorf("closed days must encode as [], got %s", raw)
	}
}

func TestValidateWeeklyHours(t *testing.T) {
	if err := ValidateWeeklyHours(sampleHours); err != nil {
		t.Fatalf("valid schedule rejected: %v", err)
	}
	bad := map[string]WeeklyHours{
		"unknown day":     {"funday": {{"10:00", "12:00"}}},
		"bad format":      {"mon": {{"9:00", "12:00"}}},
		"close before":    {"mon": {{"14:00", "13:00"}}},
		"zero length":     {"mon": {{"14:00", "14:00"}}},
		"overlap":         {"mon": {{"10:00", "14:00"}, {"13:00", "18:00"}}},
		"too many ranges": {"mon": {{"08:00", "09:00"}, {"10:00", "11:00"}, {"12:00", "13:00"}, {"14:00", "15:00"}}},
	}
	for name, h := range bad {
		if err := ValidateWeeklyHours(h); !errors.Is(err, apperror.ErrValidation) {
			t.Errorf("%s: expected validation error, got %v", name, err)
		}
	}
}

func TestStoreHoursRoundTripThroughSettings(t *testing.T) {
	stored := map[string]map[string]any{}
	repo := &adminSettingRepoStub{
		getByKey: func(_ context.Context, key string) (dto.AdminSetting, error) {
			if v, ok := stored[key]; ok {
				return dto.AdminSetting{Value: v}, nil
			}
			return dto.AdminSetting{}, &apperror.NotFoundError{Resource: "admin_setting"}
		},
		upsertByKey: func(_ context.Context, key string, value map[string]any, _ string) (dto.AdminSetting, error) {
			stored[key] = value
			return dto.AdminSetting{Value: value}, nil
		},
	}
	svc := NewAdminSettingService(repo).WithLocation(elSalvador(t))

	if got := svc.StoreOpenStatus(context.Background(), time.Now()); got.Configured || !got.IsOpen {
		t.Fatalf("no schedule saved: want unconfigured and open, got %+v", got)
	}
	if _, err := svc.UpdateStoreHours(context.Background(), sampleHours, "admin"); err != nil {
		t.Fatal(err)
	}
	read, err := svc.GetStoreHours(context.Background())
	if err != nil || len(read["sat"]) != 2 || read["sat"][0].Open != "10:00" {
		t.Fatalf("read back %v (%v), want Saturday's split shift", read, err)
	}
	monday4pm := time.Date(2026, 10, 26, 16, 0, 0, 0, elSalvador(t))
	if !svc.StoreOpenStatus(context.Background(), monday4pm).IsOpen {
		t.Fatal("expected open on Monday at 4 pm")
	}
	if _, err := svc.UpdateStoreHours(context.Background(), WeeklyHours{"mon": {{"21:00", "14:00"}}}, "admin"); !errors.Is(err, apperror.ErrValidation) {
		t.Fatalf("invalid schedule should be rejected, got %v", err)
	}
	if read2, _ := svc.GetStoreHours(context.Background()); len(read2["sat"]) != 2 {
		t.Fatal("a rejected update must not overwrite the saved schedule")
	}
}
