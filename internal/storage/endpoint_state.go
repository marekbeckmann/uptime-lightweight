package storage

import (
	"database/sql"
	"time"
)

// EndpointState represents the persisted state in endpoint_state
type EndpointState struct {
	EndpointID             string
	Status                 string // UP, DOWN, MAINTENANCE, PENDING
	ConsecutiveFailures    int
	LastCheckAt            *time.Time
	LastUpAt               *time.Time
	DownSince              *time.Time
	LastLatencyMS          int
	LastError              string
	TLSExpiryAt            *time.Time
	LastTLSWarnThreshold   int
}

// GetState fetches state for an endpoint, returning a default PENDING state if not yet recorded
func (db *DB) GetState(endpointID string) (*EndpointState, error) {
	row := db.QueryRow(`
		SELECT endpoint_id, status, consecutive_failures, last_check_at, last_up_at, down_since,
		       last_latency_ms, last_error, tls_expiry_at, last_tls_warn_threshold
		FROM endpoint_state WHERE endpoint_id = ?`, endpointID)

	var s EndpointState
	var lastCheck, lastUp, downSince, tlsExpiry sql.NullTime
	err := row.Scan(&s.EndpointID, &s.Status, &s.ConsecutiveFailures, &lastCheck, &lastUp, &downSince,
		&s.LastLatencyMS, &s.LastError, &tlsExpiry, &s.LastTLSWarnThreshold)

	if err == sql.ErrNoRows {
		return &EndpointState{
			EndpointID: endpointID,
			Status:     "PENDING",
		}, nil
	}
	if err != nil {
		return nil, err
	}

	if lastCheck.Valid {
		s.LastCheckAt = &lastCheck.Time
	}
	if lastUp.Valid {
		s.LastUpAt = &lastUp.Time
	}
	if downSince.Valid {
		s.DownSince = &downSince.Time
	}
	if tlsExpiry.Valid {
		s.TLSExpiryAt = &tlsExpiry.Time
	}

	return &s, nil
}

// GetAllStates returns a map of all persisted endpoint states
func (db *DB) GetAllStates() (map[string]EndpointState, error) {
	rows, err := db.Query(`
		SELECT endpoint_id, status, consecutive_failures, last_check_at, last_up_at, down_since,
		       last_latency_ms, last_error, tls_expiry_at, last_tls_warn_threshold
		FROM endpoint_state`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make(map[string]EndpointState)
	for rows.Next() {
		var s EndpointState
		var lastCheck, lastUp, downSince, tlsExpiry sql.NullTime
		if err := rows.Scan(&s.EndpointID, &s.Status, &s.ConsecutiveFailures, &lastCheck, &lastUp, &downSince,
			&s.LastLatencyMS, &s.LastError, &tlsExpiry, &s.LastTLSWarnThreshold); err != nil {
			return nil, err
		}
		if lastCheck.Valid {
			s.LastCheckAt = &lastCheck.Time
		}
		if lastUp.Valid {
			s.LastUpAt = &lastUp.Time
		}
		if downSince.Valid {
			s.DownSince = &downSince.Time
		}
		if tlsExpiry.Valid {
			s.TLSExpiryAt = &tlsExpiry.Time
		}
		states[s.EndpointID] = s
	}

	return states, nil
}

// SaveState inserts or updates an endpoint state
func (db *DB) SaveState(s *EndpointState) error {
	query := `
		INSERT INTO endpoint_state (
			endpoint_id, status, consecutive_failures, last_check_at, last_up_at, down_since,
			last_latency_ms, last_error, tls_expiry_at, last_tls_warn_threshold
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(endpoint_id) DO UPDATE SET
			status = excluded.status,
			consecutive_failures = excluded.consecutive_failures,
			last_check_at = excluded.last_check_at,
			last_up_at = excluded.last_up_at,
			down_since = excluded.down_since,
			last_latency_ms = excluded.last_latency_ms,
			last_error = excluded.last_error,
			tls_expiry_at = excluded.tls_expiry_at,
			last_tls_warn_threshold = excluded.last_tls_warn_threshold
	`
	_, err := db.Exec(query,
		s.EndpointID, s.Status, s.ConsecutiveFailures, s.LastCheckAt, s.LastUpAt, s.DownSince,
		s.LastLatencyMS, s.LastError, s.TLSExpiryAt, s.LastTLSWarnThreshold,
	)
	return err
}
