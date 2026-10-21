package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Duration is a custom wrapper around time.Duration for YAML parsing
type Duration time.Duration

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		var n int
		if errInt := unmarshal(&n); errInt != nil {
			return err
		}
		*d = Duration(time.Duration(n) * time.Second)
		return nil
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration format %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// GlobalConfig represents settings in config.yml
type GlobalConfig struct {
	Interval             Duration `yaml:"interval"`
	Timeout              Duration `yaml:"timeout"`
	Retries              int      `yaml:"retries"`
	Timezone             string   `yaml:"timezone"`
	HistoryRetentionDays int      `yaml:"history_retention_days"`
	SelfhstIconCDN       string   `yaml:"selfhst_icon_cdn"`
}

// Zone represents a grouping in zones/*.yml
type Zone struct {
	ID          string `yaml:"id"`
	DisplayName string `yaml:"display_name"`
	Description string `yaml:"description"`
	Order       int    `yaml:"order"`
	Icon        string `yaml:"icon"`
}

// Endpoint represents an endpoint in endpoints/*.yml
type Endpoint struct {
	ID                 string      `yaml:"id"`
	DisplayName        string      `yaml:"display_name"`
	Zone               string      `yaml:"zone"`
	Icon               string      `yaml:"icon"`
	Interval           *Duration   `yaml:"interval"`
	Timeout            *Duration   `yaml:"timeout"`
	Retries            *int        `yaml:"retries"`
	Type               string      `yaml:"type"` // ping, tcp, dns, https
	ShowTarget         bool        `yaml:"show_target"`

	// Protocol specific fields
	Target             string      `yaml:"target"`               // ping, tcp, https
	Port               int         `yaml:"port"`                 // tcp, dns
	Hostname           string      `yaml:"hostname"`             // dns
	Server             string      `yaml:"server"`               // dns
	Protocol           string      `yaml:"protocol"`             // dns: udp, tcp, dot, doh
	QueryType          string      `yaml:"query_type"`           // dns: A, AAAA, CNAME, TXT, MX
	Method             string      `yaml:"method"`               // https: GET, POST, HEAD, etc.
	AcceptedStatusCodes []string   `yaml:"accepted_status_codes"` // https: 20X, 301, 200-204
	IgnoreTLSErrors    bool        `yaml:"ignore_tls_errors"`    // https
	TLSIgnoreWarnDays  []int       `yaml:"-"` // https: e.g. [14, 7, 3]
}

// UnmarshalYAML handles both single int and array of ints for tls_expiry_warn_days
func (e *Endpoint) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawEndpoint Endpoint
	var raw struct {
		rawEndpoint `yaml:",inline"`
		RawWarnDays interface{} `yaml:"tls_expiry_warn_days"`
	}

	if err := unmarshal(&raw); err != nil {
		return err
	}

	*e = Endpoint(raw.rawEndpoint)

	if raw.RawWarnDays != nil {
		switch v := raw.RawWarnDays.(type) {
		case int:
			e.TLSIgnoreWarnDays = []int{v}
		case []interface{}:
			for _, item := range v {
				if num, ok := item.(int); ok {
					e.TLSIgnoreWarnDays = append(e.TLSIgnoreWarnDays, num)
				} else if str, ok := item.(string); ok {
					if parsed, err := strconv.Atoi(str); err == nil {
						e.TLSIgnoreWarnDays = append(e.TLSIgnoreWarnDays, parsed)
					}
				}
			}
		}
	}

	return nil
}

// OneOffMaintenance represents maintenance/*.yml
type OneOffMaintenance struct {
	ID             string   `yaml:"id"`
	Title          string   `yaml:"title"`
	StartDate      string   `yaml:"start_date"` // YYYY-MM-DD
	StartTime      string   `yaml:"start_time"` // HH:MM
	EndDate        string   `yaml:"end_date"`   // YYYY-MM-DD
	EndTime        string   `yaml:"end_time"`   // HH:MM
	Start          string   `yaml:"start"`      // Or ISO-8601
	End            string   `yaml:"end"`        // Or ISO-8601
	Message        string   `yaml:"message"`
	RemindMe       int      `yaml:"remindme"` // hours prior to start
	AffectedZones  []string `yaml:"affected_zones"`
	AffectedHosts  []string `yaml:"affected_hosts"` // list of endpoint IDs or ["all"]
}

// ScheduledMaintenance represents maintenance/scheduled/*.yml
type ScheduledMaintenance struct {
	ID             string   `yaml:"id"`
	Title          string   `yaml:"title"`
	Schedule       string   `yaml:"schedule"` // 5-field CRON
	Duration       Duration `yaml:"duration"` // e.g. 2h, 90m
	StartDate      string   `yaml:"start_date"`
	EndDate        string   `yaml:"end_date"`
	StartTime      string   `yaml:"start_time"`
	EndTime        string   `yaml:"end_time"`
	Message        string   `yaml:"message"`
	RemindMe       int      `yaml:"remindme"`
	AffectedZones  []string `yaml:"affected_zones"`
	AffectedHosts  []string `yaml:"affected_hosts"`
}

// WebhookTarget represents webhook in alerting/webhooks.yml
type WebhookTarget struct {
	ID     string   `yaml:"id"`
	Type   string   `yaml:"type"` // discord, generic
	URL    string   `yaml:"url"`
	Events []string `yaml:"events"`
}

// AlertingConfig represents alerting/webhooks.yml
type AlertingConfig struct {
	Webhooks []WebhookTarget `yaml:"webhooks"`
}

// RuntimeConfig combines all parsed configurations
type RuntimeConfig struct {
	Global                 GlobalConfig
	Zones                  map[string]Zone
	Endpoints              map[string]Endpoint
	OneOffMaintenance      map[string]OneOffMaintenance
	ScheduledMaintenance  map[string]ScheduledMaintenance
	Alerting               AlertingConfig
}

// DefaultGlobalConfig returns default settings
func DefaultGlobalConfig() GlobalConfig {
	return GlobalConfig{
		Interval:             Duration(60 * time.Second),
		Timeout:              Duration(5 * time.Second),
		Retries:              3,
		Timezone:             "Europe/Berlin",
		HistoryRetentionDays: 30,
		SelfhstIconCDN:       "https://cdn.jsdelivr.net/gh/selfhst/icons/svg/{icon}.svg",
	}
}

// MatchStatusCode checks if code matches patterns like "20X", "30X", "200-204", "200"
func MatchStatusCode(code int, patterns []string) bool {
	if len(patterns) == 0 {
		return code >= 200 && code < 300
	}
	codeStr := strconv.Itoa(code)
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == codeStr {
			return true
		}
		if strings.HasSuffix(strings.ToUpper(p), "X") {
			prefix := strings.TrimSuffix(strings.ToUpper(p), "X")
			if strings.HasPrefix(codeStr, prefix) {
				return true
			}
		}
		if strings.Contains(p, "-") {
			parts := strings.SplitN(p, "-", 2)
			if len(parts) == 2 {
				minVal, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
				maxVal, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err1 == nil && err2 == nil && code >= minVal && code <= maxVal {
					return true
				}
			}
		}
	}
	return false
}
