package httpapi

import (
	"context"
	"net/http"
	"os"
	"time"

	"grantplatform/internal/grants"
)

type contextKey string

const userContextKey contextKey = "solex_user"

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var in grants.RegisterInput
	if !decode(w, r, &in) {
		return
	}

	user, session, err := s.service.Register(in)
	if err != nil {
		fail(w, err)
		return
	}

	s.setSessionCookie(w, session.ID)
	respond(w, http.StatusCreated, map[string]any{
		"user":          user.Sanitized(),
		"session_token": session.ID,
		"message":       "Account created successfully.",
	})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.loginLimiter.Allow(ip) {
		respond(w, http.StatusTooManyRequests, map[string]string{"error": "Too many login attempts. Please wait a minute and try again."})
		return
	}

	var in grants.LoginInput
	if !decode(w, r, &in) {
		return
	}

	user, session, err := s.service.Login(in)
	if err != nil {
		fail(w, err)
		return
	}

	s.setSessionCookie(w, session.ID)
	respond(w, http.StatusOK, map[string]any{
		"user":          user.Sanitized(),
		"session_token": session.ID,
		"message":       "Login successful.",
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	token := s.extractSessionToken(r)
	if token != "" {
		_ = s.service.Logout(token)
	}

	s.clearSessionCookie(w)
	respond(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	if user == nil {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorised"})
		return
	}

	profile, _ := s.service.GetProfile(user.ID)
	app, _ := s.service.GetCurrentApplication(user.ID)

	hasProfile := profile.NIN != "" && profile.BusinessName != ""
	hasApp := app.ID != ""

	respond(w, http.StatusOK, map[string]any{
		"user":                user.Sanitized(),
		"profile_completed":   hasProfile,
		"has_application":     hasApp,
		"application_status":  app.Status,
		"application_summary": app,
	})
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	if user == nil {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorised"})
		return
	}

	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if !decode(w, r, &in) {
		return
	}

	if err := s.service.ChangePassword(user.ID, in.CurrentPassword, in.NewPassword, in.ConfirmPassword); err != nil {
		fail(w, err)
		return
	}

	respond(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "Password updated successfully.",
	})
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	secure := os.Getenv("COOKIE_SECURE") == "true"
	cookie := &http.Cookie{
		Name:     "solex_session",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		MaxAge:   86400,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	}
	http.SetCookie(w, cookie)
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	secure := os.Getenv("COOKIE_SECURE") == "true"
	cookie := &http.Cookie{
		Name:     "solex_session",
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

func (s *Server) extractSessionToken(r *http.Request) string {
	// First check Authorization header
	bearerToken := bearer(r)
	if bearerToken != "" {
		return bearerToken
	}

	// Next check HttpOnly cookie
	cookie, err := r.Cookie("solex_session")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	return ""
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := s.extractSessionToken(r)
		if token == "" {
			respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorised"})
			return
		}

		user, err := s.service.AuthenticateSession(token)
		if err != nil {
			respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorised"})
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, &user)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func (s *Server) currentUser(r *http.Request) *grants.User {
	val := r.Context().Value(userContextKey)
	if val == nil {
		token := s.extractSessionToken(r)
		if token != "" {
			u, err := s.service.AuthenticateSession(token)
			if err == nil {
				return &u
			}
		}
		return nil
	}
	u, ok := val.(*grants.User)
	if ok {
		return u
	}
	return nil
}
