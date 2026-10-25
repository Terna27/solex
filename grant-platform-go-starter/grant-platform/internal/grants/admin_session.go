package grants

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type AdminSession struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s AdminSession) IsExpired(now time.Time) bool {
	return now.After(s.ExpiresAt)
}

// VerifyAdminCredentials checks credentials strictly against the configured admin email and bcrypt password hash.
func VerifyAdminCredentials(email, password, configuredEmail, configuredHash string) bool {
	if configuredEmail == "" {
		configuredEmail = "admin@solex.ng"
	}
	normEntered, err1 := NormalizeEmail(email)
	normConfig, err2 := NormalizeEmail(configuredEmail)
	if err1 != nil || err2 != nil || normEntered != normConfig {
		return false
	}
	if password == "" || configuredHash == "" {
		return false
	}

	// 1. Bcrypt hash check (starts with $2a$, $2b$, or $2y$)
	if strings.HasPrefix(configuredHash, "$2a$") || strings.HasPrefix(configuredHash, "$2b$") || strings.HasPrefix(configuredHash, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(configuredHash), []byte(password)) == nil
	}

	// 2. SHA-256 hex string check (64 hex characters) if configured
	if len(configuredHash) == 64 {
		sum := sha256.Sum256([]byte(password))
		gotHex := hex.EncodeToString(sum[:])
		if subtle.ConstantTimeCompare([]byte(strings.ToLower(gotHex)), []byte(strings.ToLower(configuredHash))) == 1 {
			return true
		}
	}

	// Strict: No default password fallback, no legacy token login fallback
	return false
}
