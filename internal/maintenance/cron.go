package maintenance

import (
	"fmt"
	"time"
	"uptime-mon/internal/config"

	"github.com/robfig/cron/v3"
)

// ActiveWindow represents a currently active or upcoming maintenance window
type ActiveWindow struct {
	ID            string
	Message       string
	StartTime     time.Time
	EndTime       time.Time
	IsActive      bool
	AffectedZones []string
	AffectedHosts []string
}

// EvaluateCronWindow evaluates if a scheduled CRON maintenance rule is active or upcoming
func EvaluateCronWindow(sm *config.ScheduledMaintenance, now time.Time, loc *time.Location) (*ActiveWindow, error) {
	if loc == nil {
		loc = time.UTC
	}
	nowInLoc := now.In(loc)

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(sm.Schedule)
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression %q: %w", sm.Schedule, err)
	}

	duration := sm.Duration.Duration()
	if duration <= 0 {
		duration = 1 * time.Hour
	}

	// 1. Check if we are inside a window that started recently (within duration in the past)
	// We search backwards from now
	checkLookback := nowInLoc.Add(-duration)
	// Find the first schedule run after checkLookback
	t1 := schedule.Next(checkLookback)
	if !t1.After(nowInLoc) {
		// It started at or before now; check if it ends after now
		tEnd := t1.Add(duration)
		if nowInLoc.Before(tEnd) {
			return &ActiveWindow{
				ID:            sm.ID,
				Message:       sm.Message,
				StartTime:     t1,
				EndTime:       tEnd,
				IsActive:      true,
				AffectedZones: sm.AffectedZones,
				AffectedHosts: sm.AffectedHosts,
			}, nil
		}
	}

	// 2. Otherwise calculate next upcoming window
	nextStart := schedule.Next(nowInLoc)
	nextEnd := nextStart.Add(duration)

	return &ActiveWindow{
		ID:            sm.ID,
		Message:       sm.Message,
		StartTime:     nextStart,
		EndTime:       nextEnd,
		IsActive:      false,
		AffectedZones: sm.AffectedZones,
		AffectedHosts: sm.AffectedHosts,
	}, nil
}
