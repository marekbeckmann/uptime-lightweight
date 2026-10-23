package maintenance

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"uptime-mon/internal/alerting"
	"uptime-mon/internal/config"
)

// Manager orchestrates one-off and scheduled maintenance evaluation
type Manager struct {
	config     *config.RuntimeConfig
	dispatcher *alerting.Dispatcher
	location   *time.Location
	mu         sync.RWMutex
}

func NewManager(cfg *config.RuntimeConfig, dispatcher *alerting.Dispatcher) *Manager {
	loc, err := time.LoadLocation(cfg.Global.Timezone)
	if err != nil {
		loc = time.UTC
	}

	m := &Manager{
		config:     cfg,
		dispatcher: dispatcher,
		location:   loc,
	}

	// Trigger initial creation notifications
	m.announceCreatedWindows()
	return m
}

func (m *Manager) UpdateConfig(cfg *config.RuntimeConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
	if loc, err := time.LoadLocation(cfg.Global.Timezone); err == nil {
		m.location = loc
	}
	m.announceCreatedWindows()
}

// IsEndpointInMaintenance returns true if the endpoint or its zone is currently in maintenance
func (m *Manager) IsEndpointInMaintenance(endpointID, zoneID string, now time.Time) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 1. Check one-off maintenance windows
	for _, om := range m.config.OneOffMaintenance {
		start, end, err := ParseOneOffDates(&om, m.location)
		if err == nil && !now.Before(start) && now.Before(end) {
			if isTargetAffected(endpointID, zoneID, om.AffectedZones, om.AffectedHosts) {
				return true
			}
		}
	}

	// 2. Check scheduled maintenance windows
	for _, sm := range m.config.ScheduledMaintenance {
		win, err := EvaluateCronWindow(&sm, now, m.location)
		if err == nil && win != nil && win.IsActive {
			if isTargetAffected(endpointID, zoneID, sm.AffectedZones, sm.AffectedHosts) {
				return true
			}
		}
	}

	return false
}

// GetActiveBanners returns active maintenance notices for dashboard display
func (m *Manager) GetActiveBanners(now time.Time) []ActiveWindow {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var banners []ActiveWindow

	// One-off
	for _, om := range m.config.OneOffMaintenance {
		start, end, err := ParseOneOffDates(&om, m.location)
		if err == nil && !now.Before(start) && now.Before(end) {
			banners = append(banners, ActiveWindow{
				ID:            om.ID,
				Message:       om.Message,
				StartTime:     start,
				EndTime:       end,
				IsActive:      true,
				AffectedZones: om.AffectedZones,
				AffectedHosts: om.AffectedHosts,
			})
		}
	}

	// Scheduled
	for _, sm := range m.config.ScheduledMaintenance {
		win, err := EvaluateCronWindow(&sm, now, m.location)
		if err == nil && win != nil && win.IsActive {
			banners = append(banners, *win)
		}
	}

	return banners
}

// CheckReminders runs periodically to trigger remindme notifications
func (m *Manager) CheckReminders(now time.Time) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.dispatcher == nil {
		return
	}

	// 1. One-off reminders
	for _, om := range m.config.OneOffMaintenance {
		if om.RemindMe <= 0 {
			continue
		}
		start, end, err := ParseOneOffDates(&om, m.location)
		if err != nil {
			continue
		}

		reminderThreshold := start.Add(-time.Duration(om.RemindMe) * time.Hour)
		if !now.Before(reminderThreshold) && now.Before(start) {
			when := fmt.Sprintf("%s - %s", start.Format("2006-01-02 15:04"), end.Format("15:04"))
			m.dispatcher.DispatchMaintenanceReminder(
				om.ID,
				om.Message,
				when,
				om.AffectedZones,
				om.AffectedHosts,
				start,
				now,
			)
		}
	}

	// 2. Scheduled reminders
	for _, sm := range m.config.ScheduledMaintenance {
		if sm.RemindMe <= 0 {
			continue
		}
		win, err := EvaluateCronWindow(&sm, now, m.location)
		if err != nil || win == nil {
			continue
		}

		reminderThreshold := win.StartTime.Add(-time.Duration(sm.RemindMe) * time.Hour)
		if !now.Before(reminderThreshold) && now.Before(win.StartTime) {
			when := fmt.Sprintf("%s (Schedule: %s)", win.StartTime.Format("2006-01-02 15:04"), sm.Schedule)
			m.dispatcher.DispatchMaintenanceReminder(
				sm.ID,
				sm.Message,
				when,
				sm.AffectedZones,
				sm.AffectedHosts,
				win.StartTime,
				now,
			)
		}
	}
}

