package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"
	"uptime-mon/internal/config"
	"uptime-mon/internal/maintenance"
	"uptime-mon/internal/storage"
)

// MaintenanceTargetRef maps an internal ID to its human-readable display name
type MaintenanceTargetRef struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// OneOffMaintenanceView represents a future or currently active one-off maintenance
type OneOffMaintenanceView struct {
	ID            string                 `json:"id"`
	Title         string                 `json:"title"`
	Message       string                 `json:"message"`
	StartAt       time.Time              `json:"start_at"`
	EndAt         time.Time              `json:"end_at"`
	IsActive      bool                   `json:"is_active"`
	AffectedZones []MaintenanceTargetRef `json:"affected_zones"`
	AffectedHosts []MaintenanceTargetRef `json:"affected_hosts"`
	AllHosts      bool                   `json:"all_hosts"`
}

// ScheduledMaintenanceView represents a recurring scheduled maintenance policy
type ScheduledMaintenanceView struct {
	ID                string                 `json:"id"`
	Title             string                 `json:"title"`
	Message           string                 `json:"message"`
	Schedule          string                 `json:"schedule"`
	Duration          string                 `json:"duration"`
	StartAt           *time.Time             `json:"start_at,omitempty"`
	EndAt             *time.Time             `json:"end_at,omitempty"`
	NextRunAt         time.Time              `json:"next_run_at"`
	IsCurrentlyActive bool                   `json:"is_currently_active"`
	AffectedZones     []MaintenanceTargetRef `json:"affected_zones"`
	AffectedHosts     []MaintenanceTargetRef `json:"affected_hosts"`
	AllHosts          bool                   `json:"all_hosts"`
}

type DashboardBrandingView struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Footer   string `json:"footer"`
	Icon     string `json:"icon"`
	Favicon  string `json:"favicon"`
	TabTitle string `json:"tab_title"`
}

// PublicDashboardResponse is the top-level payload for GET /api/v1/dashboard
type PublicDashboardResponse struct {
	SystemStatus          string                     `json:"system_status"`
	Counts                DashboardCounts            `json:"counts"`
	Branding              DashboardBrandingView      `json:"branding"`
	OneOffMaintenances    []OneOffMaintenanceView    `json:"one_off_maintenances"`
	ScheduledMaintenances []ScheduledMaintenanceView `json:"scheduled_maintenances"`
	MaintenanceBanners    []DashboardMaintenanceView `json:"maintenance_banners"` // Retained for backward compatibility
	Zones                 []DashboardZoneView        `json:"zones"`
}

type DashboardCounts struct {
	HostsUp                   int     `json:"hosts_up"`
	HostsDown                 int     `json:"hosts_down"`
	InMaintenance             int     `json:"in_maintenance"`
	ScheduledMaintenanceCount int     `json:"scheduled_maintenance_count"`
	UptimePercentage          float64 `json:"uptime_percentage"`
}

type DashboardMaintenanceView struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	IsActive  bool      `json:"is_active"`
}

type DashboardZoneView struct {
	ID          string                  `json:"id"`
	DisplayName string                  `json:"display_name"`
	Description string                  `json:"description,omitempty"`
	Order       int                     `json:"order"`
	Icon        string                  `json:"icon,omitempty"`
	Endpoints   []DashboardEndpointView `json:"endpoints"`
}

type DashboardEndpointView struct {
	ID          string                     `json:"id"`
	DisplayName string                     `json:"display_name"`
	Zone        string                     `json:"zone"`
	Icon        string                     `json:"icon,omitempty"`
	Status      string                     `json:"status"`
	LatencyMS   int                        `json:"latency_ms"`
	Uptime24h   float64                    `json:"uptime_24h"`
	ShowTarget  bool                       `json:"show_target"`
	Target      *string                    `json:"target,omitempty"` // Strictly concealed if ShowTarget == false
	Heartbeats  []storage.ProbeHistoryItem `json:"heartbeats"`
}

// Helper to resolve zone and endpoint IDs into display names
func resolveMaintenanceTargets(cfg *config.RuntimeConfig, affectedZones, affectedHosts []string) ([]MaintenanceTargetRef, []MaintenanceTargetRef, bool) {
	var zones []MaintenanceTargetRef
	for _, zid := range affectedZones {
		name := zid
		if z, ok := cfg.Zones[zid]; ok && z.DisplayName != "" {
			name = z.DisplayName
		}
		zones = append(zones, MaintenanceTargetRef{ID: zid, DisplayName: name})
	}

	var hosts []MaintenanceTargetRef
	allHosts := false
	for _, hid := range affectedHosts {
		if strings.EqualFold(hid, "all") {
			allHosts = true
			continue
		}
		name := hid
		if ep, ok := cfg.Endpoints[hid]; ok && ep.DisplayName != "" {
			name = ep.DisplayName
		}
		hosts = append(hosts, MaintenanceTargetRef{ID: hid, DisplayName: name})
	}
	return zones, hosts, allHosts
}

