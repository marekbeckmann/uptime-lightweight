package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads all configuration files from configDir and returns a RuntimeConfig
func Load(configDir string) (*RuntimeConfig, error) {
	rc := &RuntimeConfig{
		Global:                DefaultGlobalConfig(),
		Zones:                 make(map[string]Zone),
		Endpoints:             make(map[string]Endpoint),
		OneOffMaintenance:     make(map[string]OneOffMaintenance),
		ScheduledMaintenance: make(map[string]ScheduledMaintenance),
		Alerting:              AlertingConfig{},
	}

	// 1. Load config.yml if it exists
	configFile := filepath.Join(configDir, "config.yml")
	if data, err := os.ReadFile(configFile); err == nil {
		var g GlobalConfig
		if err := yaml.Unmarshal(data, &g); err != nil {
			return nil, fmt.Errorf("error parsing %s: %w", configFile, err)
		}
		if g.Interval.Duration() > 0 {
			rc.Global.Interval = g.Interval
		}
		if g.Timeout.Duration() > 0 {
			rc.Global.Timeout = g.Timeout
		}
		if g.Retries > 0 {
			rc.Global.Retries = g.Retries
		}
		if g.Timezone != "" {
			rc.Global.Timezone = g.Timezone
		}
		if g.HistoryRetentionDays > 0 {
			rc.Global.HistoryRetentionDays = g.HistoryRetentionDays
		}
		if g.SelfhstIconCDN != "" {
			rc.Global.SelfhstIconCDN = g.SelfhstIconCDN
		}
		// Merge and default branding options
		if strings.TrimSpace(g.Branding.Title) != "" {
			rc.Global.Branding.Title = strings.TrimSpace(g.Branding.Title)
		}
		if strings.TrimSpace(g.Branding.Subtitle) != "" {
			rc.Global.Branding.Subtitle = strings.TrimSpace(g.Branding.Subtitle)
		}
		if strings.TrimSpace(g.Branding.Footer) != "" {
			rc.Global.Branding.Footer = strings.TrimSpace(g.Branding.Footer)
		}
		if strings.TrimSpace(g.Branding.Icon) != "" {
			rc.Global.Branding.Icon = strings.TrimSpace(g.Branding.Icon)
		}
		if strings.TrimSpace(g.Branding.Favicon) != "" {
			rc.Global.Branding.Favicon = strings.TrimSpace(g.Branding.Favicon)
		}
		if strings.TrimSpace(g.Branding.TabTitle) != "" {
			rc.Global.Branding.TabTitle = strings.TrimSpace(g.Branding.TabTitle)
		} else {
			rc.Global.Branding.TabTitle = rc.Global.Branding.Title + " | Status Dashboard"
		}
	}

	// 2. Load zones from zones/*.yml
	zonesDir := filepath.Join(configDir, "zones")
	if entries, err := os.ReadDir(zonesDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !isYAML(entry.Name()) {
				continue
			}
			path := filepath.Join(zonesDir, entry.Name())
			zones, err := loadZonesFile(path)
			if err != nil {
				return nil, err
			}
			for _, z := range zones {
				if z.ID == "" {
					z.ID = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
				}
				rc.Zones[z.ID] = z
			}
		}
	}

	// 3. Load endpoints from endpoints/*.yml
	endpointsDir := filepath.Join(configDir, "endpoints")
	if entries, err := os.ReadDir(endpointsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !isYAML(entry.Name()) {
				continue
			}
			path := filepath.Join(endpointsDir, entry.Name())
			endpoints, err := loadEndpointsFile(path)
			if err != nil {
				return nil, err
			}
			for i, ep := range endpoints {
				if ep.ID == "" {
					base := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
					if len(endpoints) == 1 {
						ep.ID = base
					} else {
						ep.ID = fmt.Sprintf("%s-%d", base, i+1)
					}
				}
				rc.Endpoints[ep.ID] = ep
			}
		}
	}

	// 4. Load maintenance from maintenance/*.yml and maintenance/scheduled/*.yml
	maintDir := filepath.Join(configDir, "maintenance")
	if entries, err := os.ReadDir(maintDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if !isYAML(entry.Name()) {
				continue
			}
			path := filepath.Join(maintDir, entry.Name())
			m, err := loadOneOffMaintenance(path)
			if err != nil {
				return nil, err
			}
			if m.ID == "" {
				m.ID = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			}
			if strings.TrimSpace(m.Title) == "" {
				m.Title = m.ID
			}
			rc.OneOffMaintenance[m.ID] = *m
		}
	}

	schedDir := filepath.Join(configDir, "maintenance", "scheduled")
	if entries, err := os.ReadDir(schedDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !isYAML(entry.Name()) {
				continue
			}
			path := filepath.Join(schedDir, entry.Name())
			sm, err := loadScheduledMaintenance(path)
			if err != nil {
				return nil, err
			}
			if sm.ID == "" {
				sm.ID = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			}
			if strings.TrimSpace(sm.Title) == "" {
				sm.Title = sm.ID
			}
			rc.ScheduledMaintenance[sm.ID] = *sm
		}
	}

	// 5. Load alerting from alerting/webhooks.yml
	alertFile := filepath.Join(configDir, "alerting", "webhooks.yml")
	if data, err := os.ReadFile(alertFile); err == nil {
		var a AlertingConfig
		if err := yaml.Unmarshal(data, &a); err != nil {
			return nil, fmt.Errorf("error parsing %s: %w", alertFile, err)
		}
		rc.Alerting = a
	}

	return rc, nil
}

func isYAML(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yml" || ext == ".yaml"
}

func loadZonesFile(path string) ([]Zone, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var single Zone
	if err := yaml.Unmarshal(data, &single); err == nil && single.DisplayName != "" {
		return []Zone{single}, nil
	}

	var list []Zone
	if err := yaml.Unmarshal(data, &list); err == nil && len(list) > 0 {
		return list, nil
	}

	return nil, fmt.Errorf("invalid zone format in %s", path)
}

func loadEndpointsFile(path string) ([]Endpoint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var list []Endpoint
	if err := yaml.Unmarshal(data, &list); err == nil && len(list) > 0 && list[0].Type != "" {
		return list, nil
	}

	var single Endpoint
	if err := yaml.Unmarshal(data, &single); err == nil && single.Type != "" {
		return []Endpoint{single}, nil
	}

	return nil, fmt.Errorf("invalid endpoint format in %s", path)
}

func loadOneOffMaintenance(path string) (*OneOffMaintenance, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var m OneOffMaintenance
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &m, nil
}

func loadScheduledMaintenance(path string) (*ScheduledMaintenance, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var sm ScheduledMaintenance
	if err := yaml.Unmarshal(data, &sm); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &sm, nil
}
