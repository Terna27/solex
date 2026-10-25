package grants

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestService(t *testing.T, capacity int) *Service {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "grants.json"), capacity)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time {
		return time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	}
	return s
}

func sampleProfile(category string) ApplicantProfile {
	p := ApplicantProfile{
		FullName:            "Terna Peter",
		DOB:                 "1992-06-15",
		Phone:               "+2348012345678",
		Email:               "terna@example.ng",
		NIN:                 "12345678901",
		StateOfOrigin:       "Benue",
		LGA:                 "Makurdi",
		ResidentialAddress:  "Plot 14 High Level, Makurdi",
		BusinessCategory:    category,
		BusinessName:        "Terna Global Ventures",
		BusinessType:        "Retail shop",
		BusinessDescription: "Trading of agricultural products and food items.",
		BusinessAddress:     "Shop 4 Wurukum Modern Market, Makurdi",
		StateOfOperation:    "Benue",
		LGAOfOperation:      "Makurdi",
		OperatingHistory:    "3-5 years",
		GrantPurpose:        "expansion",
		IntendedUse:         "Bulk purchase of grain inventory and digital POS equipment.",
		BankName:            "Access Bank",
		AccountNumber:       "0123456789",
		AccountName:         "Terna Peter",
	}
	if category == CategoryRegistered {
		p.CACNumber = "RC-123456"
		p.RegisteredBusinessName = "Terna Global Ventures Limited"
	}
	return p
}

func TestApplicantRegistrationAndLogin(t *testing.T) {
	s := newTestService(t, 5)

	// Valid registration
	reg := RegisterInput{
		FullName:        "Amina Bello",
		Email:           "Amina.Bello@example.ng",
		Phone:           "08091234567", // Local format normalized to +2348091234567
		Password:        "SecureP@ss123",
		ConfirmPassword: "SecureP@ss123",
	}

	user, session, err := s.Register(reg)
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}
	if user.Email != "amina.bello@example.ng" {
		t.Fatalf("expected normalized email amina.bello@example.ng, got %s", user.Email)
	}
	if user.Phone != "+2348091234567" {
		t.Fatalf("expected normalized phone +2348091234567, got %s", user.Phone)
	}
	if session.ID == "" {
		t.Fatal("expected non-empty session token")
	}

	// Duplicate email prevention
	dupEmail := reg
	dupEmail.Phone = "+2348097654321"
	if _, _, err := s.Register(dupEmail); err != ErrDuplicateEmail {
		t.Fatalf("expected ErrDuplicateEmail, got %v", err)
	}

	// Duplicate phone prevention
	dupPhone := reg
	dupPhone.Email = "other@example.ng"
	if _, _, err := s.Register(dupPhone); err != ErrDuplicatePhone {
		t.Fatalf("expected ErrDuplicatePhone, got %v", err)
	}

	// Short password
	shortPw := reg
	shortPw.Email = "short@example.ng"
	shortPw.Phone = "+2348011112222"
	shortPw.Password = "short"
	shortPw.ConfirmPassword = "short"
	if _, _, err := s.Register(shortPw); err != ErrInvalidPassword {
		t.Fatalf("expected ErrInvalidPassword, got %v", err)
	}

	// Password mismatch
	mismatchPw := reg
	mismatchPw.Email = "mismatch@example.ng"
	mismatchPw.Phone = "+2348022223333"
	mismatchPw.Password = "SecureP@ss123"
	mismatchPw.ConfirmPassword = "DifferentP@ss123"
	if _, _, err := s.Register(mismatchPw); err != ErrPasswordMismatch {
		t.Fatalf("expected ErrPasswordMismatch, got %v", err)
	}

	// Login with correct credentials
	loggedUser, loginSession, err := s.Login(LoginInput{
		Email:    "amina.bello@example.ng",
		Password: "SecureP@ss123",
	})
	if err != nil {
		t.Fatalf("failed to log in: %v", err)
	}
	if loggedUser.ID != user.ID {
		t.Fatalf("expected user ID %s, got %s", user.ID, loggedUser.ID)
	}

	// Login with incorrect password (protection against enumeration)
	if _, _, err := s.Login(LoginInput{
		Email:    "amina.bello@example.ng",
		Password: "WrongPassword",
	}); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	// Login with nonexistent email
	if _, _, err := s.Login(LoginInput{
		Email:    "unknown@example.ng",
		Password: "SecureP@ss123",
	}); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	// Session authentication
	authUser, err := s.AuthenticateSession(loginSession.ID)
	if err != nil || authUser.ID != user.ID {
		t.Fatalf("session auth failed: %v", err)
	}

	// Logout
	if err := s.Logout(loginSession.ID); err != nil {
		t.Fatalf("logout failed: %v", err)
	}
	if _, err := s.AuthenticateSession(loginSession.ID); err != ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized after logout, got %v", err)
	}
}

