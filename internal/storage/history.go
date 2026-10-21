package storage

import (
	"time"
)

// ProbeHistoryItem represents a single recorded probe
type ProbeHistoryItem struct {
	ID         int64     `json:"id"`
	EndpointID string    `json:"endpoint_id"`
	CheckedAt  time.Time `json:"timestamp"`
	Status     string    `json:"status"`
	LatencyMS  int       `json:"latency_ms"`
	Error      string    `json:"error,omitempty"`
}

// RecordProbe saves a single probe check
func (db *DB) RecordProbe(endpointID, status string, latencyMS int, errorMsg string, checkedAt time.Time) error {
	_, err := db.Exec(`
		INSERT INTO probe_history (endpoint_id, checked_at, status, latency_ms, error)
		VALUES (?, ?, ?, ?, ?)`,
		endpointID, checkedAt, status, latencyMS, errorMsg)
	return err
}

// GetRecentHistory retrieves the latest N probe results for an endpoint (ordered oldest to newest for UI)
func (db *DB) GetRecentHistory(endpointID string, limit int) ([]ProbeHistoryItem, error) {
	rows, err := db.Query(`
		SELECT id, endpoint_id, checked_at, status, latency_ms, error
		FROM probe_history
		WHERE endpoint_id = ?
		ORDER BY checked_at DESC
		LIMIT ?`, endpointID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var desc []ProbeHistoryItem
	for rows.Next() {
		var item ProbeHistoryItem
		if err := rows.Scan(&item.ID, &item.EndpointID, &item.CheckedAt, &item.Status, &item.LatencyMS, &item.Error); err != nil {
			return nil, err
		}
		desc = append(desc, item)
	}

	// Reverse to oldest-to-newest for chart/bar display
	count := len(desc)
	asc := make([]ProbeHistoryItem, count)
	for i := 0; i < count; i++ {
		asc[i] = desc[count-1-i]
	}

	return asc, nil
}

// CalculateUptimePercentage calculates the uptime percentage for an endpoint over rolling window
// MAINTENANCE checks are excluded from the total denominator
func (db *DB) CalculateUptimePercentage(endpointID string, window time.Duration) (float64, error) {
	since := time.Now().Add(-window)
	row := db.QueryRow(`
		SELECT 
			COUNT(CASE WHEN status = 'UP' THEN 1 END) as up_count,
			COUNT(CASE WHEN status IN ('UP', 'DOWN') THEN 1 END) as total_count
		FROM probe_history
		WHERE endpoint_id = ? AND checked_at >= ?`, endpointID, since)

	var upCount, totalCount int
	if err := row.Scan(&upCount, &totalCount); err != nil {
		return 100.0, err
	}

	if totalCount == 0 {
		return 100.0, nil
	}

	pct := (float64(upCount) / float64(totalCount)) * 100.0
	return pct, nil
}

// CalculateGlobalUptimePercentage calculates average uptime across all endpoints over rolling window
func (db *DB) CalculateGlobalUptimePercentage(window time.Duration) (int, error) {
	since := time.Now().Add(-window)
	row := db.QueryRow(`
		SELECT 
			COUNT(CASE WHEN status = 'UP' THEN 1 END) as up_count,
			COUNT(CASE WHEN status IN ('UP', 'DOWN') THEN 1 END) as total_count
		FROM probe_history
		WHERE checked_at >= ?`, since)

	var upCount, totalCount int
	if err := row.Scan(&upCount, &totalCount); err != nil {
		return 100, err
	}

	if totalCount == 0 {
		return 100, nil
	}

	pct := int((float64(upCount) / float64(totalCount)) * 100.0)
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}
	return pct, nil
}

// PruneHistory deletes probe history older than retentionDays
func (db *DB) PruneHistory(retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	res, err := db.Exec(`DELETE FROM probe_history WHERE checked_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
