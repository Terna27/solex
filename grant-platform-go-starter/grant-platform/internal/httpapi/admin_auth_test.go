package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"grantplatform/internal/grants"
)

func TestAdminAuth_PublicHomepageHasNoAdminLinks(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	h := New(s, strings.Repeat("a", 40))

	// Fetch index.html
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "Admin Workspace") || strings.Contains(body, "Admin Login") {
		t.Errorf("public homepage html contains admin workspace / login text")
	}

	// Fetch app.js
	req = httptest.NewRequest("GET", "/app.js", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /app.js, got %d", w.Code)
	}
	jsBody := w.Body.String()
	// Check that public navigation does not push admin into public nav
	if strings.Contains(jsBody, "items.push({ id: 'admin'") || strings.Contains(jsBody, "items.push({ id: \"admin\"") {
		t.Errorf("public nav items still contains admin push")
	}
}

func TestAdminAuth_SPARoutesServeIndexHTML(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	h := New(s, strings.Repeat("a", 40))

	for _, route := range []string{"/admin/login", "/admin/dashboard", "/admin", "/login", "/register", "/dashboard"} {
		req := httptest.NewRequest("GET", route, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("route %s expected 200, got %d", route, w.Code)
		}
		if !strings.Contains(w.Body.String(), "<!doctype html>") {
			t.Errorf("route %s expected index.html content", route)
		}
	}
}