func TestSessionExpiration(t *testing.T) {
	s := newTestService(t, 5)

	u, sess, err := s.Register(RegisterInput{
		FullName:        "Emeka Okafor",
		Email:           "emeka@example.ng",
		Phone:           "+2348033334444",
		Password:        "Password123!",
		ConfirmPassword: "Password123!",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Advance time past session expiry (24h)
	s.now = func() time.Time {
		return time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
	}

	if _, err := s.AuthenticateSession(sess.ID); err != ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized for expired session, got %v", err)
	}

	_ = u
}

func TestNINValidationAndCategories(t *testing.T) {
	s := newTestService(t, 5)

	u, _, err := s.Register(RegisterInput{
		FullName:        "Fatima Abubakar",
		Email:           "fatima@example.ng",
		Phone:           "+2348055556666",
		Password:        "Password123!",
		ConfirmPassword: "Password123!",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Invalid NIN (too short, letters, etc.)
	p := sampleProfile(CategoryUnregistered)
	p.UserID = u.ID
	for _, badNIN := range []string{"12345", "123456789012", "1234567890a", "           "} {
		p.NIN = badNIN
		if _, err := s.SaveProfile(u.ID, p); err != ErrInvalidNIN {
			t.Fatalf("expected ErrInvalidNIN for %q, got %v", badNIN, err)
		}
	}

	// Valid NIN (11 digits)
	p.NIN = "12345678901"
	saved, err := s.SaveProfile(u.ID, p)
	if err != nil {
		t.Fatalf("failed to save valid profile: %v", err)
	}
	if saved.NIN != "12345678901" {
		t.Fatalf("unexpected NIN: %s", saved.NIN)
	}

	// Registered business requirements: CAC number required
	regProf := sampleProfile(CategoryRegistered)
	regProf.UserID = u.ID
	regProf.CACNumber = "" // Missing CAC
	if _, err := s.SaveProfile(u.ID, regProf); err != ErrCACRequired {
		t.Fatalf("expected ErrCACRequired when CAC is missing, got %v", err)
	}

	regProf.CACNumber = "RC-998877"
	regProf.RegisteredBusinessName = "Fatima Enterprises Ltd"
	if _, err := s.SaveProfile(u.ID, regProf); err != nil {
		t.Fatalf("failed to save registered business profile: %v", err)
	}
}

func TestGrantAmountAndApplicationWorkflow(t *testing.T) {
	s := newTestService(t, 5)

	// User 1: Unregistered Business -> ₦250,000
	u1, _, _ := s.Register(RegisterInput{
		FullName:        "Adebayo Adeleke",
		Email:           "adebayo@example.ng",
		Phone:           "+2348077778888",
		Password:        "Password123!",
		ConfirmPassword: "Password123!",
	})
	p1 := sampleProfile(CategoryUnregistered)
	_, err := s.SaveProfile(u1.ID, p1)
	if err != nil {
		t.Fatal(err)
	}

	app1, err := s.StartApplication(u1.ID, CategoryUnregistered)
	if err != nil {
		t.Fatalf("start application failed: %v", err)
	}
	if app1.GrantAmountKobo != GrantAmountUnregisteredKobo {
		t.Fatalf("expected %d kobo (₦250k), got %d", GrantAmountUnregisteredKobo, app1.GrantAmountKobo)
	}
	if app1.RegistrationFeeKobo != RegistrationFeeKobo {
		t.Fatalf("expected ₦1,000 fee (%d kobo), got %d", RegistrationFeeKobo, app1.RegistrationFeeKobo)
	}

	// Duplicate active application prevention
	if _, err := s.StartApplication(u1.ID, CategoryUnregistered); err != ErrActiveAppExists {
		t.Fatalf("expected ErrActiveAppExists, got %v", err)
	}

	// User 2: Registered Business -> ₦500,000
	u2, _, _ := s.Register(RegisterInput{
		FullName:        "Chioma Eze",
		Email:           "chioma@example.ng",
		Phone:           "+2348088889999",
		Password:        "Password123!",
		ConfirmPassword: "Password123!",
	})
	p2 := sampleProfile(CategoryRegistered)
	_, err = s.SaveProfile(u2.ID, p2)
	if err != nil {
		t.Fatal(err)
	}

	app2, err := s.StartApplication(u2.ID, CategoryRegistered)
	if err != nil {
		t.Fatalf("start application failed: %v", err)
	}
	if app2.GrantAmountKobo != GrantAmountRegisteredKobo {
		t.Fatalf("expected %d kobo (₦500k), got %d", GrantAmountRegisteredKobo, app2.GrantAmountKobo)
	}

	// Add mock documents for app1
	mockImage := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'} // raw test
	app1.Documents = []Document{
		{ID: "doc1", Kind: DocPassportPhoto, MIME: "image/png", Size: 100},
		{ID: "doc2", Kind: DocGovernmentID, MIME: "image/png", Size: 100},
	}
	_ = s.mutate(func(db *database) error {
		db.Applications[app1.ID] = app1
		return nil
	})

	// Transition to awaiting payment
	app1, err = s.TransitionToAwaitingPayment(u1.ID, app1.ID)
	if err != nil {
		t.Fatalf("failed transition to awaiting payment: %v", err)
	}
	if app1.Status != StatusAwaitingPayment {
		t.Fatalf("expected awaiting_payment, got %s", app1.Status)
	}

	// Initialize payment
	pay, err := s.InitPayment(u1.ID, app1.ID)
	if err != nil {
		t.Fatalf("failed to init payment: %v", err)
	}
	if pay.AmountKobo != RegistrationFeeKobo {
		t.Fatalf("expected fee %d, got %d", RegistrationFeeKobo, pay.AmountKobo)
	}

	// Payment verification via sandbox simulator
	payVerified, appConfirmed, err := s.VerifyPayment(u1.ID, pay.Reference, "simulate_success", false)
	if err != nil {
		t.Fatalf("failed to verify payment: %v", err)
	}
	if payVerified.Status != PaymentStatusSuccessful {
		t.Fatalf("expected payment successful, got %s", payVerified.Status)
	}
	if appConfirmed.Status != StatusPaymentConfirmed {
		t.Fatalf("expected application payment_confirmed, got %s", appConfirmed.Status)
	}

	// Payment idempotency test
	payRepeat, _, err := s.VerifyPayment(u1.ID, pay.Reference, "simulate_success", false)
	if err != nil || payRepeat.Status != PaymentStatusSuccessful {
		t.Fatalf("idempotent verification failed: %v", err)
	}

	// Final submission
	appSubmitted, err := s.SubmitApplication(u1.ID, app1.ID)
	if err != nil {
		t.Fatalf("failed to submit application: %v", err)
	}
	if appSubmitted.Status != StatusSubmitted {
		t.Fatalf("expected submitted, got %s", appSubmitted.Status)
	}

	// Email queued check
	emails := s.AdminEmails()
	if len(emails) == 0 {
		t.Fatal("expected confirmation email to be queued")
	}
	if emails[0].To != appSubmitted.Profile.Email {
		t.Fatalf("expected email to %s, got %s", appSubmitted.Profile.Email, emails[0].To)
	}

	_ = mockImage
}

func TestAdminReviewApprovalAndDisbursement(t *testing.T) {
	s := newTestService(t, 2)

	u, _, _ := s.Register(RegisterInput{
		FullName:        "Babatunde Raji",
		Email:           "babatunde@example.ng",
		Phone:           "+2348099990000",
		Password:        "Password123!",
		ConfirmPassword: "Password123!",
	})
	p := sampleProfile(CategoryRegistered)
	_, _ = s.SaveProfile(u.ID, p)
	app, _ := s.StartApplication(u.ID, CategoryRegistered)

	// Attach all required documents for registered business
	app.Documents = []Document{
		{ID: "doc1", Kind: DocPassportPhoto, MIME: "image/jpeg", Size: 200},
		{ID: "doc2", Kind: DocGovernmentID, MIME: "image/jpeg", Size: 300},
		{ID: "doc3", Kind: DocCACCertificate, MIME: "image/jpeg", Size: 400},
	}
	app.Status = StatusPaymentConfirmed
	_ = s.mutate(func(db *database) error {
		db.Applications[app.ID] = app
		return nil
	})

	app, err := s.SubmitApplication(u.ID, app.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Admin checklist: missing CAC check blocks approval
	checks := ApplicationChecks{
		Phone:    true,
		Identity: true,
		Address:  true,
		Business: true,
		Bank:     true,
		CAC:      false, // Incomplete CAC check
	}
	if err := s.AdminSetChecks(app.ID, checks); err != nil {
		t.Fatal(err)
	}
	if err := s.AdminApprove(app.ID, "Batch A", "2026-12-10"); err == nil {
		t.Fatal("approved registered application without completed CAC check")
	}

	// Complete all checks including CAC
	checks.CAC = true
	if err := s.AdminSetChecks(app.ID, checks); err != nil {
		t.Fatal(err)
	}

	// Admin approve
	if err := s.AdminApprove(app.ID, "Batch A", "2026-12-10"); err != nil {
		t.Fatalf("admin approval failed: %v", err)
	}

	approvedApp, err := s.AdminGetApplication(app.ID)
	if err != nil {
		t.Fatal(err)
	}
	if approvedApp.Status != StatusApproved {
		t.Fatalf("expected approved, got %s", approvedApp.Status)
	}
	if approvedApp.GrantAmountKobo != GrantAmountRegisteredKobo {
		t.Fatalf("expected ₦500,000 (%d kobo), got %d", GrantAmountRegisteredKobo, approvedApp.GrantAmountKobo)
	}

	// Admin disburse
	if err := s.AdminDisburse(app.ID); err != nil {
		t.Fatalf("admin disbursement failed: %v", err)
	}

	disbursedApp, _ := s.AdminGetApplication(app.ID)
	if disbursedApp.Status != StatusDisbursed || disbursedApp.DisbursementStatus != "disbursed" {
		t.Fatalf("expected disbursed status, got %s / %s", disbursedApp.Status, disbursedApp.DisbursementStatus)
	}

	// Metrics validation
	m := s.AdminMetrics()
	if m.DisbursedApplications != 1 || m.ApprovedGrantCommitmentsKobo != GrantAmountRegisteredKobo {
		t.Fatalf("bad metrics: %+v", m)
	}

	// Export validation
	exports := s.AdminExport()
	if len(exports) != 1 {
		t.Fatalf("expected 1 export record, got %d", len(exports))
	}
	if exports[0].MaskedNIN != "*******8901" {
		t.Fatalf("expected masked NIN *******8901, got %s", exports[0].MaskedNIN)
	}
}

func TestSimplifiedFormAndDerivedAccountName(t *testing.T) {
	s := newTestService(t, 5)

	u, _, err := s.Register(RegisterInput{
		FullName:        "Zainab Ahmed",
		Email:           "zainab@example.ng",
		Phone:           "+2348099887766",
		Password:        "Password123#",
		ConfirmPassword: "Password123#",
	})
	if err != nil {
		t.Fatal(err)
	}

	p := sampleProfile(CategoryUnregistered)
	p.UserID = u.ID
	p.FullName = "Zainab Ahmed"

	// 1. Business description is not required (empty string works)
	p.BusinessDescription = ""

	// 2. Intended use of funds is not required (empty string works)
	p.IntendedUse = ""

	// 3. Purpose of grant is required and limited to valid dropdown options
	p.GrantPurpose = ""
	if _, err := s.SaveProfile(u.ID, p); err == nil {
		t.Fatalf("expected error for empty grant_purpose, got nil")
	}

	p.GrantPurpose = "arbitrary free-text value not in options"
	if _, err := s.SaveProfile(u.ID, p); err == nil {
		t.Fatalf("expected error for invalid grant_purpose option, got nil")
	}

	// Valid dropdown options must succeed
	for _, opt := range GrantPurposeOptions {
		p.GrantPurpose = opt.Value
		saved, err := s.SaveProfile(u.ID, p)
		if err != nil {
			t.Fatalf("expected valid grant_purpose %q to succeed, got %v", opt.Value, err)
		}
		if saved.GrantPurpose != opt.Value {
			t.Fatalf("expected %q, got %q", opt.Value, saved.GrantPurpose)
		}
	}

	// 4. Full name is required
	badProf := p
	badProf.FullName = "   "
	if err := ValidateApplicantProfile(badProf, s.now()); err == nil {
		t.Fatalf("expected error for empty FullName, got nil")
	}

	// 5 & 6. Bank account holder name is derived from applicant's legal name and cannot be spoofed
	p.FullName = "Zainab Ahmed"
	p.AccountName = "Mallam Malicious Spoof" // Spoofed account name
	saved, err := s.SaveProfile(u.ID, p)
	if err != nil {
		t.Fatal(err)
	}
	if saved.AccountName != "Zainab Ahmed" {
		t.Fatalf("expected derived account name %q, got %q (spoof was not overwritten)", "Zainab Ahmed", saved.AccountName)
	}

	// 7. Changing legal name updates the derived account name and user account
	p.FullName = "Zainab Ahmed Bello"
	saved, err = s.SaveProfile(u.ID, p)
	if err != nil {
		t.Fatal(err)
	}
	if saved.AccountName != "Zainab Ahmed Bello" {
		t.Fatalf("expected updated derived account name %q, got %q", "Zainab Ahmed Bello", saved.AccountName)
	}
	updatedUser, _ := s.GetUser(u.ID)
	if updatedUser.FullName != "Zainab Ahmed Bello" {
		t.Fatalf("expected user account full name %q, got %q", "Zainab Ahmed Bello", updatedUser.FullName)
	}

	// 8. Existing stored applications remain readable
	legacyApp := Application{
		ID:            "legacy-app-1",
		UserID:        u.ID,
		ReferenceCode: "LEGACY-001",
		Category:      CategoryUnregistered,
		Status:        StatusSubmitted,
		Profile: ApplicantProfile{
			FullName:            "Legacy Applicant",
			NIN:                 "11223344556",
			BusinessDescription: "Old business description from prior version.",
			GrantPurpose:        "Legacy free-text purpose of funds.",
			IntendedUse:         "Old intended use text.",
			BankName:            "Access Bank",
			AccountNumber:       "0123456789",
			AccountName:         "Legacy Applicant",
		},
	}
	_ = s.mutate(func(db *database) error {
		db.Applications[legacyApp.ID] = legacyApp
		return nil
	})

	loadedApp, err := s.AdminGetApplication("legacy-app-1")
	if err != nil {
		t.Fatalf("failed to read existing stored application: %v", err)
	}
	if loadedApp.Profile.BusinessDescription != "Old business description from prior version." {
		t.Fatalf("expected old business description preserved, got %q", loadedApp.Profile.BusinessDescription)
	}
	if loadedApp.Profile.GrantPurpose != "Legacy free-text purpose of funds." {
		t.Fatalf("expected old grant purpose preserved, got %q", loadedApp.Profile.GrantPurpose)
	}

	// FormatGrantPurpose handles legacy free-text gracefully
	formatted := FormatGrantPurpose(loadedApp.Profile.GrantPurpose)
	if formatted != "Legacy free-text purpose of funds." {
		t.Fatalf("expected legacy purpose formatted as-is, got %q", formatted)
	}
	formattedValid := FormatGrantPurpose("working_capital")
	if formattedValid != "Business working capital" {
		t.Fatalf("expected %q, got %q", "Business working capital", formattedValid)
	}
}
