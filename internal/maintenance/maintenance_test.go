package maintenance_test

import (
	"testing"
	"time"
	"uptime-mon/internal/config"
	"uptime-mon/internal/maintenance"
)

func TestOneOffMaintenanceEvaluation(t *testing.T) {
	cfg := &config.RuntimeConfig{
		Global: config.DefaultGlobalConfig(),
		OneOffMaintenance: map[string]config.OneOffMaintenance{
			"switch-reboot": {
				ID:            "switch-reboot",
				StartDate:     "2026-10-21",
				StartTime:     "14:00",
				EndDate:       "2026-10-21",
				EndTime:       "16:00",
				Message:       "Switch reload",
				RemindMe:      2,
				AffectedZones: []string{"networking"},
				AffectedHosts: []string{"all"},
			},
		},
		ScheduledMaintenance: make(map[string]config.ScheduledMaintenance),
	}

	mgr := maintenance.NewManager(cfg, nil)

	// In Berlin (UTC+2 in October) or UTC
	loc, _ := time.LoadLocation("Europe/Berlin")

	// 1. Before maintenance window
	before := time.Date(2026, 10, 21, 13, 0, 0, 0, loc)
	if mgr.IsEndpointInMaintenance("core-switch", "networking", before) {
		t.Errorf("expected false before maintenance window starts")
	}

	// 2. Inside maintenance window
	inside := time.Date(2026, 10, 21, 14, 30, 0, 0, loc)
	if !mgr.IsEndpointInMaintenance("core-switch", "networking", inside) {
		t.Errorf("expected true inside maintenance window")
	}

	// 3. Different zone not affected
	if mgr.IsEndpointInMaintenance("web-app", "webserver", inside) {
		t.Errorf("expected false for un-affected zone")
	}

	// 4. After maintenance window
	after := time.Date(2026, 10, 21, 16, 01, 0, 0, loc)
	if mgr.IsEndpointInMaintenance("core-switch", "networking", after) {
		t.Errorf("expected false after maintenance window ended")
	}
}

func TestScheduledCronMaintenance(t *testing.T) {
	sm := &config.ScheduledMaintenance{
		ID:            "weekly-backup",
		Schedule:      "0 2 * * 0", // Every Sunday at 02:00
		Duration:      config.Duration(2 * time.Hour),
		Message:       "Weekly backup",
		AffectedZones: []string{"webserver"},
		AffectedHosts: []string{"db-master"},
	}

	loc := time.UTC

	// Sunday 2026-10-25 at 02:30 UTC -> should be active!
	sundayInside := time.Date(2026, 10, 25, 2, 30, 0, 0, loc)
	winActive, err := maintenance.EvaluateCronWindow(sm, sundayInside, loc)
	if err != nil {
		t.Fatalf("EvaluateCronWindow failed: %v", err)
	}
	if !winActive.IsActive {
		t.Errorf("expected CRON maintenance to be active on Sunday at 02:30")
	}

	// Sunday 2026-10-25 at 04:30 UTC -> finished at 04:00, so not active
	sundayAfter := time.Date(2026, 10, 25, 4, 30, 0, 0, loc)
	winAfter, err := maintenance.EvaluateCronWindow(sm, sundayAfter, loc)
	if err != nil {
		t.Fatalf("EvaluateCronWindow failed: %v", err)
	}
	if winAfter.IsActive {
		t.Errorf("expected CRON maintenance to be inactive after window ends")
	}
}
