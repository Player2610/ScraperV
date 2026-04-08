package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"time"
)

// CreateSession inserts a new operator session into operator_sessions and
// returns the 64-hex-char token (32 random bytes).
func CreateSession(db *sql.DB, operatorID int64) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("auth: generate session token: %w", err)
	}
	token := hex.EncodeToString(raw) // 64 chars, matches CHAR(64)

	const q = `
		INSERT INTO operator_sessions (token, operator_id, expires_at)
		VALUES ($1, $2, NOW() + INTERVAL '8 hours')
	`
	if _, err := db.Exec(q, token, operatorID); err != nil {
		return "", fmt.Errorf("auth: create session: %w", err)
	}
	return token, nil
}

// ValidateSession looks up a non-expired session and returns the operator_id.
// Returns sql.ErrNoRows (wrapped) if the session is missing or expired.
func ValidateSession(db *sql.DB, token string) (int64, error) {
	const q = `
		SELECT operator_id
		FROM operator_sessions
		WHERE token = $1 AND expires_at > NOW()
	`
	var operatorID int64
	err := db.QueryRow(q, token).Scan(&operatorID)
	if err != nil {
		return 0, fmt.Errorf("auth: validate session: %w", err)
	}
	return operatorID, nil
}

// CleanupExpiredSessions deletes expired operator sessions. Intended to run
// on a timer goroutine started from main.go.
func CleanupExpiredSessions(db *sql.DB) {
	const q = `DELETE FROM operator_sessions WHERE expires_at < NOW()`
	res, err := db.Exec(q)
	if err != nil {
		log.Printf("auth: cleanup expired sessions: %v", err)
		return
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		log.Printf("auth: deleted %d expired operator sessions", n)
	}
}

// StartSessionCleanup launches a background goroutine that cleans up expired
// operator sessions every hour. Call from main.go after DB is ready.
func StartSessionCleanup(db *sql.DB) {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			CleanupExpiredSessions(db)
		}
	}()
}