func (m *Manager) announceCreatedWindows() {
	if m.dispatcher == nil {
		return
	}
	now := time.Now()

	for _, om := range m.config.OneOffMaintenance {
		start, end, err := ParseOneOffDates(&om, m.location)
		when := "Scheduled Window"
		if err == nil {
			when = fmt.Sprintf("%s - %s", start.Format("2006-01-02 15:04"), end.Format("15:04"))
		}
		m.dispatcher.DispatchMaintenanceCreated(om.ID, om.Message, when, om.AffectedZones, om.AffectedHosts, now)
	}

	for _, sm := range m.config.ScheduledMaintenance {
		when := fmt.Sprintf("Recurring: %s (%v)", sm.Schedule, sm.Duration.Duration())
		m.dispatcher.DispatchMaintenanceCreated(sm.ID, sm.Message, when, sm.AffectedZones, sm.AffectedHosts, now)
	}
}

// ParseOneOffDates parses start and end dates/times for a one-off maintenance window
func ParseOneOffDates(om *config.OneOffMaintenance, loc *time.Location) (time.Time, time.Time, error) {
	if loc == nil {
		loc = time.UTC
	}

	// Try ISO start/end
	if om.Start != "" && om.End != "" {
		s, err1 := time.Parse(time.RFC3339, om.Start)
		e, err2 := time.Parse(time.RFC3339, om.End)
		if err1 == nil && err2 == nil {
			if e.Before(s) {
				return time.Time{}, time.Time{}, fmt.Errorf("maintenance end time must be after start time")
			}
			return s, e, nil
		}
	}

	// Try date + time
	layout := "2006-01-02 15:04"
	startStr := fmt.Sprintf("%s %s", strings.TrimSpace(om.StartDate), strings.TrimSpace(om.StartTime))
	endStr := fmt.Sprintf("%s %s", strings.TrimSpace(om.EndDate), strings.TrimSpace(om.EndTime))

	s, err1 := time.ParseInLocation(layout, startStr, loc)
	e, err2 := time.ParseInLocation(layout, endStr, loc)

	if err1 != nil || err2 != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid one-off maintenance date formats")
	}

	if e.Before(s) {
		return time.Time{}, time.Time{}, fmt.Errorf("maintenance end time must be after start time")
	}

	return s, e, nil
}

func isTargetAffected(endpointID, zoneID string, affectedZones, affectedHosts []string) bool {
	// 1. If explicit host match, it is affected
	for _, h := range affectedHosts {
		if h != "all" && strings.EqualFold(h, endpointID) {
			return true
		}
	}

	// 2. If zone matches, check if all hosts or this host is affected
	zoneMatched := false
	for _, z := range affectedZones {
		if strings.EqualFold(z, zoneID) || z == "all" {
			zoneMatched = true
			break
		}
	}

	if zoneMatched {
		if len(affectedHosts) == 0 {
			return true
		}
		for _, h := range affectedHosts {
			if h == "all" || strings.EqualFold(h, endpointID) {
				return true
			}
		}
	}

	return false
}

// StartBackgroundChecker launches periodic reminder checks
func (m *Manager) StartBackgroundChecker(stopCh <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			m.CheckReminders(time.Now())
		}
	}
}
