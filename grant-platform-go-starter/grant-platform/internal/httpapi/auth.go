package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const sessionCookie = "solex_admin_session"
const sessionTTL = 8 * time.Hour

type adminAuth struct {
	email        string
	salt         []byte
	hash         []byte
	iterations   int
	mu           sync.Mutex
	sessions     map[[32]byte]time.Time
	failed       int
	blockedUntil time.Time
}

func derive(password string, salt []byte, iterations int) []byte {
	// PBKDF2-HMAC-SHA256, 32-byte output, one block.
	var block [4]byte
	block[3] = 1
	mac := hmac.New(sha256.New, []byte(password))
	mac.Write(salt)
	mac.Write(block[:])
	u := mac.Sum(nil)
	result := append([]byte(nil), u...)
	for i := 1; i < iterations; i++ {
		mac = hmac.New(sha256.New, []byte(password))
		mac.Write(u)
		u = mac.Sum(nil)
		for j := range result {
			result[j] ^= u[j]
		}
	}
	return result
}
func newAdminAuth(email, encoded string) (*adminAuth, error) {
	parts := strings.Split(encoded, "$")
	if strings.TrimSpace(email) == "" || len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return nil, errors.New("set ADMIN_EMAIL and ADMIN_PASSWORD_HASH (pbkdf2_sha256$iterations$salt$hash)")
	}
	var iter int
	if _, err := fmtSscanf(parts[1], &iter); err != nil || iter < 100000 || iter > 1000000 {
		return nil, errors.New("invalid admin password hash iterations")
	}
	salt, err := hex.DecodeString(parts[2])
	if err != nil || len(salt) < 16 {
		return nil, errors.New("invalid admin password salt")
	}
	hash, err := hex.DecodeString(parts[3])
	if err != nil || len(hash) != 32 {
		return nil, errors.New("invalid admin password hash")
	}
	return &adminAuth{email: strings.ToLower(strings.TrimSpace(email)), salt: salt, hash: hash, iterations: iter, sessions: make(map[[32]byte]time.Time)}, nil
}
func fmtSscanf(s string, n *int) (int, error) { return fmt.Sscanf(s, "%d", n) }
func (a *adminAuth) authorized(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil || len(raw) != 32 {
		return false
	}
	key := sha256.Sum256(raw)
	a.mu.Lock()
	defer a.mu.Unlock()
	expiry, ok := a.sessions[key]
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		delete(a.sessions, key)
		return false
	}
	return true
}
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" { // Fetch Metadata for browsers; non-browser callers can omit it.
		return r.Header.Get("Sec-Fetch-Site") != "cross-site"
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && strings.EqualFold(u.Host, r.Host) && (r.TLS == nil || u.Scheme == "https")
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		http.NotFound(w, r)
		return
	}
	if !sameOrigin(r) {
		respond(w, 403, map[string]string{"error": "invalid request origin"})
		return
	}
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	a := s.auth
	a.mu.Lock()
	if time.Now().Before(a.blockedUntil) {
		a.mu.Unlock()
		respond(w, 429, map[string]string{"error": "too many attempts; try again later"})
		return
	}
	a.mu.Unlock()
	derived := derive(in.Password, a.salt, a.iterations)
	ok := subtle.ConstantTimeCompare(derived, a.hash) == 1 && strings.ToLower(strings.TrimSpace(in.Email)) == a.email
	if !ok {
		a.mu.Lock()
		a.failed++
		if a.failed >= 5 {
			a.blockedUntil = time.Now().Add(5 * time.Minute)
			a.failed = 0
		}
		a.mu.Unlock()
		respond(w, 401, map[string]string{"error": "incorrect email or password"})
		return
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		respond(w, 500, map[string]string{"error": "could not start session"})
		return
	}
	key := sha256.Sum256(raw[:])
	a.mu.Lock()
	a.failed = 0
	a.blockedUntil = time.Time{}
	for k, expiry := range a.sessions {
		if time.Now().After(expiry) {
			delete(a.sessions, k)
		}
	}
	a.sessions[key] = time.Now().Add(sessionTTL)
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: base64.RawURLEncoding.EncodeToString(raw[:]), Path: "/api/v1/admin", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: int(sessionTTL.Seconds())})
	respond(w, 200, map[string]bool{"authenticated": true})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		http.NotFound(w, r)
		return
	}
	if !sameOrigin(r) {
		respond(w, 403, map[string]string{"error": "invalid request origin"})
		return
	}
	if c, err := r.Cookie(sessionCookie); err == nil {
		if raw, e := base64.RawURLEncoding.DecodeString(c.Value); e == nil {
			key := sha256.Sum256(raw)
			s.auth.mu.Lock()
			delete(s.auth.sessions, key)
			s.auth.mu.Unlock()
		}
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/api/v1/admin", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	respond(w, 200, map[string]bool{"signed_out": true})
}
