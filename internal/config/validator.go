package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// Validate checks the RuntimeConfig for semantic and cross-reference errors
func Validate(rc *RuntimeConfig) []string {
	var errors []string

	// 1. Validate zones
	if len(rc.Zones) == 0 {
		errors = append(errors, "at least one zone must be defined in ./config/zones/")
	}
	for id, z := range rc.Zones {
		if strings.TrimSpace(z.ID) == "" {
			errors = append(errors, fmt.Sprintf("zone %q: id must not be empty", id))
		}
		if strings.TrimSpace(z.DisplayName) == "" {
			errors = append(errors, fmt.Sprintf("zone %q: display_name must not be empty", id))
		}
	}

	// 2. Validate endpoints
	if len(rc.Endpoints) == 0 {
		errors = append(errors, "at least one endpoint must be defined in ./config/endpoints/")
	}
	for id, ep := range rc.Endpoints {
		if strings.TrimSpace(ep.ID) == "" {
			errors = append(errors, fmt.Sprintf("endpoint %q: id must not be empty", id))
		}
		if strings.TrimSpace(ep.DisplayName) == "" {
			errors = append(errors, fmt.Sprintf("endpoint %q: display_name must not be empty", id))
		}
		if ep.Zone == "" {
			errors = append(errors, fmt.Sprintf("endpoint %q: zone reference must not be empty", id))
		} else if _, exists := rc.Zones[ep.Zone]; !exists {
			errors = append(errors, fmt.Sprintf("endpoint %q: referenced zone %q does not exist in ./config/zones/", id, ep.Zone))
		}

		switch strings.ToLower(ep.Type) {
		case "ping":
			if strings.TrimSpace(ep.Target) == "" {
				errors = append(errors, fmt.Sprintf("endpoint %q (ping): target (IP or hostname) must not be empty", id))
			}
		case "tcp":
			if strings.TrimSpace(ep.Target) == "" {
				errors = append(errors, fmt.Sprintf("endpoint %q (tcp): target must not be empty", id))
			}
			if ep.Port < 1 || ep.Port > 65535 {
				errors = append(errors, fmt.Sprintf("endpoint %q (tcp): port must be between 1 and 65535, got %d", id, ep.Port))
			}
		case "dns":
			if strings.TrimSpace(ep.Hostname) == "" {
				errors = append(errors, fmt.Sprintf("endpoint %q (dns): hostname must not be empty", id))
			}
			if strings.TrimSpace(ep.Server) == "" {
				errors = append(errors, fmt.Sprintf("endpoint %q (dns): server must not be empty", id))
			}
			proto := strings.ToLower(ep.Protocol)
			if proto != "" && proto != "udp" && proto != "tcp" && proto != "dot" && proto != "doh" {
				errors = append(errors, fmt.Sprintf("endpoint %q (dns): invalid protocol %q, must be udp, tcp, dot, or doh", id, ep.Protocol))
			}
			qtype := strings.ToUpper(ep.QueryType)
			if qtype != "" && qtype != "A" && qtype != "AAAA" && qtype != "CNAME" && qtype != "TXT" && qtype != "MX" {
				errors = append(errors, fmt.Sprintf("endpoint %q (dns): unsupported query_type %q", id, ep.QueryType))
			}
		case "https":
			if strings.TrimSpace(ep.Target) == "" {
				errors = append(errors, fmt.Sprintf("endpoint %q (https): target URL must not be empty", id))
			} else {
				parsed, err := url.Parse(ep.Target)
				if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
					errors = append(errors, fmt.Sprintf("endpoint %q (https): target %q must be a valid http or https URL", id, ep.Target))
				}
			}
		default:
			errors = append(errors, fmt.Sprintf("endpoint %q: unsupported type %q (must be ping, tcp, dns, or https)", id, ep.Type))
		}
	}

	// 3. Validate one-off maintenance
	for id, m := range rc.OneOffMaintenance {
		if strings.TrimSpace(m.Message) == "" {
			errors = append(errors, fmt.Sprintf("maintenance %q: message must not be empty", id))
		}
		for _, zID := range m.AffectedZones {
			if _, exists := rc.Zones[zID]; !exists {
				errors = append(errors, fmt.Sprintf("maintenance %q: referenced zone %q does not exist", id, zID))
			}
		}
		for _, hID := range m.AffectedHosts {
			if hID != "all" {
				if _, exists := rc.Endpoints[hID]; !exists {
					errors = append(errors, fmt.Sprintf("maintenance %q: referenced endpoint %q does not exist", id, hID))
				}
			}
		}
		if m.Start != "" && m.End != "" {
			s, err1 := time.Parse(time.RFC3339, m.Start)
			e, err2 := time.Parse(time.RFC3339, m.End)
			if err1 != nil || err2 != nil {
				errors = append(errors, fmt.Sprintf("maintenance %q: invalid start/end RFC3339 format", id))
			} else if e.Before(s) {
				errors = append(errors, fmt.Sprintf("maintenance %q: end time must be after start time", id))
			}
		} else if m.StartDate != "" && m.EndDate != "" {
			layout := "2006-01-02 15:04"
			sStr := fmt.Sprintf("%s %s", strings.TrimSpace(m.StartDate), strings.TrimSpace(m.StartTime))
			eStr := fmt.Sprintf("%s %s", strings.TrimSpace(m.EndDate), strings.TrimSpace(m.EndTime))
			s, err1 := time.Parse(layout, sStr)
			e, err2 := time.Parse(layout, eStr)
			if err1 != nil || err2 != nil {
				errors = append(errors, fmt.Sprintf("maintenance %q: invalid start/end date format (must be YYYY-MM-DD HH:MM)", id))
			} else if e.Before(s) {
				errors = append(errors, fmt.Sprintf("maintenance %q: end time must be after start time", id))
			}
		}
	}

	// 4. Validate scheduled maintenance
	cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	for id, sm := range rc.ScheduledMaintenance {
		if strings.TrimSpace(sm.Message) == "" {
			errors = append(errors, fmt.Sprintf("scheduled maintenance %q: message must not be empty", id))
		}
		if sm.Schedule == "" {
			errors = append(errors, fmt.Sprintf("scheduled maintenance %q: schedule CRON expression must not be empty", id))
		} else {
			if _, err := cronParser.Parse(sm.Schedule); err != nil {
				errors = append(errors, fmt.Sprintf("scheduled maintenance %q: invalid CRON schedule %q: %v", id, sm.Schedule, err))
			}
		}
		for _, zID := range sm.AffectedZones {
			if _, exists := rc.Zones[zID]; !exists {
				errors = append(errors, fmt.Sprintf("scheduled maintenance %q: referenced zone %q does not exist", id, zID))
			}
		}
		for _, hID := range sm.AffectedHosts {
			if hID != "all" {
				if _, exists := rc.Endpoints[hID]; !exists {
					errors = append(errors, fmt.Sprintf("scheduled maintenance %q: referenced endpoint %q does not exist", id, hID))
				}
			}
		}
	}

	// 5. Validate webhooks
	for _, w := range rc.Alerting.Webhooks {
		wType := strings.ToLower(w.Type)
		if wType != "discord" && wType != "generic" {
			errors = append(errors, fmt.Sprintf("webhook %q: invalid type %q, must be discord or generic", w.ID, w.Type))
		}
		if strings.TrimSpace(w.URL) == "" {
			errors = append(errors, fmt.Sprintf("webhook %q: url must not be empty", w.ID))
		}
	}

	// 6. Validate branding
	if strings.HasPrefix(rc.Global.Branding.Favicon, "http://") || strings.HasPrefix(rc.Global.Branding.Favicon, "https://") {
		if _, err := url.ParseRequestURI(rc.Global.Branding.Favicon); err != nil {
			errors = append(errors, fmt.Sprintf("branding: invalid favicon URL %q: %v", rc.Global.Branding.Favicon, err))
		}
	}
	if strings.HasPrefix(rc.Global.Branding.Icon, "http://") || strings.HasPrefix(rc.Global.Branding.Icon, "https://") {
		if _, err := url.ParseRequestURI(rc.Global.Branding.Icon); err != nil {
			errors = append(errors, fmt.Sprintf("branding: invalid icon URL %q: %v", rc.Global.Branding.Icon, err))
		}
	}

	return errors
}