// BuildDashboardResponse creates the sanitized dashboard view
func BuildDashboardResponse(cfg *config.RuntimeConfig, db *storage.DB, isMaintFunc func(string, string, time.Time) bool) (*PublicDashboardResponse, error) {
	now := time.Now()
	loc, err := time.LoadLocation(cfg.Global.Timezone)
	if err != nil {
		loc = time.UTC
	}

	states, err := db.GetAllStates()
	if err != nil {
		return nil, err
	}

	var totalUp, totalDown, totalMaint int

	// 1. Group endpoints by zone
	zoneMap := make(map[string][]DashboardEndpointView)
	for _, ep := range cfg.Endpoints {
		st, ok := states[ep.ID]
		status := "PENDING"
		latency := 0
		if ok {
			status = st.Status
			latency = st.LastLatencyMS
		}

		// Check active maintenance override
		if isMaintFunc != nil && isMaintFunc(ep.ID, ep.Zone, now) {
			status = "MAINTENANCE"
		}

		switch status {
		case "UP":
			totalUp++
		case "DOWN":
			totalDown++
		case "MAINTENANCE":
			totalMaint++
		}

		uptime24h, _ := db.CalculateUptimePercentage(ep.ID, 24*time.Hour)
		heartbeats, _ := db.GetRecentHistory(ep.ID, 60)
		if heartbeats == nil {
			heartbeats = []storage.ProbeHistoryItem{}
		}

		view := DashboardEndpointView{
			ID:          ep.ID,
			DisplayName: ep.DisplayName,
			Zone:        ep.Zone,
			Icon:        ep.Icon,
			Status:      status,
			LatencyMS:   latency,
			Uptime24h:   uptime24h,
			ShowTarget:  ep.ShowTarget,
			Heartbeats:  heartbeats,
		}

		// Security Enforcement (Principle II): Conceal target unless show_target is explicitly true
		if ep.ShowTarget {
			targetVal := ep.Target
			if targetVal == "" && ep.Hostname != "" {
				targetVal = ep.Hostname
			}
			view.Target = &targetVal
		}

		zoneMap[ep.Zone] = append(zoneMap[ep.Zone], view)
	}

	// 2. Build zones list sorted by order
	var zones []DashboardZoneView
	for _, z := range cfg.Zones {
		eps := zoneMap[z.ID]
		if eps == nil {
			eps = []DashboardEndpointView{}
		}
		sort.Slice(eps, func(i, j int) bool {
			return eps[i].DisplayName < eps[j].DisplayName
		})

		zones = append(zones, DashboardZoneView{
			ID:          z.ID,
			DisplayName: z.DisplayName,
			Description: z.Description,
			Order:       z.Order,
			Icon:        z.Icon,
			Endpoints:   eps,
		})
	}

	sort.Slice(zones, func(i, j int) bool {
		if zones[i].Order != zones[j].Order {
			return zones[i].Order < zones[j].Order
		}
		return zones[i].DisplayName < zones[j].DisplayName
	})

	// 3. One-Off Maintenances (where EndAt.After(now), sorted by StartAt ASC)
	var oneOffMaintenances []OneOffMaintenanceView
	var legacyBanners []DashboardMaintenanceView

	for _, om := range cfg.OneOffMaintenance {
		start, end, err := maintenance.ParseOneOffDates(&om, loc)
		if err != nil {
			continue
		}
		// Only display items where EndAt.After(now)
		if end.After(now) {
			isActive := !now.Before(start) && now.Before(end)
			zRefs, hRefs, allHosts := resolveMaintenanceTargets(cfg, om.AffectedZones, om.AffectedHosts)

			title := om.Title
			if title == "" {
				title = om.ID
			}

			oneOffMaintenances = append(oneOffMaintenances, OneOffMaintenanceView{
				ID:            om.ID,
				Title:         title,
				Message:       om.Message,
				StartAt:       start,
				EndAt:         end,
				IsActive:      isActive,
				AffectedZones: zRefs,
				AffectedHosts: hRefs,
				AllHosts:      allHosts,
			})

			// Backward compatibility legacy banner
			if isActive {
				legacyBanners = append(legacyBanners, DashboardMaintenanceView{
					ID:        om.ID,
					Message:   om.Message,
					StartTime: start,
					EndTime:   end,
					IsActive:  true,
				})
			}
		}
	}

	sort.Slice(oneOffMaintenances, func(i, j int) bool {
		return oneOffMaintenances[i].StartAt.Before(oneOffMaintenances[j].StartAt)
	})

	// 4. Scheduled Maintenances (where overall validity has not expired, sorted by NextRunAt ASC)
	var scheduledMaintenances []ScheduledMaintenanceView

	for _, sm := range cfg.ScheduledMaintenance {
		// Check overall validity bounds if defined
		if sm.EndDate != "" {
			endLayout := "2006-01-02"
			if sm.EndTime != "" {
				endLayout = "2006-01-02 15:04"
			}
			endStr := strings.TrimSpace(sm.EndDate)
			if sm.EndTime != "" {
				endStr = endStr + " " + strings.TrimSpace(sm.EndTime)
			}
			if overallEnd, err := time.ParseInLocation(endLayout, endStr, loc); err == nil {
				if overallEnd.Before(now) {
					continue // Rule has expired permanently
				}
			}
		}

		win, err := maintenance.EvaluateCronWindow(&sm, now, loc)
		if err != nil || win == nil {
			continue
		}

		zRefs, hRefs, allHosts := resolveMaintenanceTargets(cfg, sm.AffectedZones, sm.AffectedHosts)

		title := sm.Title
		if title == "" {
			title = sm.ID
		}

		durationStr := sm.Duration.Duration().String()
		if durationStr == "0s" {
			durationStr = "1h"
		}

		scheduledMaintenances = append(scheduledMaintenances, ScheduledMaintenanceView{
			ID:                sm.ID,
			Title:             title,
			Message:           sm.Message,
			Schedule:          sm.Schedule,
			Duration:          durationStr,
			NextRunAt:         win.StartTime,
			IsCurrentlyActive: win.IsActive,
			AffectedZones:     zRefs,
			AffectedHosts:     hRefs,
			AllHosts:          allHosts,
		})
	}

	sort.Slice(scheduledMaintenances, func(i, j int) bool {
		return scheduledMaintenances[i].NextRunAt.Before(scheduledMaintenances[j].NextRunAt)
	})

	// 5. Compute overall system status and uptime percentage
	sysStatus := "UP"
	if totalDown > 0 {
		sysStatus = "DOWN"
	} else if totalMaint > 0 && totalUp == 0 {
		sysStatus = "MAINTENANCE"
	} else if totalUp == 0 && totalDown == 0 {
		sysStatus = "PENDING"
	}

	globalUptime, _ := db.CalculateGlobalUptimePercentage(24 * time.Hour)

	// Build resolved branding
	b := cfg.Global.Branding
	title := b.Title
	if title == "" {
		title = "uptime-mon"
	}
	subtitle := b.Subtitle
	if subtitle == "" {
		subtitle = "Continuous GitOps Uptime & Protocol Monitoring"
	}
	footer := b.Footer
	if footer == "" {
		footer = "uptime-mon • Single-Binary Declarative Uptime Monitor"
	}
	tabTitle := b.TabTitle
	if tabTitle == "" {
		tabTitle = title + " | Status Dashboard"
	}

	icon := b.Icon
	if icon != "" && !strings.HasPrefix(icon, "http://") && !strings.HasPrefix(icon, "https://") && !strings.HasPrefix(icon, "data:") {
		cdnPattern := cfg.Global.SelfhstIconCDN
		if cdnPattern == "" {
			cdnPattern = "https://cdn.jsdelivr.net/gh/selfhst/icons/svg/{icon}.svg"
		}
		icon = strings.Replace(cdnPattern, "{icon}", icon, 1)
	}

	favicon := b.Favicon
	if favicon == "" && icon != "" && (strings.HasPrefix(icon, "http://") || strings.HasPrefix(icon, "https://") || strings.HasPrefix(icon, "data:")) {
		favicon = icon
	}

	brandingView := DashboardBrandingView{
		Title:    title,
		Subtitle: subtitle,
		Footer:   footer,
		Icon:     icon,
		Favicon:  favicon,
		TabTitle: tabTitle,
	}

	resp := &PublicDashboardResponse{
		SystemStatus: sysStatus,
		Counts: DashboardCounts{
			HostsUp:                   totalUp,
			HostsDown:                 totalDown,
			InMaintenance:             totalMaint,
			ScheduledMaintenanceCount: len(scheduledMaintenances),
			UptimePercentage:          float64(globalUptime),
		},
		Branding:              brandingView,
		OneOffMaintenances:    oneOffMaintenances,
		ScheduledMaintenances: scheduledMaintenances,
		MaintenanceBanners:    legacyBanners,
		Zones:                 zones,
	}

	return resp, nil
}

// HandleDashboard serves GET /api/v1/dashboard
func (s *Server) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp, err := BuildDashboardResponse(s.Config, s.DB, s.IsMaintenanceActive)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(resp)
}