func TestAdminAuth_LoginWithBcryptPassword(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	adminEmail := "director@solex.ng"
	adminPass := "SuperSecretAdmin2026!"
	hash, err := grants.HashPassword(adminPass)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	t.Setenv("ADMIN_EMAIL", adminEmail)
	t.Setenv("ADMIN_PASSWORD_HASH", hash)

	h := New(s, strings.Repeat("a", 40))

	// 1. Invalid credentials
	badPayload, _ := json.Marshal(map[string]string{
		"email":    adminEmail,
		"password": "wrong-password",
	})
	req := httptest.NewRequest("POST", "/api/v1/admin/auth/login", bytes.NewReader(badPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password, got %d", w.Code)
	}

	// 2. Valid credentials
	goodPayload, _ := json.Marshal(map[string]string{
		"email":    adminEmail,
		"password": adminPass,
	})
	req = httptest.NewRequest("POST", "/api/v1/admin/auth/login", bytes.NewReader(goodPayload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid admin login, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response body
	var res struct {
		Status       string            `json:"status"`
		SessionToken string            `json:"session_token"`
		Admin        map[string]string `json:"admin"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse login response: %v", err)
	}
	if res.Status != "ok" || res.SessionToken == "" || res.Admin["email"] != adminEmail {
		t.Fatalf("unexpected login response: %+v", res)
	}

	// Verify HttpOnly cookie
	cookies := w.Result().Cookies()
	var adminCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "solex_admin_session" {
			adminCookie = c
			break
		}
	}
	if adminCookie == nil {
		t.Fatalf("solex_admin_session cookie was not set")
	}
	if !adminCookie.HttpOnly {
		t.Errorf("expected cookie HttpOnly to be true")
	}
	if adminCookie.Value != res.SessionToken {
		t.Errorf("cookie value mismatch: got %s, want %s", adminCookie.Value, res.SessionToken)
	}

	// Verify GET /api/v1/admin/auth/me works with cookie
	meReq := httptest.NewRequest("GET", "/api/v1/admin/auth/me", nil)
	meReq.AddCookie(adminCookie)
	meW := httptest.NewRecorder()
	h.ServeHTTP(meW, meReq)
	if meW.Code != http.StatusOK {
		t.Fatalf("expected 200 on /admin/auth/me, got %d: %s", meW.Code, meW.Body.String())
	}

	// Verify GET /api/v1/admin/metrics works with cookie
	metricsReq := httptest.NewRequest("GET", "/api/v1/admin/metrics", nil)
	metricsReq.AddCookie(adminCookie)
	metricsW := httptest.NewRecorder()
	h.ServeHTTP(metricsW, metricsReq)
	if metricsW.Code != http.StatusOK {
		t.Fatalf("expected 200 on /admin/metrics with admin session cookie, got %d: %s", metricsW.Code, metricsW.Body.String())
	}

	// Verify admin logout
	logoutReq := httptest.NewRequest("POST", "/api/v1/admin/auth/logout", nil)
	logoutReq.AddCookie(adminCookie)
	logoutW := httptest.NewRecorder()
	h.ServeHTTP(logoutW, logoutReq)
	if logoutW.Code != http.StatusOK {
		t.Fatalf("expected 200 on admin logout, got %d", logoutW.Code)
	}

	// Check cookie was cleared
	clearedCookies := logoutW.Result().Cookies()
	var clearedCookie *http.Cookie
	for _, c := range clearedCookies {
		if c.Name == "solex_admin_session" {
			clearedCookie = c
			break
		}
	}
	if clearedCookie == nil || clearedCookie.MaxAge != -1 {
		t.Errorf("expected cookie to be cleared with MaxAge -1")
	}

	// Verify previous session token is now rejected
	metricsReq2 := httptest.NewRequest("GET", "/api/v1/admin/metrics", nil)
	metricsReq2.AddCookie(adminCookie)
	metricsW2 := httptest.NewRecorder()
	h.ServeHTTP(metricsW2, metricsReq2)
	if metricsW2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on /admin/metrics after logout, got %d", metricsW2.Code)
	}
}

func TestAdminAuth_LoginWithSha256Hash(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	adminEmail := "operations@solex.ng"
	adminPass := "Sha256StrongPass2026!"
	sum := sha256.Sum256([]byte(adminPass))
	shaHex := hex.EncodeToString(sum[:])

	t.Setenv("ADMIN_EMAIL", adminEmail)
	t.Setenv("ADMIN_PASSWORD_HASH", shaHex)

	h := New(s, strings.Repeat("b", 40))

	goodPayload, _ := json.Marshal(map[string]string{
		"email":    adminEmail,
		"password": adminPass,
	})
	req := httptest.NewRequest("POST", "/api/v1/admin/auth/login", bytes.NewReader(goodPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for sha256 admin login, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminAuth_RateLimiting(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	h := New(s, strings.Repeat("c", 40))

	body, _ := json.Marshal(map[string]string{
		"email":    "admin@solex.ng",
		"password": "wrong-password",
	})

	// Make 6 consecutive attempts from same IP (limiter capacity is 5)
	var lastCode int
	for i := 0; i < 6; i++ {
		req := httptest.NewRequest("POST", "/api/v1/admin/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.10.50:1234"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		lastCode = w.Code
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests after 6 attempts, got %d", lastCode)
	}
}

func TestAdminAuth_ApplicantCannotAccessAdminEndpoints(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	h := New(s, strings.Repeat("d", 40))

	// Register an applicant user
	user, session, err := s.Register(grants.RegisterInput{
		FullName:        "Amina Ibrahim",
		Email:           "amina@example.ng",
		Phone:           "+2348011223344",
		Password:        "ApplicantPass123!",
		ConfirmPassword: "ApplicantPass123!",
	})
	if err != nil {
		t.Fatalf("failed to register applicant: %v", err)
	}

	// 1. Unauthenticated request to /admin/metrics
	req := httptest.NewRequest("GET", "/api/v1/admin/metrics", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated admin access expected 401, got %d", w.Code)
	}

	// 2. Request with applicant cookie `solex_session`
	req = httptest.NewRequest("GET", "/api/v1/admin/metrics", nil)
	req.AddCookie(&http.Cookie{
		Name:  "solex_session",
		Value: session.ID,
	})
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("applicant session cookie accessing admin expected 401, got %d", w.Code)
	}

	// 3. Request with applicant token passed as Bearer token
	req = httptest.NewRequest("GET", "/api/v1/admin/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+session.ID)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("applicant bearer token accessing admin expected 401, got %d", w.Code)
	}

	// 4. Request with applicant token passed as `solex_admin_session` cookie
	req = httptest.NewRequest("GET", "/api/v1/admin/metrics", nil)
	req.AddCookie(&http.Cookie{
		Name:  "solex_admin_session",
		Value: session.ID,
	})
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("applicant token disguised as admin cookie expected 401, got %d", w.Code)
	}

	// 5. Admin cannot access applicant protected endpoints with admin token
	adminSess, err := s.CreateAdminSession("admin@solex.ng")
	if err != nil {
		t.Fatalf("failed to create admin session: %v", err)
	}

	appReq := httptest.NewRequest("GET", "/api/v1/profile", nil)
	appReq.AddCookie(&http.Cookie{
		Name:  "solex_session",
		Value: adminSess.ID,
	})
	appW := httptest.NewRecorder()
	h.ServeHTTP(appW, appReq)
	if appW.Code != http.StatusUnauthorized {
		t.Fatalf("admin session accessing applicant profile expected 401, got %d", appW.Code)
	}

	_ = user
}

func TestAdminAuth_LegacyAdminTokenCompatibility(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	legacyToken := strings.Repeat("z", 40)
	h := New(s, legacyToken)

	// Legacy Authorization: Bearer <token>
	req := httptest.NewRequest("GET", "/api/v1/admin/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+legacyToken)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with legacy admin token, got %d: %s", w.Code, w.Body.String())
	}
}
