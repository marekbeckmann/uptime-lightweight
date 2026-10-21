package api

import (
	"encoding/base64"
	"net/http"
	"strings"
)

// AuthenticateToken verifies the request against registered API tokens
func (s *Server) AuthenticateToken(r *http.Request) bool {
	// 1. Check custom headers: X-API-Token-Id and X-API-Token-Secret
	tokenID := r.Header.Get("X-API-Token-Id")
	tokenSecret := r.Header.Get("X-API-Token-Secret")
	if tokenID != "" && tokenSecret != "" {
		return s.DB.VerifyToken(tokenID, tokenSecret)
	}

	// 2. Check Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		// Bearer <token_id>:<token_secret>
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenPart := strings.TrimPrefix(authHeader, "Bearer ")
			parts := strings.SplitN(tokenPart, ":", 2)
			if len(parts) == 2 {
				return s.DB.VerifyToken(parts[0], parts[1])
			}
		}

		// Basic Auth (username = token_id, password = token_secret)
		if strings.HasPrefix(authHeader, "Basic ") {
			b64Part := strings.TrimPrefix(authHeader, "Basic ")
			decoded, err := base64.StdEncoding.DecodeString(b64Part)
			if err == nil {
				parts := strings.SplitN(string(decoded), ":", 2)
				if len(parts) == 2 {
					return s.DB.VerifyToken(parts[0], parts[1])
				}
			}
		}
	}

	return false
}
