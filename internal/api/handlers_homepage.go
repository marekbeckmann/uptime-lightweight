package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// HomepageSummaryResponse matches the gethomepage.dev customapi format
type HomepageSummaryResponse struct {
	HostsUp   int `json:"Hosts Up"`
	HostsDown int `json:"Hosts Down"`
	Uptime    int `json:"Uptime"`
}

// HandleHomepageSummary handles GET /api/v1/status/summary
func (s *Server) HandleHomepageSummary(w http.ResponseWriter, r *http.Request) {
	// Authenticate request
	if !s.AuthenticateToken(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "unauthorized",
			"message": "Invalid or revoked API credentials",
		})
		return
	}

	now := time.Now()
	states, err := s.DB.GetAllStates()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var hostsUp, hostsDown int
	for _, ep := range s.Config.Endpoints {
		st, ok := states[ep.ID]
		status := "PENDING"
		if ok {
			status = st.Status
		}

		// Maintenance check: endpoints in MAINTENANCE must NOT increment Hosts Down (FR-027)
		if s.IsMaintenanceActive != nil && s.IsMaintenanceActive(ep.ID, ep.Zone, now) {
			status = "MAINTENANCE"
		}

		if status == "UP" {
			hostsUp++
		} else if status == "DOWN" {
			hostsDown++
		}
	}

	uptime, _ := s.DB.CalculateGlobalUptimePercentage(24 * time.Hour)

	resp := HomepageSummaryResponse{
		HostsUp:   hostsUp,
		HostsDown: hostsDown,
		Uptime:    uptime,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
