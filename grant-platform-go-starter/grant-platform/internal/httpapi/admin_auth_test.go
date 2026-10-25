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

func TestAdminApprovalWorkflow_CompleteAndConflictPrevention(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	adminEmail := "lead@solex.ng"
	adminPass := "AdminP@ss2026!"
	hash, _ := grants.HashPassword(adminPass)
	t.Setenv("ADMIN_EMAIL", adminEmail)
	t.Setenv("ADMIN_PASSWORD_HASH", hash)

	h := New(s, strings.Repeat("x", 40))

	// Admin login
	loginBody, _ := json.Marshal(map[string]string{
		"email":    adminEmail,
		"password": adminPass,
	})
	w := request(h, "POST", "/api/v1/admin/auth/login", "", string(loginBody))
	if w.Code != http.StatusOK {
		t.Fatalf("admin login failed: %d %s", w.Code, w.Body.String())
	}
	var loginRes struct {
		SessionToken string `json:"session_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &loginRes)
	adminToken := loginRes.SessionToken

	// Register applicant
	regBody := `{"full_name":"Ngozi Okonjo","email":"ngozi@example.ng","phone":"+2348033331111","password":"Password123#","confirm_password":"Password123#"}`
	w = request(h, "POST", "/api/v1/auth/register", "", regBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("register failed: %d %s", w.Code, w.Body.String())
	}
	var regRes struct {
		SessionToken string `json:"session_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &regRes)
	appToken := regRes.SessionToken

	// Save profile
	prof := `{
		"full_name": "Ngozi Okonjo",
		"date_of_birth": "1990-01-01",
		"nin": "12345678901",
		"phone": "+2348033331111",
		"email": "ngozi@example.ng",
		"state_of_origin": "Abia",
		"lga": "Umuahia",
		"residential_address": "10 Bank Road, Umuahia",
		"business_category": "unregistered",
		"business_name": "Ngozi Ventures",
		"business_type": "Retail",
		"business_address": "Market Square",
		"state_of_operation": "Abia",
		"lga_of_operation": "Umuahia",
		"operating_history": "2 years",
		"grant_purpose": "working_capital",
		"bank_name": "Zenith Bank",
		"account_number": "0123456789"
	}`
	w = request(h, "PUT", "/api/v1/profile", appToken, prof)
	if w.Code != http.StatusOK {
		t.Fatalf("save profile failed: %d %s", w.Code, w.Body.String())
	}

	// Start application
	w = request(h, "POST", "/api/v1/applications/start", appToken, `{"category":"unregistered"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("start application failed: %d %s", w.Code, w.Body.String())
	}
	var app grants.Application
	_ = json.Unmarshal(w.Body.Bytes(), &app)

	// Upload documents
	pngBytes := makeTestDocPNG()
	w = uploadTestDoc(h, appToken, app.ID, "passport_photo", pngBytes)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload passport failed: %d %s", w.Code, w.Body.String())
	}
	w = uploadTestDoc(h, appToken, app.ID, "government_id", pngBytes)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload ID failed: %d %s", w.Code, w.Body.String())
	}

	// Ready for payment
	w = request(h, "POST", "/api/v1/applications/ready-for-payment", appToken, `{"application_id":"`+app.ID+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("ready for payment failed: %d %s", w.Code, w.Body.String())
	}

	// Payment init & verify
	w = request(h, "POST", "/api/v1/payments/init", appToken, `{"application_id":"`+app.ID+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("payment init failed: %d %s", w.Code, w.Body.String())
	}
	var payRes struct {
		Payment grants.Payment `json:"payment"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &payRes)

	w = request(h, "POST", "/api/v1/payments/verify", appToken, `{"reference":"`+payRes.Payment.Reference+`","simulator_action":"simulate_success"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("payment verify failed: %d %s", w.Code, w.Body.String())
	}

	// Submit application
	w = request(h, "POST", "/api/v1/applications/submit", appToken, `{"application_id":"`+app.ID+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("submit application failed: %d %s", w.Code, w.Body.String())
	}

	// Check metrics before approval
	w = request(h, "GET", "/api/v1/admin/metrics", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("admin metrics failed: %d %s", w.Code, w.Body.String())
	}
	var mPre grants.AdminMetrics
	_ = json.Unmarshal(w.Body.Bytes(), &mPre)
	if mPre.ApprovedApplications != 0 || mPre.SubmittedApplications != 1 {
		t.Fatalf("pre-approval metrics unexpected: %+v", mPre)
	}

	// 1. Approval fails before checks are complete
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, `{"disbursement_batch":"Batch A","disbursement_date":"2026-12-10"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("approval before checks complete should return 400, got %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "all verification checks must be complete prior to approval") {
		t.Fatalf("expected meaningful checks incomplete error, got %s", w.Body.String())
	}

	// 2. Complete checklist
	checksPayload := `{
		"phone": true,
		"identity_and_age": true,
		"address": true,
		"business": true,
		"bank_account_ownership": true
	}`
	w = request(h, "PUT", "/api/v1/admin/applications/"+app.ID+"/checks", adminToken, checksPayload)
	if w.Code != http.StatusOK {
		t.Fatalf("update checks failed: %d %s", w.Code, w.Body.String())
	}

	// 3. Approval succeeds
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, `{"disbursement_batch":"Batch A","disbursement_date":"2026-12-10"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("approval failed: %d %s", w.Code, w.Body.String())
	}
	var appvRes struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &appvRes)
	if appvRes.Status != "approved" {
		t.Fatalf("expected status approved, got %s", appvRes.Status)
	}

	// 4. Metrics update: approved count = 1, grant commitments = 25,000,000 kobo
	w = request(h, "GET", "/api/v1/admin/metrics", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("metrics failed: %d %s", w.Code, w.Body.String())
	}
	var mPost grants.AdminMetrics
	_ = json.Unmarshal(w.Body.Bytes(), &mPost)
	if mPost.ApprovedApplications != 1 {
		t.Errorf("expected 1 approved application, got %d", mPost.ApprovedApplications)
	}
	if mPost.ApprovedGrantCommitmentsKobo != 25000000 {
		t.Errorf("expected 25,000,000 kobo commitments, got %d", mPost.ApprovedGrantCommitmentsKobo)
	}

	// 5. Application details show approved status and scheduled date
	w = request(h, "GET", "/api/v1/admin/applications/"+app.ID, adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("admin get application failed: %d %s", w.Code, w.Body.String())
	}
	var appDetails grants.Application
	_ = json.Unmarshal(w.Body.Bytes(), &appDetails)
	if appDetails.Status != grants.StatusApproved {
		t.Errorf("expected status approved, got %s", appDetails.Status)
	}
	if appDetails.DisbursementDate != "2026-12-10" {
		t.Errorf("expected disbursement date 2026-12-10, got %s", appDetails.DisbursementDate)
	}
	if appDetails.DisbursementStatus != "scheduled" {
		t.Errorf("expected disbursement status scheduled, got %s", appDetails.DisbursementStatus)
	}

	// 6. Duplicate approval returns 409 Conflict (prevents double approval)
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, `{"disbursement_batch":"Batch A","disbursement_date":"2026-12-10"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate approval should return 409 Conflict, got %d %s", w.Code, w.Body.String())
	}

	// 7. Disbursement records successfully
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/disburse", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("disburse failed: %d %s", w.Code, w.Body.String())
	}

	// 8. Metrics reflect disbursement
	w = request(h, "GET", "/api/v1/admin/metrics", adminToken, "")
	_ = json.Unmarshal(w.Body.Bytes(), &mPost)
	if mPost.DisbursedApplications != 1 {
		t.Errorf("expected 1 disbursed application, got %d", mPost.DisbursedApplications)
	}
	if mPost.RecordedDisbursementsKobo != 25000000 {
		t.Errorf("expected 25,000,000 kobo disbursed, got %d", mPost.RecordedDisbursementsKobo)
	}
}

func TestAdminRejectionWorkflow_Complete(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	adminEmail := "lead@solex.ng"
	adminPass := "AdminP@ss2026!"
	hash, _ := grants.HashPassword(adminPass)
	t.Setenv("ADMIN_EMAIL", adminEmail)
	t.Setenv("ADMIN_PASSWORD_HASH", hash)

	h := New(s, strings.Repeat("y", 40))

	// Admin login
	loginBody, _ := json.Marshal(map[string]string{
		"email":    adminEmail,
		"password": adminPass,
	})
	w := request(h, "POST", "/api/v1/admin/auth/login", "", string(loginBody))
	var loginRes struct {
		SessionToken string `json:"session_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &loginRes)
	adminToken := loginRes.SessionToken

	// Register applicant
	regBody := `{"full_name":"Tunde Bakare","email":"tunde@example.ng","phone":"+2348055556666","password":"Password123#","confirm_password":"Password123#"}`
	w = request(h, "POST", "/api/v1/auth/register", "", regBody)
	var regRes struct {
		SessionToken string `json:"session_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &regRes)
	appToken := regRes.SessionToken

	// Profile
	prof := `{
		"full_name": "Tunde Bakare",
		"date_of_birth": "1995-03-21",
		"nin": "98765432101",
		"phone": "+2348055556666",
		"email": "tunde@example.ng",
		"state_of_origin": "Oyo",
		"lga": "Ibadan North",
		"residential_address": "12 Ring Road, Ibadan",
		"business_category": "unregistered",
		"business_name": "Tunde Tech Supplies",
		"business_type": "Electronics",
		"business_address": "Shop 4 Dugbe",
		"state_of_operation": "Oyo",
		"lga_of_operation": "Ibadan North",
		"operating_history": "1 - 2 years",
		"grant_purpose": "equipment",
		"bank_name": "First Bank of Nigeria",
		"account_number": "2012345678"
	}`
	_ = request(h, "PUT", "/api/v1/profile", appToken, prof)
	w = request(h, "POST", "/api/v1/applications/start", appToken, `{"category":"unregistered"}`)
	var app grants.Application
	_ = json.Unmarshal(w.Body.Bytes(), &app)

	pngBytes := makeTestDocPNG()
	_ = uploadTestDoc(h, appToken, app.ID, "passport_photo", pngBytes)
	_ = uploadTestDoc(h, appToken, app.ID, "government_id", pngBytes)
	_ = request(h, "POST", "/api/v1/applications/ready-for-payment", appToken, `{"application_id":"`+app.ID+`"}`)
	w = request(h, "POST", "/api/v1/payments/init", appToken, `{"application_id":"`+app.ID+`"}`)
	var payRes struct {
		Payment grants.Payment `json:"payment"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &payRes)
	w = request(h, "POST", "/api/v1/payments/verify", appToken, `{"reference":"`+payRes.Payment.Reference+`","simulator_action":"simulate_success"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("payment verify failed: %d %s", w.Code, w.Body.String())
	}
	w = request(h, "POST", "/api/v1/applications/submit", appToken, `{"application_id":"`+app.ID+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("submit application failed: %d %s", w.Code, w.Body.String())
	}

	// Reject application with reason
	rejectBody := `{"reason":"Inauthentic identity documentation provided."}`
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/reject", adminToken, rejectBody)
	if w.Code != http.StatusOK {
		t.Fatalf("rejection failed: %d %s", w.Code, w.Body.String())
	}

	// Verify metrics reflect rejection
	w = request(h, "GET", "/api/v1/admin/metrics", adminToken, "")
	var mPost grants.AdminMetrics
	_ = json.Unmarshal(w.Body.Bytes(), &mPost)
	if mPost.RejectedApplications != 1 {
		t.Errorf("expected 1 rejected application, got %d", mPost.RejectedApplications)
	}

	// Verify application details reflect rejected status and reason
	w = request(h, "GET", "/api/v1/admin/applications/"+app.ID, adminToken, "")
	var appDetails grants.Application
	_ = json.Unmarshal(w.Body.Bytes(), &appDetails)
	if appDetails.Status != grants.StatusRejected {
		t.Errorf("expected status rejected, got %s", appDetails.Status)
	}
	if appDetails.Reason != "Inauthentic identity documentation provided." {
		t.Errorf("expected rejection reason, got %s", appDetails.Reason)
	}

	// Duplicate rejection returns 409 Conflict
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/reject", adminToken, rejectBody)
	if w.Code != http.StatusConflict {
		t.Errorf("duplicate rejection should return 409 Conflict, got %d", w.Code)
	}
}

func TestAdminFrontend_LoadAdminDefinedAndDoubleSubmitProtection(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	h := New(s, strings.Repeat("x", 40))

	// Fetch /app.js
	req := httptest.NewRequest("GET", "/app.js", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /app.js, got %d", w.Code)
	}
	code := w.Body.String()

	// 1. Assert loadAdmin is defined as a function
	if !strings.Contains(code, "async function loadAdmin(") && !strings.Contains(code, "function loadAdmin(") {
		t.Errorf("expected loadAdmin function to be defined in app.js")
	}

	// 2. Assert approveForm has double-click protection
	if !strings.Contains(code, "approveForm.dataset.submitting") {
		t.Errorf("expected approveForm to have duplicate submission check (dataset.submitting)")
	}

	// 3. Assert rejectForm has double-click protection
	if !strings.Contains(code, "rejectForm.dataset.submitting") {
		t.Errorf("expected rejectForm to have duplicate submission check (dataset.submitting)")
	}

	// 4. Assert disburseBtn has double-click protection
	if !strings.Contains(code, "disburseBtn.dataset.submitting") {
		t.Errorf("expected disburseBtn to have duplicate submission check (dataset.submitting)")
	}

	// 5. Assert checksForm has double-click protection
	if !strings.Contains(code, "checksForm.dataset.submitting") {
		t.Errorf("expected checksForm to have duplicate submission check (dataset.submitting)")
	}

	// 6. Assert refresh error after approval does not mask approval success
	if !strings.Contains(code, "Dashboard refresh error after approval") && !strings.Contains(code, "Dashboard refresh failed after approval") {
		t.Errorf("expected guarded dashboard refresh after approval")
	}
}

func TestAdminAuth_NoDefaultPasswordOrTokenLoginFallback(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	legacyToken := strings.Repeat("k", 40)
	h := New(s, legacyToken)

	// 1. Direct unit verification on VerifyAdminCredentials
	// Empty configured hash must NEVER match default password or legacy token
	if grants.VerifyAdminCredentials("admin@solex.ng", "AdminP@ssw0rd2026!", "admin@solex.ng", "") {
		t.Errorf("VerifyAdminCredentials must return false for default password when configuredHash is empty")
	}
	if grants.VerifyAdminCredentials("admin@solex.ng", legacyToken, "admin@solex.ng", "") {
		t.Errorf("VerifyAdminCredentials must return false for legacy token when configuredHash is empty")
	}
	if grants.VerifyAdminCredentials("admin@solex.ng", "", "admin@solex.ng", "") {
		t.Errorf("VerifyAdminCredentials must return false for empty password when configuredHash is empty")
	}

	// 2. HTTP Admin login when ADMIN_PASSWORD_HASH is unset
	t.Setenv("ADMIN_EMAIL", "admin@solex.ng")
	t.Setenv("ADMIN_PASSWORD_HASH", "")

	// Trying default prototype password
	body, _ := json.Marshal(map[string]string{
		"email":    "admin@solex.ng",
		"password": "AdminP@ssw0rd2026!",
	})
	w := request(h, "POST", "/api/v1/admin/auth/login", "", string(body))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when ADMIN_PASSWORD_HASH is unset and default password is used, got %d", w.Code)
	}

	// Trying legacy token as login password
	bodyToken, _ := json.Marshal(map[string]string{
		"email":    "admin@solex.ng",
		"password": legacyToken,
	})
	w = request(h, "POST", "/api/v1/admin/auth/login", "", string(bodyToken))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when legacy token is used as password in login, got %d", w.Code)
	}

	// 3. HTTP Admin login when ADMIN_PASSWORD_HASH is set to a bcrypt hash
	configuredPass := "CustomSecureAdminPass2026!"
	bcryptHash, _ := grants.HashPassword(configuredPass)
	t.Setenv("ADMIN_PASSWORD_HASH", bcryptHash)

	// Trying default prototype password must fail
	w = request(h, "POST", "/api/v1/admin/auth/login", "", string(body))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for default prototype password against configured hash, got %d", w.Code)
	}

	// Trying legacy token as password must fail
	w = request(h, "POST", "/api/v1/admin/auth/login", "", string(bodyToken))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for legacy token as password against configured hash, got %d", w.Code)
	}

	// Only configuredPass succeeds
	goodBody, _ := json.Marshal(map[string]string{
		"email":    "admin@solex.ng",
		"password": configuredPass,
	})
	w = request(h, "POST", "/api/v1/admin/auth/login", "", string(goodBody))
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for configured password, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminAuth_EyeIconAccessibilityAndAttributesInHTMLAndJS(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	h := New(s, strings.Repeat("x", 40))

	// Fetch /app.js
	w := request(h, "GET", "/app.js", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /app.js, got %d", w.Code)
	}
	js := w.Body.String()

	// 1. Check password field helper pwField
	if !strings.Contains(js, `type="password"`) {
		t.Errorf("expected input type='password' by default")
	}
	if !strings.Contains(js, `class="pw-toggle"`) {
		t.Errorf("expected pw-toggle class on toggle button")
	}
	if !strings.Contains(js, `aria-label="Show password"`) {
		t.Errorf("expected accessible aria-label='Show password'")
	}
	if !strings.Contains(js, `aria-pressed="false"`) {
		t.Errorf("expected aria-pressed='false'")
	}
	if !strings.Contains(js, `type="button"`) {
		t.Errorf("expected type='button' on eye toggle button")
	}
	if !strings.Contains(js, `EYE_OPEN_SVG`) || !strings.Contains(js, `EYE_CLOSED_SVG`) {
		t.Errorf("expected SVG eye icon definitions")
	}

	// 2. Check toggle functionality in initPasswordToggles
	if !strings.Contains(js, `input.type = willShow ? 'text' : 'password'`) {
		t.Errorf("expected toggle between text and password types")
	}
	if !strings.Contains(js, `btn.setAttribute('aria-label', willShow ? 'Hide password' : 'Show password')`) {
		t.Errorf("expected dynamic aria-label update on toggle")
	}
	if !strings.Contains(js, `btn.setAttribute('aria-pressed', willShow ? 'true' : 'false')`) {
		t.Errorf("expected dynamic aria-pressed update on toggle")
	}

	// 3. Check admin login view uses pwField
	if !strings.Contains(js, `function adminLoginView()`) {
		t.Errorf("expected adminLoginView function")
	}
	if !strings.Contains(js, `pwField('password', 'Administrator Password', 8)`) {
		t.Errorf("expected adminLoginView to use pwField with toggle")
	}

	// 4. Check CSS for keyboard focus visibility and button styling
	wCSS := request(h, "GET", "/style.css", "", "")
	if wCSS.Code != http.StatusOK {
		t.Fatalf("expected 200 for /style.css, got %d", wCSS.Code)
	}
	css := wCSS.Body.String()
	if !strings.Contains(css, ".pw-toggle") {
		t.Errorf("expected .pw-toggle in style.css")
	}
	if !strings.Contains(css, ".pw-toggle:focus-visible") {
		t.Errorf("expected :focus-visible styling for keyboard accessibility in style.css")
	}
}

