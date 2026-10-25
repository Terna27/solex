package httpapi

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"grantplatform/internal/grants"
)

func makeTestDocPNG() []byte {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func uploadTestDoc(h http.Handler, sessionToken, appID, kind string, content []byte) *httptest.ResponseRecorder {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("kind", kind)
	f, _ := mw.CreateFormFile("file", "test.png")
	_, _ = f.Write(content)
	_ = mw.Close()

	r := httptest.NewRequest("POST", "/api/v1/applications/"+appID+"/documents", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	if sessionToken != "" {
		r.Header.Set("Authorization", "Bearer "+sessionToken)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestPublicHomepageAndGrantAmounts(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatal(err)
	}
	adminToken := strings.Repeat("c", 40)
	h := New(s, adminToken)

	// 1. Verify GET / serves index.html and does NOT display grant amounts (₦250,000 or ₦500,000)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("GET / returned status %d", w.Code)
	}
	body := w.Body.String()

	prohibitedAmounts := []string{"250,000", "500,000", "250000", "500000", "₦250k", "₦500k"}
	for _, prohibited := range prohibitedAmounts {
		if strings.Contains(body, prohibited) {
			t.Fatalf("Public homepage index.html leaked grant amount: %q", prohibited)
		}
	}

	// 2. Verify static landing page docs/index.html also does not display grant amounts
	docsPath := filepath.Join("..", "..", "..", "docs", "index.html")
	if docsBytes, err := os.ReadFile(docsPath); err == nil {
		docsContent := string(docsBytes)
		for _, prohibited := range prohibitedAmounts {
			if strings.Contains(docsContent, prohibited) {
				t.Fatalf("docs/index.html leaked grant amount: %q", prohibited)
			}
		}
		if strings.Contains(strings.ToLower(docsContent), "apply for grant") {
			t.Fatalf("docs/index.html exposes 'Apply for Grant' button before authentication")
		}
	}

	// 3. Verify public homepage does NOT expose 'Apply for Grant' button before authentication
	if strings.Contains(body, "Apply for Grant") {
		t.Fatalf("Public homepage HTML exposed 'Apply for Grant' to unauthenticated visitors")
	}

	// Verify web/app.js nav configuration for unauthenticated visitors
	appJSBytes, err := os.ReadFile(filepath.Join("web", "app.js"))
	if err != nil {
		t.Fatal(err)
	}
	appJS := string(appJSBytes)

	// Ensure home() function does not hardcode public amounts
	homeFuncIdx := strings.Index(appJS, "function home()")
	if homeFuncIdx != -1 {
		aboutIdx := strings.Index(appJS, "function about()")
		if aboutIdx != -1 && aboutIdx > homeFuncIdx {
			homeBlock := appJS[homeFuncIdx:aboutIdx]
			if strings.Contains(homeBlock, "₦250,000") || strings.Contains(homeBlock, "₦500,000") {
				t.Fatalf("web/app.js home() contains hardcoded grant amounts")
			}
		}
	}
}

func TestUnauthenticatedCannotStartApplications(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatal(err)
	}
	adminToken := strings.Repeat("c", 40)
	h := New(s, adminToken)

	// Unauthenticated start application must be rejected
	w := request(h, "POST", "/api/v1/applications/start", "", `{"category":"unregistered"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for unauthenticated start application, got %d: %s", w.Code, w.Body.String())
	}

	// Unauthenticated get current application must be rejected
	w = request(h, "GET", "/api/v1/applications/current", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for unauthenticated get current application, got %d: %s", w.Code, w.Body.String())
	}

	// Unauthenticated ready for payment must be rejected
	w = request(h, "POST", "/api/v1/applications/ready-for-payment", "", `{"application_id":"dummy"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for unauthenticated ready-for-payment, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthenticatedApplicationAndCategoryFlow(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatal(err)
	}
	adminToken := strings.Repeat("c", 40)
	h := New(s, adminToken)
	pngBytes := makeTestDocPNG()

	// 1. Programme configuration returns correct amounts
	w := request(h, "GET", "/api/v1/programme", "", "")
	if w.Code != 200 {
		t.Fatalf("GET /programme returned %d", w.Code)
	}
	var prog grants.ProgrammeConfig
	if err := json.Unmarshal(w.Body.Bytes(), &prog); err != nil {
		t.Fatal(err)
	}
	if prog.Categories["unregistered"].AmountKobo != 25000000 {
		t.Fatalf("expected unregistered amount 25,000,000 kobo (₦250,000), got %d", prog.Categories["unregistered"].AmountKobo)
	}
	if prog.Categories["registered"].AmountKobo != 50000000 {
		t.Fatalf("expected registered amount 50,000,000 kobo (₦500,000), got %d", prog.Categories["registered"].AmountKobo)
	}

	// 2. Register applicant 1 (Unregistered Business)
	reg1 := `{"full_name":"Chinedu Eze","email":"chinedu@example.ng","phone":"+2348033334444","password":"Password123#","confirm_password":"Password123#"}`
	w = request(h, "POST", "/api/v1/auth/register", "", reg1)
	if w.Code != 201 {
		t.Fatalf("register failed: %d %s", w.Code, w.Body.String())
	}
	var r1 struct {
		User         grants.User `json:"user"`
		SessionToken string      `json:"session_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &r1)
	token1 := r1.SessionToken

	// Save profile for applicant 1
	prof1 := `{
		"full_name": "Chinedu Eze",
		"date_of_birth": "1992-05-15",
		"nin": "12345678901",
		"phone": "+2348033334444",
		"email": "chinedu@example.ng",
		"state_of_origin": "Enugu",
		"lga": "Nsukka",
		"residential_address": "45 Market Road, Nsukka",
		"business_category": "unregistered",
		"business_name": "Chinedu Provisions",
		"business_type": "Retail shop",
		"business_address": "Shop 12 Nsukka Main Market",
		"state_of_operation": "Enugu",
		"lga_of_operation": "Nsukka",
		"operating_history": "3 - 5 years",
		"grant_purpose": "inventory",
		"bank_name": "Access Bank",
		"account_number": "0123456789"
	}`
	w = request(h, "PUT", "/api/v1/profile", token1, prof1)
	if w.Code != 200 {
		t.Fatalf("PUT /profile failed: %d %s", w.Code, w.Body.String())
	}

	// Authenticated applicant can access Apply for Grant (start application)
	w = request(h, "POST", "/api/v1/applications/start", token1, `{"category":"unregistered"}`)
	if w.Code != 201 {
		t.Fatalf("POST /applications/start failed: %d %s", w.Code, w.Body.String())
	}
	var appUnregistered grants.Application
	_ = json.Unmarshal(w.Body.Bytes(), &appUnregistered)

	// Verify server calculated correct amount for unregistered: ₦250,000 (25,000,000 kobo)
	if appUnregistered.Category != "unregistered" {
		t.Fatalf("expected category unregistered, got %s", appUnregistered.Category)
	}
	if appUnregistered.GrantAmountKobo != 25000000 {
		t.Fatalf("expected 25,000,000 kobo, got %d", appUnregistered.GrantAmountKobo)
	}

	// Upload passport and ID for unregistered business
	w = uploadTestDoc(h, token1, appUnregistered.ID, "passport_photo", pngBytes)
	if w.Code != 201 {
		t.Fatalf("passport upload failed: %d %s", w.Code, w.Body.String())
	}
	w = uploadTestDoc(h, token1, appUnregistered.ID, "government_id", pngBytes)
	if w.Code != 201 {
		t.Fatalf("id upload failed: %d %s", w.Code, w.Body.String())
	}

	// Unregistered business does NOT require CAC document to transition to awaiting_payment
	w = request(h, "POST", "/api/v1/applications/ready-for-payment", token1, `{"application_id":"`+appUnregistered.ID+`"}`)
	if w.Code != 200 {
		t.Fatalf("unregistered business should transition without CAC certificate: %d %s", w.Code, w.Body.String())
	}
	var readyApp1 grants.Application
	_ = json.Unmarshal(w.Body.Bytes(), &readyApp1)
	if readyApp1.Status != grants.StatusAwaitingPayment {
		t.Fatalf("expected status awaiting_payment, got %s", readyApp1.Status)
	}

	// 3. Register applicant 2 (Registered Business)
	reg2 := `{"full_name":"Aminu Bello","email":"aminu@example.ng","phone":"+2348022223333","password":"Password123#","confirm_password":"Password123#"}`
	w = request(h, "POST", "/api/v1/auth/register", "", reg2)
	if w.Code != 201 {
		t.Fatalf("register failed: %d %s", w.Code, w.Body.String())
	}
	var r2 struct {
		User         grants.User `json:"user"`
		SessionToken string      `json:"session_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &r2)
	token2 := r2.SessionToken

	prof2 := `{
		"full_name": "Aminu Bello",
		"date_of_birth": "1988-11-20",
		"nin": "98765432101",
		"phone": "+2348022223333",
		"email": "aminu@example.ng",
		"state_of_origin": "Kano",
		"lga": "Dala",
		"residential_address": "10 Airport Road, Kano",
		"business_category": "registered",
		"cac_number": "RC-998877",
		"registered_business_name": "Bello Agro Logistics Ltd",
		"business_name": "Bello Agro Logistics",
		"business_type": "Agriculture & agro-processing",
		"business_address": "Plot 5 Industrial Layout, Kano",
		"state_of_operation": "Kano",
		"lga_of_operation": "Dala",
		"operating_history": "More than 5 years",
		"grant_purpose": "equipment",
		"bank_name": "Zenith Bank",
		"account_number": "1012345678"
	}`
	w = request(h, "PUT", "/api/v1/profile", token2, prof2)
	if w.Code != 200 {
		t.Fatalf("PUT /profile for registered business failed: %d %s", w.Code, w.Body.String())
	}

	// Start application for registered business
	w = request(h, "POST", "/api/v1/applications/start", token2, `{"category":"registered"}`)
	if w.Code != 201 {
		t.Fatalf("POST /applications/start failed: %d %s", w.Code, w.Body.String())
	}
	var appRegistered grants.Application
	_ = json.Unmarshal(w.Body.Bytes(), &appRegistered)

	// Verify server calculated correct amount for registered: ₦500,000 (50,000,000 kobo)
	if appRegistered.Category != "registered" {
		t.Fatalf("expected category registered, got %s", appRegistered.Category)
	}
	if appRegistered.GrantAmountKobo != 50000000 {
		t.Fatalf("expected 50,000,000 kobo, got %d", appRegistered.GrantAmountKobo)
	}

	// Upload passport and ID only (omit CAC certificate)
	w = uploadTestDoc(h, token2, appRegistered.ID, "passport_photo", pngBytes)
	if w.Code != 201 {
		t.Fatalf("passport upload failed: %d %s", w.Code, w.Body.String())
	}
	w = uploadTestDoc(h, token2, appRegistered.ID, "government_id", pngBytes)
	if w.Code != 201 {
		t.Fatalf("id upload failed: %d %s", w.Code, w.Body.String())
	}

	// Registered business MUST require CAC certificate to advance to awaiting_payment
	w = request(h, "POST", "/api/v1/applications/ready-for-payment", token2, `{"application_id":"`+appRegistered.ID+`"}`)
	if w.Code == 200 {
		t.Fatalf("registered business without CAC certificate should NOT be allowed to proceed to payment")
	}

	// Now upload CAC Certificate
	w = uploadTestDoc(h, token2, appRegistered.ID, "cac_certificate", pngBytes)
	if w.Code != 201 {
		t.Fatalf("CAC certificate upload failed: %d %s", w.Code, w.Body.String())
	}

	// Now ready-for-payment must succeed
	w = request(h, "POST", "/api/v1/applications/ready-for-payment", token2, `{"application_id":"`+appRegistered.ID+`"}`)
	if w.Code != 200 {
		t.Fatalf("registered business with CAC certificate should succeed: %d %s", w.Code, w.Body.String())
	}
	var readyApp2 grants.Application
	_ = json.Unmarshal(w.Body.Bytes(), &readyApp2)
	if readyApp2.Status != grants.StatusAwaitingPayment {
		t.Fatalf("expected status awaiting_payment, got %s", readyApp2.Status)
	}
}

func TestHTTPProfileFormSimplificationAndAccountNameSync(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatal(err)
	}
	adminToken := strings.Repeat("d", 40)
	h := New(s, adminToken)

	// 1. Programme config contains grant purposes
	w := request(h, "GET", "/api/v1/programme", "", "")
	if w.Code != 200 {
		t.Fatalf("GET /programme returned %d", w.Code)
	}
	var prog grants.ProgrammeConfig
	_ = json.Unmarshal(w.Body.Bytes(), &prog)
	if len(prog.GrantPurposes) != 9 {
		t.Fatalf("expected 9 grant purposes, got %d", len(prog.GrantPurposes))
	}

	// Register user
	reg := `{"full_name":"Maryam Sani","email":"maryam@example.ng","phone":"+2348055556666","password":"Password123#","confirm_password":"Password123#"}`
	w = request(h, "POST", "/api/v1/auth/register", "", reg)
	if w.Code != 201 {
		t.Fatalf("register failed: %d %s", w.Code, w.Body.String())
	}
	var authResp struct {
		User         grants.User `json:"user"`
		SessionToken string      `json:"session_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &authResp)
	token := authResp.SessionToken

	// 2. Reject arbitrary free-text grant_purpose
	badPurposeBody := `{
		"full_name": "Maryam Sani",
		"date_of_birth": "1995-04-12",
		"nin": "55667788990",
		"phone": "+2348055556666",
		"email": "maryam@example.ng",
		"state_of_origin": "Kaduna",
		"lga": "Zaria",
		"residential_address": "12 Queen Amina Way, Zaria",
		"business_category": "unregistered",
		"business_name": "Maryam Crafts",
		"business_type": "Manufacturing & crafts",
		"business_address": "Zaria Central Market",
		"state_of_operation": "Kaduna",
		"lga_of_operation": "Zaria",
		"operating_history": "1 - 2 years",
		"grant_purpose": "arbitrary_custom_value",
		"bank_name": "Access Bank",
		"account_number": "0123456789"
	}`
	w = request(h, "PUT", "/api/v1/profile", token, badPurposeBody)
	if w.Code != 400 {
		t.Fatalf("expected 400 Bad Request for arbitrary grant_purpose, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Submit profile without business_description and without intended_use, and with spoofed account_name
	validProfBody := `{
		"full_name": "Maryam Sani",
		"date_of_birth": "1995-04-12",
		"nin": "55667788990",
		"phone": "+2348055556666",
		"email": "maryam@example.ng",
		"state_of_origin": "Kaduna",
		"lga": "Zaria",
		"residential_address": "12 Queen Amina Way, Zaria",
		"business_category": "unregistered",
		"business_name": "Maryam Crafts",
		"business_type": "Manufacturing & crafts",
		"business_address": "Zaria Central Market",
		"state_of_operation": "Kaduna",
		"lga_of_operation": "Zaria",
		"operating_history": "1 - 2 years",
		"grant_purpose": "working_capital",
		"bank_name": "Access Bank",
		"account_number": "0123456789",
		"account_name": "Imposter Account Name"
	}`
	w = request(h, "PUT", "/api/v1/profile", token, validProfBody)
	if w.Code != 200 {
		t.Fatalf("expected 200 OK for profile without description and intended use, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Verify account_name was derived from full_name and spoofed name was ignored
	var profResp struct {
		Profile grants.ApplicantProfile `json:"profile"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &profResp)
	if profResp.Profile.AccountName != "Maryam Sani" {
		t.Fatalf("expected derived account name 'Maryam Sani', got %q", profResp.Profile.AccountName)
	}

	// 5. Updating legal name updates derived account name
	updateNameBody := `{
		"full_name": "Maryam Sani Danbatta",
		"date_of_birth": "1995-04-12",
		"nin": "55667788990",
		"phone": "+2348055556666",
		"email": "maryam@example.ng",
		"state_of_origin": "Kaduna",
		"lga": "Zaria",
		"residential_address": "12 Queen Amina Way, Zaria",
		"business_category": "unregistered",
		"business_name": "Maryam Crafts",
		"business_type": "Manufacturing & crafts",
		"business_address": "Zaria Central Market",
		"state_of_operation": "Kaduna",
		"lga_of_operation": "Zaria",
		"operating_history": "1 - 2 years",
		"grant_purpose": "inventory",
		"bank_name": "Access Bank",
		"account_number": "0123456789"
	}`
	w = request(h, "PUT", "/api/v1/profile", token, updateNameBody)
	if w.Code != 200 {
		t.Fatalf("PUT /profile name update failed: %d %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &profResp)
	if profResp.Profile.FullName != "Maryam Sani Danbatta" || profResp.Profile.AccountName != "Maryam Sani Danbatta" {
		t.Fatalf("expected updated name and derived account name 'Maryam Sani Danbatta', got Name=%q AccountName=%q", profResp.Profile.FullName, profResp.Profile.AccountName)
	}

	// GET /profile also reflects the derived name
	w = request(h, "GET", "/api/v1/profile", token, "")
	if w.Code != 200 {
		t.Fatalf("GET /profile failed: %d", w.Code)
	}
	var getProf grants.ApplicantProfile
	_ = json.Unmarshal(w.Body.Bytes(), &getProf)
	if getProf.AccountName != "Maryam Sani Danbatta" {
		t.Fatalf("expected derived account name on GET, got %q", getProf.AccountName)
	}
}
