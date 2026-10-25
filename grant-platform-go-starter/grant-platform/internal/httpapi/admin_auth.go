package httpapi

import (
	"crypto/subtle"
	"net/http"
	"os"
	"time"

	"grantplatform/internal/grants"
)

func (s *Server) getAdminEmail() string {
	email := os.Getenv("ADMIN_EMAIL")
	if email == "" {
		return "admin@solex.ng"
	}
	return email
}

func (s *Server) getAdminPasswordHash() string {
	return os.Getenv("ADMIN_PASSWORD_HASH")
}

func (s *Server) adminLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.adminLimiter.Allow(ip) {
		respond(w, http.StatusTooManyRequests, map[string]string{
			"error": "Too many administrator login attempts. Please wait a minute and try again.",
		})
		return
	}

	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}

	adminEmail := s.getAdminEmail()
	adminHash := s.getAdminPasswordHash()

	if !grants.VerifyAdminCredentials(in.Email, in.Password, adminEmail, adminHash, s.adminToken) {
		respond(w, http.StatusUnauthorized, map[string]string{
			"error": "Invalid administrator email or password.",
		})
		return
	}

	session, err := s.service.CreateAdminSession(adminEmail)
	if err != nil {
		fail(w, err)
		return
	}

	s.setAdminSessionCookie(w, r, session.ID)
	respond(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"session_token": session.ID,
		"admin": map[string]string{
			"email": adminEmail,
		},
		"message": "Administrator login successful.",
	})
}

func (s *Server) adminLogout(w http.ResponseWriter, r *http.Request) {
	token := s.extractAdminSessionToken(r)
	if token != "" {
		_ = s.service.DeleteAdminSession(token)
	}

	s.clearAdminSessionCookie(w, r)
	respond(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (s *Server) adminMe(w http.ResponseWriter, r *http.Request) {
	token := s.extractAdminSessionToken(r)
	if token == "" {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorised"})
		return
	}

	// Legacy token compatibility
	legacyMatch := subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken)) == 1
	if legacyMatch && s.adminToken != "" {
		respond(w, http.StatusOK, map[string]any{
			"authenticated": true,
			"email":         s.getAdminEmail(),
		})
		return
	}

	sess, err := s.service.AuthenticateAdminSession(token)
	if err != nil {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorised"})
		return
	}

	respond(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"email":         sess.Email,
	})
}

func (s *Server) setAdminSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	secure := os.Getenv("COOKIE_SECURE") == "true" || r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	cookie := &http.Cookie{
		Name:     "solex_admin_session",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(12 * time.Hour),
		MaxAge:   43200,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	}
	http.SetCookie(w, cookie)
}

func (s *Server) clearAdminSessionCookie(w http.ResponseWriter, r *http.Request) {
	secure := os.Getenv("COOKIE_SECURE") == "true" || r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	cookie := &http.Cookie{
		Name:     "solex_admin_session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	}
	http.SetCookie(w, cookie)
}

func (s *Server) extractAdminSessionToken(r *http.Request) string {
	// 1. HttpOnly admin cookie
	cookie, err := r.Cookie("solex_admin_session")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// 2. Authorization Bearer header
	bearerToken := bearer(r)
	if bearerToken != "" {
		return bearerToken
	}

	// 3. Query param token
	qToken := r.URL.Query().Get("token")
	if qToken != "" {
		return qToken
	}

	return ""
}
