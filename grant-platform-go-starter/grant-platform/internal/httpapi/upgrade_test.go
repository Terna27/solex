package httpapi

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"grantplatform/internal/grants"
)

func makeTestPNG() []byte {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestFullHTTPUpgradeWorkflow(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 5)
	if err != nil {
		t.Fatal(err)
	}
	adminToken := strings.Repeat("b", 40)
	h := New(s, adminToken)

	// 1. Programme configuration API
	w := request(h, "GET", "/api/v1/programme", "", "")
	if w.Code != 200 {
		t.Fatalf("GET /programme returned %d", w.Code)
	}
	var prog grants.ProgrammeConfig
	if err := json.Unmarshal(w.Body.Bytes(), &prog); err != nil {
		t.Fatal(err)
	}
	if prog.CommencementDate != "2026-12-10" {
		t.Fatalf("expected commencement 2026-12-10, got %s", prog.CommencementDate)
	}
	if prog.Categories["unregistered"].AmountKobo != 25000000 || prog.Categories["registered"].AmountKobo != 50000000 {
		t.Fatalf("wrong category amounts: %+v", prog.Categories)
	}

	// 2. Fee disclosure API
	w = request(h, "GET", "/api/v1/fee-disclosure", "", "")
	if w.Code != 200 {
		t.Fatalf("GET /fee-disclosure returned %d", w.Code)
	}
	var disclosure grants.FeeDisclosure
	if err := json.Unmarshal(w.Body.Bytes(), &disclosure); err != nil {
		t.Fatal(err)
	}
	if disclosure.FeeAmountKobo != 100000 {
		t.Fatalf("expected 100000 kobo (₦1,000), got %d", disclosure.FeeAmountKobo)
	}

	// 3. Register applicant account
	regBody := `{"full_name":"Halima Danjuma","email":"halima@example.ng","phone":"+2348023456789","password":"SuperSecretP@ss1","confirm_password":"SuperSecretP@ss1"}`
	w = request(h, "POST", "/api/v1/auth/register", "", regBody)
	if w.Code != 201 {
		t.Fatalf("registration failed: %d %s", w.Code, w.Body.String())
	}
	var regResp struct {
		User         grants.User `json:"user"`
		SessionToken string      `json:"session_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &regResp); err != nil {
		t.Fatal(err)
	}
	sessionToken := regResp.SessionToken

	// Verify cookie was set
	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "solex_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value != sessionToken || !sessionCookie.HttpOnly {
		t.Fatalf("expected HttpOnly solex_session cookie, got %+v", sessionCookie)
	}

	// 4. Check /api/v1/auth/me
	w = request(h, "GET", "/api/v1/auth/me", sessionToken, "")
	if w.Code != 200 {
		t.Fatalf("GET /auth/me failed: %d %s", w.Code, w.Body.String())
	}

	// 5. Complete Profile with NIN and Nigerian bank
	profileBody := `{
		"full_name": "Halima Danjuma",
		"date_of_birth": "1994-08-20",
		"phone": "+2348023456789",
		"email": "halima@example.ng",
		"nin": "98765432109",
		"state_of_origin": "Kano",
		"lga": "Nassarawa",
		"residential_address": "15 Bompai Road, Kano",
		"business_category": "unregistered",
		"business_name": "Halima Textile Enterprise",
		"business_type": "Market trading",
		"business_description": "Wholesale and retail sales of Northern fabrics and traditional textiles.",
		"business_address": "Shop 12 Kurmi Market, Kano",
		"state_of_operation": "Kano",
		"lga_of_operation": "Kano Municipal",
		"operating_history": "3-5 years",
		"grant_purpose": "working_capital",
		"bank_name": "Access Bank",
		"account_number": "0123456789",
		"account_name": "Halima Danjuma"
	}`
	w = request(h, "PUT", "/api/v1/profile", sessionToken, profileBody)
	if w.Code != 200 {
		t.Fatalf("PUT /profile failed: %d %s", w.Code, w.Body.String())
	}

	// 6. Start Application
	startBody := `{"category":"unregistered"}`
	w = request(h, "POST", "/api/v1/applications/start", sessionToken, startBody)
	if w.Code != 201 {
		t.Fatalf("POST /applications/start failed: %d %s", w.Code, w.Body.String())
	}
	var app grants.Application
	if err := json.Unmarshal(w.Body.Bytes(), &app); err != nil {
		t.Fatal(err)
	}
	if app.GrantAmountKobo != 25000000 {
		t.Fatalf("expected 25000000 kobo (₦250k), got %d", app.GrantAmountKobo)
	}

	// 7. Upload Passport and Government ID
	pngBytes := makeTestPNG()
	for _, kind := range []string{grants.DocPassportPhoto, grants.DocGovernmentID} {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		_ = mw.WriteField("kind", kind)
		fw, _ := mw.CreateFormFile("file", "image.png")
		_, _ = fw.Write(pngBytes)
		_ = mw.Close()

		req := httptest.NewRequest("POST", "/api/v1/applications/"+app.ID+"/documents", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+sessionToken)
		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, req)
		if rw.Code != 201 {
			t.Fatalf("document upload for %s failed: %d %s", kind, rw.Code, rw.Body.String())
		}
	}

	// 8. Ready for payment
	readyBody := `{"application_id":"` + app.ID + `"}`
	w = request(h, "POST", "/api/v1/applications/ready-for-payment", sessionToken, readyBody)
	if w.Code != 200 {
		t.Fatalf("POST /applications/ready-for-payment failed: %d %s", w.Code, w.Body.String())
	}

	// 9. Init payment
	w = request(h, "POST", "/api/v1/payments/init", sessionToken, readyBody)
	if w.Code != 200 {
		t.Fatalf("POST /payments/init failed: %d %s", w.Code, w.Body.String())
	}
	var initResp struct {
		Payment grants.Payment `json:"payment"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &initResp); err != nil {
		t.Fatal(err)
	}
	payRef := initResp.Payment.Reference

	// 10. Verify payment via sandbox simulator
	verifyBody := `{"reference":"` + payRef + `","simulator_action":"simulate_success"}`
	w = request(h, "POST", "/api/v1/payments/verify", sessionToken, verifyBody)
	if w.Code != 200 {
		t.Fatalf("POST /payments/verify failed: %d %s", w.Code, w.Body.String())
	}

	// 11. Final submit application
	submitBody := `{"application_id":"` + app.ID + `"}`
	w = request(h, "POST", "/api/v1/applications/submit", sessionToken, submitBody)
	if w.Code != 200 {
		t.Fatalf("POST /applications/submit failed: %d %s", w.Code, w.Body.String())
	}

	// 12. Admin operations
	// Admin Metrics
	w = request(h, "GET", "/api/v1/admin/metrics", adminToken, "")
	if w.Code != 200 {
		t.Fatalf("GET /admin/metrics failed: %d %s", w.Code, w.Body.String())
	}
	var metrics grants.AdminMetrics
	if err := json.Unmarshal(w.Body.Bytes(), &metrics); err != nil {
		t.Fatal(err)
	}
	if metrics.TotalApplications != 1 || metrics.SubmittedApplications != 1 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
	if metrics.VerifiedFeesCollectedKobo != 100000 {
		t.Fatalf("expected 100000 kobo fees collected, got %d", metrics.VerifiedFeesCollectedKobo)
	}

	// Admin checklist
	checksBody := `{"phone":true,"identity_and_age":true,"address":true,"business":true,"bank_account_ownership":true}`
	w = request(h, "PUT", "/api/v1/admin/applications/"+app.ID+"/checks", adminToken, checksBody)
	if w.Code != 200 {
		t.Fatalf("PUT /admin/checks failed: %d %s", w.Code, w.Body.String())
	}

	// Admin approve
	approveBody := `{"disbursement_batch":"Batch A","disbursement_date":"2026-12-10"}`
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, approveBody)
	if w.Code != 200 {
		t.Fatalf("POST /admin/approve failed: %d %s", w.Code, w.Body.String())
	}

	// Admin disburse
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/disburse", adminToken, "")
	if w.Code != 200 {
		t.Fatalf("POST /admin/disburse failed: %d %s", w.Code, w.Body.String())
	}

	// Admin CSV Export
	w = request(h, "GET", "/api/v1/admin/export?format=csv", adminToken, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Halima Danjuma") {
		t.Fatalf("admin export failed: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "*******2109") {
		t.Fatalf("export did not mask NIN: %s", w.Body.String())
	}

	// Admin Audit
	w = request(h, "GET", "/api/v1/admin/audit", adminToken, "")
	if w.Code != 200 {
		t.Fatalf("admin audit failed: %d", w.Code)
	}

	// Admin Emails
	w = request(h, "GET", "/api/v1/admin/emails", adminToken, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "halima@example.ng") {
		t.Fatalf("admin emails check failed: %d", w.Code)
	}
}
