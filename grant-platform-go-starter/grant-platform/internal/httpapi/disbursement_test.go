package httpapi

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"grantplatform/internal/grants"
)

// Helper to set up an admin session and return the admin token
func setupTestAdmin(t *testing.T, h http.Handler) (string, string) {
	adminEmail := "lead@solex.ng"
	adminPass := "AdminP@ss2026!"
	hash, err := grants.HashPassword(adminPass)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	t.Setenv("ADMIN_EMAIL", adminEmail)
	t.Setenv("ADMIN_PASSWORD_HASH", hash)

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
	return loginRes.SessionToken, adminEmail
}

// Helper to create, verify payment, and submit a complete application
func setupSubmittedApp(t *testing.T, h http.Handler, name, email, phone, category string) (string, grants.Application) {
	regBody, _ := json.Marshal(map[string]string{
		"full_name":        name,
		"email":            email,
		"phone":            phone,
		"password":         "Password123#",
		"confirm_password": "Password123#",
	})
	w := request(h, "POST", "/api/v1/auth/register", "", string(regBody))
	if w.Code != http.StatusCreated {
		t.Fatalf("register failed: %d %s", w.Code, w.Body.String())
	}
	var regRes struct {
		SessionToken string `json:"session_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &regRes)
	appToken := regRes.SessionToken

	profMap := map[string]string{
		"full_name":           name,
		"date_of_birth":       "1990-01-01",
		"nin":                 "12345678901",
		"phone":               phone,
		"email":               email,
		"state_of_origin":     "Abia",
		"lga":                 "Umuahia",
		"residential_address": "10 Bank Road, Umuahia",
		"business_category":   category,
		"business_name":       name + " Enterprises",
		"business_type":       "Retail",
		"business_address":    "Market Square",
		"state_of_operation":  "Abia",
		"lga_of_operation":    "Umuahia",
		"operating_history":   "2 years",
		"grant_purpose":       "working_capital",
		"bank_name":           "Zenith Bank",
		"account_number":      "0123456789",
	}
	if category == "registered" {
		profMap["cac_number"] = "RC-998877"
		profMap["registered_business_name"] = name + " Enterprises Ltd"
	}
	profJSON, _ := json.Marshal(profMap)
	w = request(h, "PUT", "/api/v1/profile", appToken, string(profJSON))
	if w.Code != http.StatusOK {
		t.Fatalf("save profile failed: %d %s", w.Code, w.Body.String())
	}

	startBody, _ := json.Marshal(map[string]string{"category": category})
	w = request(h, "POST", "/api/v1/applications/start", appToken, string(startBody))
	if w.Code != http.StatusCreated {
		t.Fatalf("start application failed: %d %s", w.Code, w.Body.String())
	}
	var app grants.Application
	_ = json.Unmarshal(w.Body.Bytes(), &app)

	pngBytes := makeTestDocPNG()
	w = uploadTestDoc(h, appToken, app.ID, "passport_photo", pngBytes)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload passport failed: %d %s", w.Code, w.Body.String())
	}
	w = uploadTestDoc(h, appToken, app.ID, "government_id", pngBytes)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload ID document failed: %d %s", w.Code, w.Body.String())
	}
	if category == "registered" {
		w = uploadTestDoc(h, appToken, app.ID, "cac_certificate", pngBytes)
		if w.Code != http.StatusCreated {
			t.Fatalf("upload CAC cert failed: %d %s", w.Code, w.Body.String())
		}
	}

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

	w = request(h, "POST", "/api/v1/applications/submit", appToken, `{"application_id":"`+app.ID+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("submit application failed: %d %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &app)

	return appToken, app
}

// Complete verification checks as admin
func passAdminChecks(t *testing.T, h http.Handler, adminToken, appID, category string) {
	checks := map[string]bool{
		"phone":                  true,
		"identity_and_age":       true,
		"address":                true,
		"business":               true,
		"bank_account_ownership": true,
	}
	if category == "registered" {
		checks["cac_registration"] = true
	}
	checksJSON, _ := json.Marshal(checks)
	w := request(h, "PUT", "/api/v1/admin/applications/"+appID+"/checks", adminToken, string(checksJSON))
	if w.Code != http.StatusOK {
		t.Fatalf("admin checks failed: %d %s", w.Code, w.Body.String())
	}
}

func TestDisbursement_ApprovalRequiresBatchAndDecember2026Date(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	h := New(s, strings.Repeat("a", 40))
	adminToken, _ := setupTestAdmin(t, h)
	_, app := setupSubmittedApp(t, h, "Amina Yusuf", "amina@example.ng", "+2348011112233", "unregistered")
	passAdminChecks(t, h, adminToken, app.ID, "unregistered")

	// 1. Missing batch
	w := request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, `{"disbursement_date":"2026-12-10"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing batch, got %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "disbursement batch is required") {
		t.Errorf("expected batch required message, got %s", w.Body.String())
	}

	// 2. Invalid batch (e.g. Batch C)
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, `{"disbursement_batch":"Batch C","disbursement_date":"2026-12-10"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid batch, got %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid disbursement batch") {
		t.Errorf("expected invalid batch error, got %s", w.Body.String())
	}

	// 3. Missing date
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, `{"disbursement_batch":"Batch A"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing date, got %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "scheduled disbursement date is required") {
		t.Errorf("expected date required error, got %s", w.Body.String())
	}

	// 4. Invalid date format
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, `{"disbursement_batch":"Batch A","disbursement_date":"15-12-2026"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid date format, got %d %s", w.Code, w.Body.String())
	}

	// 5. Non-December 2026 date (November 2026)
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, `{"disbursement_batch":"Batch A","disbursement_date":"2026-11-30"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-December date, got %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "December 2026") {
		t.Errorf("expected December 2026 error message, got %s", w.Body.String())
	}

	// 6. Non-December 2026 date (January 2027)
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, `{"disbursement_batch":"Batch A","disbursement_date":"2027-01-05"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for 2027 date, got %d %s", w.Code, w.Body.String())
	}

	// 7. Successful approval with Batch A and valid December 2026 date
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, `{"disbursement_batch":"Batch A","disbursement_date":"2026-12-15"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("approval failed: %d %s", w.Code, w.Body.String())
	}

	// Verify application record
	w = request(h, "GET", "/api/v1/admin/applications/"+app.ID, adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get app failed: %d %s", w.Code, w.Body.String())
	}
	var approvedApp grants.Application
	_ = json.Unmarshal(w.Body.Bytes(), &approvedApp)
	if approvedApp.Status != "approved" {
		t.Errorf("expected status approved, got %s", approvedApp.Status)
	}
	if approvedApp.DisbursementBatch != "Batch A" {
		t.Errorf("expected Batch A, got %s", approvedApp.DisbursementBatch)
	}
	if approvedApp.DisbursementDate != "2026-12-15" {
		t.Errorf("expected 2026-12-15 disbursement date, got %v", approvedApp.DisbursementDate)
	}
	if approvedApp.GrantAmountKobo != 25000000 {
		t.Errorf("expected 25000000 kobo (₦250k), got %d", approvedApp.GrantAmountKobo)
	}
}

func TestDisbursement_AutomaticGrantAmountAndCategory(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	h := New(s, strings.Repeat("b", 40))
	adminToken, _ := setupTestAdmin(t, h)

	// Registered business should receive ₦500,000 (50,000,000 kobo)
	_, regApp := setupSubmittedApp(t, h, "Emeka Okafor", "emeka@example.ng", "+2348022223344", "registered")
	passAdminChecks(t, h, adminToken, regApp.ID, "registered")

	w := request(h, "POST", "/api/v1/admin/applications/"+regApp.ID+"/approve", adminToken, `{"disbursement_batch":"Batch B","disbursement_date":"2026-12-22"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("registered app approval failed: %d %s", w.Code, w.Body.String())
	}

	w = request(h, "GET", "/api/v1/admin/applications/"+regApp.ID, adminToken, "")
	var approvedReg grants.Application
	_ = json.Unmarshal(w.Body.Bytes(), &approvedReg)
	if approvedReg.DisbursementBatch != "Batch B" {
		t.Errorf("expected Batch B, got %s", approvedReg.DisbursementBatch)
	}
	if approvedReg.GrantAmountKobo != 50000000 {
		t.Errorf("expected 50,000,000 kobo (₦500,000) for registered business, got %d", approvedReg.GrantAmountKobo)
	}
}

func TestDisbursement_ApprovalEmailQueuedAndSanitized(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	h := New(s, strings.Repeat("c", 40))
	adminToken, _ := setupTestAdmin(t, h)
	_, app := setupSubmittedApp(t, h, "Chiamaka Nze", "chiamaka@example.ng", "+2348033334455", "unregistered")
	passAdminChecks(t, h, adminToken, app.ID, "unregistered")

	// Approve
	w := request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, `{"disbursement_batch":"Batch A","disbursement_date":"2026-12-18"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("approval failed: %d %s", w.Code, w.Body.String())
	}

	// Fetch outbox
	w = request(h, "GET", "/api/v1/admin/emails", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get emails failed: %d %s", w.Code, w.Body.String())
	}
	var emailList []grants.EmailMessage
	_ = json.Unmarshal(w.Body.Bytes(), &emailList)

	var approvalEmail *grants.EmailMessage
	for _, em := range emailList {
		if strings.Contains(em.ID, app.ID) || (em.To == "chiamaka@example.ng" && strings.Contains(em.Subject, "Approved")) {
			approvalEmail = &em
			break
		}
	}
	if approvalEmail == nil {
		t.Fatalf("no approval email found in outbox for %s: %+v", app.ID, emailList)
	}

	// 1. Check required fields
	if approvalEmail.To != "chiamaka@example.ng" {
		t.Errorf("expected To: chiamaka@example.ng, got %s", approvalEmail.To)
	}
	if !strings.Contains(approvalEmail.Subject, "Solex Business Grant — Application Approved") {
		t.Errorf("subject missing expected title, got: %s", approvalEmail.Subject)
	}

	body := approvalEmail.Body
	// Full legal name
	if !strings.Contains(body, "Chiamaka Nze") {
		t.Errorf("body missing applicant full legal name")
	}
	// Programme name
	if !strings.Contains(body, "Solex Business Grant Programme") {
		t.Errorf("body missing programme name")
	}
	// Reference code
	if !strings.Contains(body, app.ReferenceCode) && !strings.Contains(body, "SOLEX-") {
		t.Errorf("body missing application reference code %s", app.ReferenceCode)
	}
	// Amount
	if !strings.Contains(body, "250,000") {
		t.Errorf("body missing grant amount ₦250,000")
	}
	// Batch
	if !strings.Contains(body, "Batch A") {
		t.Errorf("body missing assigned batch: Batch A")
	}
	// Scheduled date
	if !strings.Contains(body, "2026-12-18") && !strings.Contains(body, "18 December 2026") {
		t.Errorf("body missing scheduled disbursement date")
	}
	// Banking processing disclaimer
	if !strings.Contains(body, "subject to standard banking processing") {
		t.Errorf("body missing banking processing disclaimer")
	}
	// Dashboard login instructions
	if !strings.Contains(body, "applicant dashboard") {
		t.Errorf("body missing instructions to access applicant dashboard")
	}

	// 2. PRIVACY & SECURITY AUDIT: strictly ensure no sensitive info leaked
	forbiddenSensitive := []string{
		"12345678901",       // applicant's NIN
		"Password123#",      // password
		"password_hash",     // hash
		"0123456789",        // full bank account number
		"passport_photo",    // uploaded doc path
		"solex_session",     // token name
	}
	for _, forbidden := range forbiddenSensitive {
		if strings.Contains(body, forbidden) {
			t.Errorf("SECURITY LEAK: email body contains sensitive info %q", forbidden)
		}
		if strings.Contains(approvalEmail.Subject, forbidden) {
			t.Errorf("SECURITY LEAK: email subject contains sensitive info %q", forbidden)
		}
	}
}

func TestDisbursement_EmailOutboxVsProductionDelivery(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Attach MockEmailSender to simulate production SMTP delivery
	mockSender := grants.NewMockEmailSender()
	s.SetEmailSender(mockSender)

	h := New(s, strings.Repeat("d", 40))
	adminToken, _ := setupTestAdmin(t, h)
	_, app := setupSubmittedApp(t, h, "Folake Ade", "folake@example.ng", "+2348044445566", "unregistered")
	passAdminChecks(t, h, adminToken, app.ID, "unregistered")

	// Approve
	w := request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, `{"disbursement_batch":"Batch A","disbursement_date":"2026-12-10"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("approval failed: %d %s", w.Code, w.Body.String())
	}

	// Check MockEmailSender received the message
	if mockSender.GetSendCount() < 1 {
		t.Fatalf("expected at least 1 email sent through MockEmailSender, got %d", mockSender.GetSendCount())
	}

	// Inspect email in DB: status should be 'sent'
	w = request(h, "GET", "/api/v1/admin/emails", adminToken, "")
	var emails []grants.EmailMessage
	_ = json.Unmarshal(w.Body.Bytes(), &emails)

	var sentEmail *grants.EmailMessage
	for _, em := range emails {
		if em.To == "folake@example.ng" && strings.Contains(em.Subject, "Approved") {
			sentEmail = &em
			break
		}
	}
	if sentEmail == nil {
		t.Fatalf("email not found in admin email outbox")
	}
	if sentEmail.Status != grants.EmailStatusSent {
		t.Errorf("expected email status to be 'sent', got %s", sentEmail.Status)
	}
	if sentEmail.SentAt == nil {
		t.Errorf("expected sent_at timestamp to be set")
	}
	if sentEmail.Attempts != 1 {
		t.Errorf("expected 1 delivery attempt, got %d", sentEmail.Attempts)
	}
}

func TestDisbursement_ApprovalResilientToEmailOutageAndRetryEndpoint(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	mockSender := grants.NewMockEmailSender()
	s.SetEmailSender(mockSender)

	h := New(s, strings.Repeat("e", 40))
	adminToken, _ := setupTestAdmin(t, h)
	_, app := setupSubmittedApp(t, h, "Ibrahim Danladi", "ibrahim@example.ng", "+2348055556677", "unregistered")
	passAdminChecks(t, h, adminToken, app.ID, "unregistered")

	// Set FailNext = 1 specifically for the approval email dispatch
	mockSender.SetFailNext(1)

	// 1. Approval MUST SUCCEED even if email delivery fails
	w := request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, `{"disbursement_batch":"Batch B","disbursement_date":"2026-12-20"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("approval should succeed despite email delivery error, got %d %s", w.Code, w.Body.String())
	}

	// Application is approved
	w = request(h, "GET", "/api/v1/admin/applications/"+app.ID, adminToken, "")
	var appObj grants.Application
	_ = json.Unmarshal(w.Body.Bytes(), &appObj)
	if appObj.Status != "approved" {
		t.Fatalf("application status expected approved, got %s", appObj.Status)
	}

	// 2. Email is stored with status 'failed'
	w = request(h, "GET", "/api/v1/admin/emails", adminToken, "")
	var emails []grants.EmailMessage
	_ = json.Unmarshal(w.Body.Bytes(), &emails)

	var failedEmail *grants.EmailMessage
	for _, em := range emails {
		if em.To == "ibrahim@example.ng" && strings.Contains(em.Subject, "Approved") {
			failedEmail = &em
			break
		}
	}
	if failedEmail == nil {
		t.Fatalf("approval email not found for ibrahim")
	}
	if failedEmail.Status != grants.EmailStatusFailed {
		t.Errorf("expected email status 'failed', got %s", failedEmail.Status)
	}
	if failedEmail.LastError == "" {
		t.Errorf("expected LastError to be populated on failure")
	}

	// 3. Admin retries email delivery via POST /api/v1/admin/emails/{id}/retry
	// mockSender has no more FailNext, so retry should succeed!
	w = request(h, "POST", "/api/v1/admin/emails/"+failedEmail.ID+"/retry", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("email retry failed: %d %s", w.Code, w.Body.String())
	}

	// 4. Verify email status is now 'sent'
	w = request(h, "GET", "/api/v1/admin/emails", adminToken, "")
	var retriedEmails []grants.EmailMessage
	_ = json.Unmarshal(w.Body.Bytes(), &retriedEmails)
	for _, em := range retriedEmails {
		if em.ID == failedEmail.ID {
			if em.Status != grants.EmailStatusSent {
				t.Errorf("expected retried email status 'sent', got %s", em.Status)
			}
			if em.LastError != "" {
				t.Errorf("expected LastError cleared, got %s", em.LastError)
			}
			break
		}
	}
}

func TestDisbursement_AdminUpdateScheduleWithAuditAndEmail(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	mockSender := grants.NewMockEmailSender()
	s.SetEmailSender(mockSender)

	h := New(s, strings.Repeat("f", 40))
	adminToken, _ := setupTestAdmin(t, h)
	_, app := setupSubmittedApp(t, h, "Zainab Bello", "zainab@example.ng", "+2348066667788", "unregistered")
	passAdminChecks(t, h, adminToken, app.ID, "unregistered")

	// Approve into Batch A
	w := request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, `{"disbursement_batch":"Batch A","disbursement_date":"2026-12-12"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("approval failed: %d %s", w.Code, w.Body.String())
	}

	// 1. Reschedule validation: missing reason
	w = request(h, "PUT", "/api/v1/admin/applications/"+app.ID+"/disbursement-schedule", adminToken, `{"disbursement_batch":"Batch B","disbursement_date":"2026-12-24","reason":""}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty reason, got %d %s", w.Code, w.Body.String())
	}

	// 2. Reschedule validation: date outside December 2026
	w = request(h, "PUT", "/api/v1/admin/applications/"+app.ID+"/disbursement-schedule", adminToken, `{"disbursement_batch":"Batch B","disbursement_date":"2027-01-10","reason":"Operational reschedule"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for 2027 date, got %d %s", w.Code, w.Body.String())
	}

	// 3. Reschedule success: move to Batch B on 2026-12-24
	reschedPayload := `{"disbursement_batch":"Batch B","disbursement_date":"2026-12-24","reason":"Reassigned due to bank processing schedule"}`
	w = request(h, "PUT", "/api/v1/admin/applications/"+app.ID+"/disbursement-schedule", adminToken, reschedPayload)
	if w.Code != http.StatusOK {
		t.Fatalf("reschedule failed: %d %s", w.Code, w.Body.String())
	}

	// 4. Verify updated application
	w = request(h, "GET", "/api/v1/admin/applications/"+app.ID, adminToken, "")
	var updatedApp grants.Application
	_ = json.Unmarshal(w.Body.Bytes(), &updatedApp)
	if updatedApp.DisbursementBatch != "Batch B" {
		t.Errorf("expected updated batch 'Batch B', got %s", updatedApp.DisbursementBatch)
	}
	if updatedApp.DisbursementDate != "2026-12-24" {
		t.Errorf("expected updated date 2026-12-24, got %v", updatedApp.DisbursementDate)
	}

	// 5. Verify audit event
	w = request(h, "GET", "/api/v1/admin/audit", adminToken, "")
	var auditEvents []grants.Event
	_ = json.Unmarshal(w.Body.Bytes(), &auditEvents)
	var foundReschedAudit bool
	for _, ev := range auditEvents {
		if ev.Action == "disbursement_schedule_updated" && ev.ApplicationID == app.ID {
			foundReschedAudit = true
			if ev.Actor != "admin" {
				t.Errorf("expected actor 'admin', got %s", ev.Actor)
			}
			if !strings.Contains(ev.Details, "Reassigned due to bank processing schedule") {
				t.Errorf("expected audit details to include reason, got %s", ev.Details)
			}
			break
		}
	}
	if !foundReschedAudit {
		t.Errorf("audit event for disbursement_schedule_updated not found in audit trail")
	}

	// 6. Verify schedule change notification email queued/sent
	w = request(h, "GET", "/api/v1/admin/emails", adminToken, "")
	var emails []grants.EmailMessage
	_ = json.Unmarshal(w.Body.Bytes(), &emails)
	var foundUpdateEmail bool
	for _, em := range emails {
		if em.To == "zainab@example.ng" && strings.Contains(em.Subject, "Schedule Update") {
			foundUpdateEmail = true
			if !strings.Contains(em.Body, "Batch B") {
				t.Errorf("reschedule email missing new batch 'Batch B'")
			}
			if !strings.Contains(em.Body, "2026-12-24") && !strings.Contains(em.Body, "24 December 2026") {
				t.Errorf("reschedule email missing new date '2026-12-24'")
			}
			if !strings.Contains(em.Body, "Reassigned due to bank processing schedule") {
				t.Errorf("reschedule email missing reason")
			}
			break
		}
	}
	if !foundUpdateEmail {
		t.Errorf("notification email for schedule change not found in outbox")
	}
}

func TestDisbursement_BatchFilteringAndMetrics(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	h := New(s, strings.Repeat("g", 40))
	adminToken, _ := setupTestAdmin(t, h)

	// App 1: Unregistered (250k) -> Batch A
	_, app1 := setupSubmittedApp(t, h, "Applicant One", "app1@example.ng", "+2348077771122", "unregistered")
	passAdminChecks(t, h, adminToken, app1.ID, "unregistered")
	w := request(h, "POST", "/api/v1/admin/applications/"+app1.ID+"/approve", adminToken, `{"disbursement_batch":"Batch A","disbursement_date":"2026-12-11"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("approve app1 failed: %d %s", w.Code, w.Body.String())
	}

	// App 2: Registered (500k) -> Batch B
	_, app2 := setupSubmittedApp(t, h, "Applicant Two", "app2@example.ng", "+2348077772233", "registered")
	passAdminChecks(t, h, adminToken, app2.ID, "registered")
	w = request(h, "POST", "/api/v1/admin/applications/"+app2.ID+"/approve", adminToken, `{"disbursement_batch":"Batch B","disbursement_date":"2026-12-21"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("approve app2 failed: %d %s", w.Code, w.Body.String())
	}

	// 1. Filter by Batch A
	w = request(h, "GET", "/api/v1/admin/applications?batch=Batch+A", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("filter batch A failed: %d %s", w.Code, w.Body.String())
	}
	var batchAList []grants.Application
	_ = json.Unmarshal(w.Body.Bytes(), &batchAList)
	if len(batchAList) != 1 || batchAList[0].ID != app1.ID {
		t.Errorf("batch A filter expected only app1, got %d items", len(batchAList))
	}

	// 2. Filter by Batch B
	w = request(h, "GET", "/api/v1/admin/applications?batch=Batch+B", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("filter batch B failed: %d %s", w.Code, w.Body.String())
	}
	var batchBList []grants.Application
	_ = json.Unmarshal(w.Body.Bytes(), &batchBList)
	if len(batchBList) != 1 || batchBList[0].ID != app2.ID {
		t.Errorf("batch B filter expected only app2, got %d items", len(batchBList))
	}

	// 3. Metrics verification
	w = request(h, "GET", "/api/v1/admin/metrics", adminToken, "")
	var m grants.AdminMetrics
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	if m.BatchACount != 1 {
		t.Errorf("expected BatchACount 1, got %d", m.BatchACount)
	}
	if m.BatchAAmountKobo != 25000000 {
		t.Errorf("expected BatchAAmountKobo 25,000,000, got %d", m.BatchAAmountKobo)
	}
	if m.BatchBCount != 1 {
		t.Errorf("expected BatchBCount 1, got %d", m.BatchBCount)
	}
	if m.BatchBAmountKobo != 50000000 {
		t.Errorf("expected BatchBAmountKobo 50,000,000, got %d", m.BatchBAmountKobo)
	}
}

func TestDisbursement_DuplicateApprovalAndEmailPrevention(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	h := New(s, strings.Repeat("h", 40))
	adminToken, _ := setupTestAdmin(t, h)
	_, app := setupSubmittedApp(t, h, "Kelechi Obi", "kelechi@example.ng", "+2348088889900", "unregistered")
	passAdminChecks(t, h, adminToken, app.ID, "unregistered")

	// First approval succeeds
	w := request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, `{"disbursement_batch":"Batch A","disbursement_date":"2026-12-15"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("first approval failed: %d %s", w.Code, w.Body.String())
	}

	// Count emails after first approval
	w = request(h, "GET", "/api/v1/admin/emails", adminToken, "")
	var initialEmails []grants.EmailMessage
	_ = json.Unmarshal(w.Body.Bytes(), &initialEmails)
	initialCount := len(initialEmails)

	// Second approval attempt must return 409 Conflict
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, `{"disbursement_batch":"Batch A","disbursement_date":"2026-12-15"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for duplicate approval, got %d %s", w.Code, w.Body.String())
	}

	// Verify no duplicate approval email was queued
	w = request(h, "GET", "/api/v1/admin/emails", adminToken, "")
	var postEmails []grants.EmailMessage
	_ = json.Unmarshal(w.Body.Bytes(), &postEmails)
	if len(postEmails) != initialCount {
		t.Errorf("duplicate approval created additional emails: was %d, now %d", initialCount, len(postEmails))
	}
}

func TestDisbursement_ApplicantDashboardDisplay(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 10)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	h := New(s, strings.Repeat("i", 40))
	adminToken, _ := setupTestAdmin(t, h)
	appToken, app := setupSubmittedApp(t, h, "Damilola Coker", "damilola@example.ng", "+2348099990011", "unregistered")
	passAdminChecks(t, h, adminToken, app.ID, "unregistered")

	// Approve into Batch A
	w := request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/approve", adminToken, `{"disbursement_batch":"Batch A","disbursement_date":"2026-12-14"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("approval failed: %d %s", w.Code, w.Body.String())
	}

	// Applicant views dashboard / current application
	w = request(h, "GET", "/api/v1/applications/current", appToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get current app failed: %d %s", w.Code, w.Body.String())
	}
	var currentApp grants.Application
	_ = json.Unmarshal(w.Body.Bytes(), &currentApp)

	if currentApp.Status != "approved" {
		t.Errorf("expected status 'approved', got %s", currentApp.Status)
	}
	if currentApp.DisbursementBatch != "Batch A" {
		t.Errorf("expected Batch A, got %s", currentApp.DisbursementBatch)
	}
	if currentApp.DisbursementDate != "2026-12-14" {
		t.Errorf("expected 2026-12-14 disbursement date, got %v", currentApp.DisbursementDate)
	}
	// Payment status MUST NOT be 'disbursed' merely because approved/scheduled!
	if currentApp.DisbursementStatus == "disbursed" {
		t.Errorf("payment status must not be 'disbursed' prior to explicit bank disbursement")
	}
	if currentApp.DisbursementStatus != "scheduled" {
		t.Errorf("expected payment status 'scheduled', got %s", currentApp.DisbursementStatus)
	}

	// Explicit disbursement by admin
	w = request(h, "POST", "/api/v1/admin/applications/"+app.ID+"/disburse", adminToken, `{"reference":"DISB-2026-0001"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("admin disburse failed: %d %s", w.Code, w.Body.String())
	}

	// Verify applicant now sees disbursed status
	w = request(h, "GET", "/api/v1/applications/current", appToken, "")
	_ = json.Unmarshal(w.Body.Bytes(), &currentApp)
	if currentApp.Status != "disbursed" && currentApp.DisbursementStatus != "disbursed" {
		t.Errorf("expected disbursement status 'disbursed' after explicit disbursement, got status=%s, disb_status=%s", currentApp.Status, currentApp.DisbursementStatus)
	}
}
