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

// VerifyAdminCredentials checks applicant credentials against the configured admin email and password/hash.
func VerifyAdminCredentials(email, password, configuredEmail, configuredHash, adminToken string) bool {
	if configuredEmail == "" {
		configuredEmail = "admin@solex.ng"
	}
	normEntered, err1 := NormalizeEmail(email)
	normConfig, err2 := NormalizeEmail(configuredEmail)
	if err1 != nil || err2 != nil || normEntered != normConfig {
		return false
	}
	if password == "" {
		return false
	}

	if configuredHash != "" {
		// 1. Bcrypt hash check (starts with $2a$, $2b$, or $2y$)
		if strings.HasPrefix(configuredHash, "$2a$") || strings.HasPrefix(configuredHash, "$2b$") || strings.HasPrefix(configuredHash, "$2y$") {
			return bcrypt.CompareHashAndPassword([]byte(configuredHash), []byte(password)) == nil
		}
		// 2. SHA-256 hex string check (64 hex characters)
		if len(configuredHash) == 64 {
			sum := sha256.Sum256([]byte(password))
			gotHex := hex.EncodeToString(sum[:])
			if subtle.ConstantTimeCompare([]byte(strings.ToLower(gotHex)), []byte(strings.ToLower(configuredHash))) == 1 {
				return true
			}
		}
		// 3. Fallback constant-time string comparison
		return subtle.ConstantTimeCompare([]byte(password), []byte(configuredHash)) == 1
	}

	// If configuredHash is empty, check against adminToken or default dev prototype credentials
	if adminToken != "" && subtle.ConstantTimeCompare([]byte(password), []byte(adminToken)) == 1 {
		return true
	}
	if subtle.ConstantTimeCompare([]byte(password), []byte("AdminP@ssw0rd2026!")) == 1 {
		return true
	}
	return false
}
