package storage

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// APIToken represents a registered credential for Homepage widget API access
type APIToken struct {
	TokenID    string
	Name       string
	SecretHash string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	Revoked    bool
}

// GenerateToken creates a new token_id and high-entropy secret, storing only the bcrypt hash
func (db *DB) GenerateToken(name string) (tokenID, plaintextSecret string, err error) {
	if name == "" {
		name = "Homepage Widget"
	}

	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return "", "", fmt.Errorf("crypto rand failed: %w", err)
	}
	tokenID = "tok_" + hex.EncodeToString(idBytes)

	secBytes := make([]byte, 24)
	if _, err := rand.Read(secBytes); err != nil {
		return "", "", fmt.Errorf("crypto rand failed: %w", err)
	}
	plaintextSecret = "sec_" + hex.EncodeToString(secBytes)

	hashed, err := bcrypt.GenerateFromPassword([]byte(plaintextSecret), bcrypt.DefaultCost)
	if err != nil {
		return "", "", fmt.Errorf("failed to hash secret: %w", err)
	}

	now := time.Now()
	_, err = db.Exec(`
		INSERT INTO api_tokens (token_id, name, secret_hash, created_at, revoked)
		VALUES (?, ?, ?, ?, 0)`,
		tokenID, name, string(hashed), now)
	if err != nil {
		return "", "", fmt.Errorf("failed to store api token: %w", err)
	}

	return tokenID, plaintextSecret, nil
}

// VerifyToken validates token credentials and updates last_used_at on success
func (db *DB) VerifyToken(tokenID, tokenSecret string) bool {
	var secretHash string
	var revoked int
	err := db.QueryRow(`
		SELECT secret_hash, revoked FROM api_tokens
		WHERE token_id = ?`, tokenID).Scan(&secretHash, &revoked)

	if err != nil || revoked == 1 {
		return false
	}

	if err := bcrypt.CompareHashAndPassword([]byte(secretHash), []byte(tokenSecret)); err != nil {
		return false
	}

	now := time.Now()
	_, _ = db.Exec(`UPDATE api_tokens SET last_used_at = ? WHERE token_id = ?`, now, tokenID)
	return true
}

// ListTokens returns all active and revoked tokens
func (db *DB) ListTokens() ([]APIToken, error) {
	rows, err := db.Query(`
		SELECT token_id, name, secret_hash, created_at, last_used_at, revoked
		FROM api_tokens
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []APIToken
	for rows.Next() {
		var t APIToken
		var lastUsed sql.NullTime
		var revokedInt int
		if err := rows.Scan(&t.TokenID, &t.Name, &t.SecretHash, &t.CreatedAt, &lastUsed, &revokedInt); err != nil {
			return nil, err
		}
		t.Revoked = (revokedInt == 1)
		if lastUsed.Valid {
			t.LastUsedAt = &lastUsed.Time
		}
		tokens = append(tokens, t)
	}

	return tokens, nil
}

// RevokeToken marks a token as invalid immediately
func (db *DB) RevokeToken(tokenID string) error {
	res, err := db.Exec(`UPDATE api_tokens SET revoked = 1 WHERE token_id = ?`, tokenID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("token %s not found", tokenID)
	}
	return nil
}