func TestAdminAuth_ResetApplicantPasswordWorkflow(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	adminEmail := "director@solex.ng"
	adminPass := "SecureAdminPass2026!"
	adminHash, _ := grants.HashPassword(adminPass)
	t.Setenv("ADMIN_EMAIL", adminEmail)
	t.Setenv("ADMIN_PASSWORD_HASH", adminHash)

	h := New(s, strings.Repeat("r", 40))

	// 1. Log in admin
	loginBody, _ := json.Marshal(map[string]string{
		"email":    adminEmail,
		"password": adminPass,
	})
	w := request(h, "POST", "/api/v1/admin/auth/login", "", string(loginBody))
	if w.Code != http.StatusOK {
		t.Fatalf("admin login failed: %d %s", w.Code, w.Body.String())
	}
	var loginRes struct {
		SessionToken string `json:"session_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &loginRes)
	adminToken := loginRes.SessionToken

	// 2. Register applicant
	regBody := `{"full_name":"Chidinma Eze","email":"chidinma@example.ng","phone":"+2348055554433","password":"OldPassword2026!","confirm_password":"OldPassword2026!"}`
	w = request(h, "POST", "/api/v1/auth/register", "", regBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("register failed: %d %s", w.Code, w.Body.String())
	}
	var regRes struct {
		SessionToken string `json:"session_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &regRes)
	origAppToken := regRes.SessionToken

	// 3. Save profile & create application
	prof := `{
		"full_name": "Chidinma Eze",
		"date_of_birth": "1994-06-15",
		"nin": "55512345678",
		"phone": "+2348055554433",
		"email": "chidinma@example.ng",
		"state_of_origin": "Enugu",
		"lga": "Nsukka",
		"residential_address": "15 University Road, Nsukka",
		"business_name": "Chidinma Agro Enterprises",
		"business_type": "Agribusiness",
		"state_of_operation": "Enugu",
		"lga_of_operation": "Nsukka",
		"business_address": "Market Road, Nsukka",
		"operating_history": "3 years",
		"bank_name": "Access Bank",
		"account_number": "0123456789",
		"grant_purpose": "agriculture",
		"business_category": "unregistered"
	}`
	w = request(h, "PUT", "/api/v1/profile", origAppToken, prof)
	if w.Code != http.StatusOK {
		t.Fatalf("save profile failed: %d %s", w.Code, w.Body.String())
	}
	w = request(h, "POST", "/api/v1/applications/start", origAppToken, `{"category":"unregistered"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create application failed: %d %s", w.Code, w.Body.String())
	}
	var app struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &app)

	// 4. Test unauthenticated request to reset password endpoint
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/reset-password", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 unauthenticated on reset-password, got %d", w.Code)
	}

	// 5. Test applicant token trying to reset password endpoint
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/reset-password", origAppToken, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 applicant token on reset-password, got %d", w.Code)
	}

	// 6. Admin resets applicant password
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/reset-password", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("admin reset password failed: %d %s", w.Code, w.Body.String())
	}
	var resetRes struct {
		Status            string `json:"status"`
		Message           string `json:"message"`
		TemporaryPassword string `json:"temporary_password"`
		UserID            string `json:"user_id"`
		Email             string `json:"email"`
		FullName          string `json:"full_name"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resetRes); err != nil {
		t.Fatalf("failed to decode reset response: %v", err)
	}
	if resetRes.Status != "ok" || resetRes.TemporaryPassword == "" {
		t.Fatalf("invalid reset response: %+v", resetRes)
	}
	if len(resetRes.TemporaryPassword) < 12 {
		t.Errorf("temporary password should be at least 12 chars, got %d", len(resetRes.TemporaryPassword))
	}
	if resetRes.Email != "chidinma@example.ng" {
		t.Errorf("expected email chidinma@example.ng, got %s", resetRes.Email)
	}

	// 7. Verify previous applicant session is invalidated
	w = request(h, "GET", "/api/v1/profile", origAppToken, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected old applicant session to be revoked (401), got %d", w.Code)
	}

	// 8. Verify old password cannot log in
	badLogin, _ := json.Marshal(map[string]string{
		"email":    "chidinma@example.ng",
		"password": "OldPassword2026!",
	})
	w = request(h, "POST", "/api/v1/auth/login", "", string(badLogin))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with old password after reset, got %d", w.Code)
	}

	// 9. Applicant logs in with temporary password
	tempLogin, _ := json.Marshal(map[string]string{
		"email":    "chidinma@example.ng",
		"password": resetRes.TemporaryPassword,
	})
	w = request(h, "POST", "/api/v1/auth/login", "", string(tempLogin))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with temporary password, got %d: %s", w.Code, w.Body.String())
	}
	var tempLoginRes struct {
		SessionToken string `json:"session_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &tempLoginRes)
	newAppToken := tempLoginRes.SessionToken

	// 10. Applicant changes password
	changeBody, _ := json.Marshal(map[string]string{
		"current_password": resetRes.TemporaryPassword,
		"new_password":     "MyBrandNewSecurePass2026!",
		"confirm_password": "MyBrandNewSecurePass2026!",
	})
	w = request(h, "POST", "/api/v1/auth/change-password", newAppToken, string(changeBody))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on change-password, got %d: %s", w.Code, w.Body.String())
	}

	// 11. Verify login with the brand new password succeeds
	brandNewLogin, _ := json.Marshal(map[string]string{
		"email":    "chidinma@example.ng",
		"password": "MyBrandNewSecurePass2026!",
	})
	w = request(h, "POST", "/api/v1/auth/login", "", string(brandNewLogin))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with brand new password, got %d: %s", w.Code, w.Body.String())
	}

	// 12. Check emails queue contains notification with temp password
	w = request(h, "GET", "/api/v1/admin/emails", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on /admin/emails, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), resetRes.TemporaryPassword) {
		t.Errorf("expected emails queue to record reset email")
	}
}

func TestAdminAuth_NoExposedHashesOrPlaintextPasswords(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	adminEmail := "secadmin@solex.ng"
	adminPass := "TopSecretPassword2026!"
	adminHash, _ := grants.HashPassword(adminPass)
	t.Setenv("ADMIN_EMAIL", adminEmail)
	t.Setenv("ADMIN_PASSWORD_HASH", adminHash)

	h := New(s, strings.Repeat("s", 40))

	// 1. Admin login
	loginBody, _ := json.Marshal(map[string]string{
		"email":    adminEmail,
		"password": adminPass,
	})
	w := request(h, "POST", "/api/v1/admin/auth/login", "", string(loginBody))
	if w.Code != http.StatusOK {
		t.Fatalf("admin login failed: %d %s", w.Code, w.Body.String())
	}
	bodyStr := w.Body.String()
	if strings.Contains(bodyStr, adminPass) || strings.Contains(bodyStr, adminHash) || strings.Contains(bodyStr, "password_hash") {
		t.Errorf("admin login response exposed password or hash: %s", bodyStr)
	}

	var loginRes struct {
		SessionToken string `json:"session_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &loginRes)
	adminToken := loginRes.SessionToken

	// 2. Admin auth me
	w = request(h, "GET", "/api/v1/admin/auth/me", adminToken, "")
	bodyStr = w.Body.String()
	if strings.Contains(bodyStr, adminPass) || strings.Contains(bodyStr, adminHash) || strings.Contains(bodyStr, "password_hash") {
		t.Errorf("admin auth me exposed password or hash: %s", bodyStr)
	}

	// 3. Register applicant
	regBody := `{"full_name":"Tunde Bakare","email":"tunde@example.ng","phone":"+2348077778899","password":"ApplicantSecret2026!","confirm_password":"ApplicantSecret2026!"}`
	w = request(h, "POST", "/api/v1/auth/register", "", regBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("register failed: %d %s", w.Code, w.Body.String())
	}
	bodyStr = w.Body.String()
	if strings.Contains(bodyStr, "ApplicantSecret2026!") || strings.Contains(bodyStr, "password_hash") {
		t.Errorf("applicant register response exposed password or hash: %s", bodyStr)
	}

	var regRes struct {
		SessionToken string `json:"session_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &regRes)
	appToken := regRes.SessionToken

	// 4. Applicant auth me
	w = request(h, "GET", "/api/v1/auth/me", appToken, "")
	bodyStr = w.Body.String()
	if strings.Contains(bodyStr, "ApplicantSecret2026!") || strings.Contains(bodyStr, "password_hash") {
		t.Errorf("applicant auth me exposed password or hash: %s", bodyStr)
	}

	// 5. Admin audit log
	w = request(h, "GET", "/api/v1/admin/audit", adminToken, "")
	bodyStr = w.Body.String()
	if strings.Contains(bodyStr, adminPass) || strings.Contains(bodyStr, adminHash) || strings.Contains(bodyStr, "ApplicantSecret2026!") || strings.Contains(bodyStr, "password_hash") {
		t.Errorf("admin audit log exposed plaintext passwords or hashes: %s", bodyStr)
	}
}
