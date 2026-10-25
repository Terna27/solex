package grants

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var phonePattern = regexp.MustCompile(`^\+234[789][01][0-9]{8}$`)

type Input struct {
	FullName            string `json:"full_name"`
	DOB                 string `json:"date_of_birth"`
	Phone               string `json:"phone"`
	State               string `json:"state"`
	LGA                 string `json:"lga"`
	BusinessType        string `json:"business_type"`
	BusinessDescription string `json:"business_description"`
	Consent             bool   `json:"consent"`
	ResidentialAddress  string `json:"residential_address"`
	BusinessName        string `json:"business_name"`
	BusinessAddress     string `json:"business_address"`
	BankName            string `json:"bank_name"`
	AccountName         string `json:"account_name"`
	AccountNumber       string `json:"account_number"`
}

type Checks struct {
	Phone    bool `json:"phone"`
	Identity bool `json:"identity_and_age"`
	Address  bool `json:"address"`
	Business bool `json:"business"`
	Bank     bool `json:"bank_account_ownership"`
}

func (c Checks) Complete() bool {
	return c.Phone && c.Identity && c.Address && c.Business && c.Bank
}

type Instalment struct {
	Number     int    `json:"number"`
	AmountKobo int64  `json:"amount_kobo"`
	DueDate    string `json:"due_date"`
	Status     string `json:"status"`
}

type Notification struct {
	ID            string `json:"id"`
	ApplicationID string `json:"application_id"`
	Channel       string `json:"channel"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}

type database struct {
	Version       int                         `json:"version"`
	Users         map[string]User             `json:"users"`
	Sessions      map[string]Session          `json:"sessions"`
	AdminSessions map[string]AdminSession     `json:"admin_sessions,omitempty"`
	Profiles      map[string]ApplicantProfile `json:"profiles"`
	Applications  map[string]Application      `json:"applications"`
	Payments      map[string]Payment          `json:"payments"`
	TokenHashes   map[string]string           `json:"token_hashes"` // legacy
	Events        []Event                     `json:"events"`
	Outbox        []Notification              `json:"outbox"`
	Emails        []EmailMessage              `json:"emails"`
}

type Service struct {
	mu          sync.Mutex
	path        string
	capacity    int
	db          database
	now         func() time.Time
	emailSender EmailSender
}

func New(path string, capacity int) (*Service, error) {
	if capacity < 1 {
		return nil, errors.New("capacity must be positive")
	}
	s := &Service{
		path:     path,
		capacity: capacity,
		now:      time.Now,
		db: database{
			Version:      2,
			Users:         map[string]User{},
			Sessions:      map[string]Session{},
			AdminSessions: map[string]AdminSession{},
			Profiles:      map[string]ApplicantProfile{},
			Applications: map[string]Application{},
			Payments:     map[string]Payment{},
			TokenHashes:  map[string]string{},
			Events:       []Event{},
			Outbox:       []Notification{},
			Emails:       []EmailMessage{},
		},
	}

	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err == nil {
			if err = json.Unmarshal(b, &s.db); err != nil {
				return nil, err
			}
			if s.db.Users == nil {
				s.db.Users = map[string]User{}
			}
			if s.db.Sessions == nil {
				s.db.Sessions = map[string]Session{}
			}
			if s.db.AdminSessions == nil {
				s.db.AdminSessions = map[string]AdminSession{}
			}
			if s.db.Profiles == nil {
				s.db.Profiles = map[string]ApplicantProfile{}
			}
			if s.db.Applications == nil {
				s.db.Applications = map[string]Application{}
			}
			if s.db.Payments == nil {
				s.db.Payments = map[string]Payment{}
			}
			if s.db.TokenHashes == nil {
				s.db.TokenHashes = map[string]string{}
			}
			if s.db.Emails == nil {
				s.db.Emails = []EmailMessage{}
			}

			// Migration to schema v2
			if s.db.Version < 2 {
				s.db.Version = 2
				for id, a := range s.db.Applications {
					if a.Category == "" {
						a.Category = CategoryUnregistered
						a.GrantAmountKobo = GrantAmountUnregisteredKobo
						a.RegistrationFeeKobo = RegistrationFeeKobo
					}
					s.db.Applications[id] = a
				}
				_ = s.mutate(func(db *database) error { return nil })
			}
		}
	}

	// Default email sender: uses environment or development outbox
	s.emailSender = NewEmailSenderFromEnv()

	return s, nil
}

func (s *Service) EmailSender() EmailSender {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.emailSender
}

func (s *Service) SetEmailSender(sender EmailSender) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emailSender = sender
}

func randomID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (s *Service) mutate(fn func(*database) error) error {
	b, err := json.Marshal(s.db)
	if err != nil {
		return ErrStorage
	}
	var next database
	if json.Unmarshal(b, &next) != nil {
		return ErrStorage
	}
	if err = fn(&next); err != nil {
		return err
	}
	if s.path != "" {
		b, err = json.MarshalIndent(next, "", "  ")
		if err != nil {
			return ErrStorage
		}
		if err = os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
			return ErrStorage
		}
		f, err := os.CreateTemp(filepath.Dir(s.path), ".grants-*")
		if err != nil {
			return ErrStorage
		}
		defer os.Remove(f.Name())
		if _, err = f.Write(b); err != nil {
			f.Close()
			return ErrStorage
		}
		if err = f.Sync(); err != nil {
			f.Close()
			return ErrStorage
		}
		if err = f.Close(); err != nil {
			return ErrStorage
		}
		if err = os.Rename(f.Name(), s.path); err != nil {
			return ErrStorage
		}
	}
	s.db = next
	return nil
}

func day(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// ---------------- USER & AUTH METHODS ----------------

func (s *Service) Register(in RegisterInput) (User, Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fullName := strings.TrimSpace(in.FullName)
	if len(fullName) < 3 || len(fullName) > 150 {
		return User{}, Session{}, errors.New("full name must contain 3 to 150 characters")
	}

	email, err := NormalizeEmail(in.Email)
	if err != nil {
		return User{}, Session{}, err
	}

	phone, err := NormalizePhone(in.Phone)
	if err != nil {
		return User{}, Session{}, err
	}

	if err := ValidatePassword(in.Password, in.ConfirmPassword); err != nil {
		return User{}, Session{}, err
	}

	for _, u := range s.db.Users {
		if u.Email == email {
			return User{}, Session{}, ErrDuplicateEmail
		}
		if u.Phone == phone {
			return User{}, Session{}, ErrDuplicatePhone
		}
	}

	pwHash, err := HashPassword(in.Password)
	if err != nil {
		return User{}, Session{}, ErrStorage
	}

	userID, err := randomID()
	if err != nil {
		return User{}, Session{}, ErrStorage
	}
	sessionID, err := generateSecureToken(32)
	if err != nil {
		return User{}, Session{}, ErrStorage
	}

	now := s.now().UTC()
	user := User{
		ID:           userID,
		FullName:     fullName,
		Email:        email,
		Phone:        phone,
		PasswordHash: pwHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	session := Session{
		ID:        sessionID,
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(DefaultSessionDurationHours * time.Hour),
	}

	err = s.mutate(func(db *database) error {
		db.Users[userID] = user
		db.Sessions[sessionID] = session
		db.Events = append(db.Events, Event{
			UserID: userID,
			Actor:  "user:" + userID,
			Action: "registered",
			At:     now,
		})
		return nil
	})
	if err != nil {
		return User{}, Session{}, err
	}

	return user, session, nil
}

func (s *Service) Login(in LoginInput) (User, Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	email, err := NormalizeEmail(in.Email)
	if err != nil {
		return User{}, Session{}, ErrInvalidCredentials
	}

	var matchedUser *User
	for _, u := range s.db.Users {
		if u.Email == email {
			userCopy := u
			matchedUser = &userCopy
			break
		}
	}

	if matchedUser == nil {
		return User{}, Session{}, ErrInvalidCredentials
	}

	if !CheckPassword(matchedUser.PasswordHash, in.Password) {
		return User{}, Session{}, ErrInvalidCredentials
	}

	sessionID, err := generateSecureToken(32)
	if err != nil {
		return User{}, Session{}, ErrStorage
	}

	now := s.now().UTC()
	session := Session{
		ID:        sessionID,
		UserID:    matchedUser.ID,
		CreatedAt: now,
		ExpiresAt: now.Add(DefaultSessionDurationHours * time.Hour),
	}

	err = s.mutate(func(db *database) error {
		db.Sessions[sessionID] = session
		db.Events = append(db.Events, Event{
			UserID: matchedUser.ID,
			Actor:  "user:" + matchedUser.ID,
			Action: "logged_in",
			At:     now,
		})
		return nil
	})
	if err != nil {
		return User{}, Session{}, err
	}

	return *matchedUser, session, nil
}

func (s *Service) Logout(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sessionID == "" {
		return nil
	}

	return s.mutate(func(db *database) error {
		sess, ok := db.Sessions[sessionID]
		if ok {
			delete(db.Sessions, sessionID)
			db.Events = append(db.Events, Event{
				UserID: sess.UserID,
				Actor:  "user:" + sess.UserID,
				Action: "logged_out",
				At:     s.now().UTC(),
			})
		}
		return nil
	})
}

func (s *Service) AuthenticateSession(sessionID string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sessionID == "" {
		return User{}, ErrUnauthorized
	}

	sess, ok := s.db.Sessions[sessionID]
	if !ok || sess.IsExpired(s.now().UTC()) {
		return User{}, ErrUnauthorized
	}

	u, ok := s.db.Users[sess.UserID]
	if !ok {
		return User{}, ErrUnauthorized
	}

	return u, nil
}

func (s *Service) GetUser(userID string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.db.Users[userID]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}

// ---------------- ADMINISTRATOR SESSIONS ----------------

func (s *Service) CreateAdminSession(email string) (AdminSession, error) {
	token, err := generateSecureToken(32)
	if err != nil {
		return AdminSession{}, err
	}

	now := s.now().UTC()
	sess := AdminSession{
		ID:        token,
		Email:     email,
		CreatedAt: now,
		ExpiresAt: now.Add(12 * time.Hour),
	}

	err = s.mutate(func(db *database) error {
		if db.AdminSessions == nil {
			db.AdminSessions = map[string]AdminSession{}
		}
		db.AdminSessions[sess.ID] = sess
		db.Events = append(db.Events, Event{
			Actor:  "admin:" + email,
			Action: "admin_logged_in",
			At:     now,
		})
		return nil
	})
	if err != nil {
		return AdminSession{}, err
	}
	return sess, nil
}

func (s *Service) AuthenticateAdminSession(sessionID string) (AdminSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sessionID == "" {
		return AdminSession{}, ErrUnauthorized
	}

	if s.db.AdminSessions == nil {
		return AdminSession{}, ErrUnauthorized
	}

	sess, ok := s.db.AdminSessions[sessionID]
	if !ok || sess.IsExpired(s.now().UTC()) {
		return AdminSession{}, ErrUnauthorized
	}

	return sess, nil
}

func (s *Service) DeleteAdminSession(sessionID string) error {
	now := s.now().UTC()
	return s.mutate(func(db *database) error {
		if db.AdminSessions != nil {
			sess, ok := db.AdminSessions[sessionID]
			if ok {
				delete(db.AdminSessions, sessionID)
				db.Events = append(db.Events, Event{
					Actor:  "admin:" + sess.Email,
					Action: "admin_logged_out",
					At:     now,
				})
			}
		}
		return nil
	})
}

// ---------------- SECURE PASSWORD WORKFLOWS ----------------

func (s *Service) AdminResetApplicantPassword(applicationID string) (string, User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.db.Applications[applicationID]
	if !ok {
		return "", User{}, ErrNotFound
	}
	if a.UserID == "" {
		return "", User{}, errors.New("application has no associated applicant user account")
	}

	u, ok := s.db.Users[a.UserID]
	if !ok {
		return "", User{}, ErrUserNotFound
	}

	tempPass, err := generateSecurePassword(12)
	if err != nil {
		return "", User{}, err
	}

	hash, err := HashPassword(tempPass)
	if err != nil {
		return "", User{}, err
	}

	now := s.now().UTC()
	err = s.mutate(func(db *database) error {
		u.PasswordHash = hash
		u.UpdatedAt = now
		db.Users[u.ID] = u

		// Invalidate all active sessions for this applicant
		for sessID, sess := range db.Sessions {
			if sess.UserID == u.ID {
				delete(db.Sessions, sessID)
			}
		}

		// Security audit log (NEVER store password or hash)
		db.Events = append(db.Events, Event{
			ApplicationID: applicationID,
			UserID:        u.ID,
			Actor:         "admin",
			Action:        "admin_reset_applicant_password",
			Details:       fmt.Sprintf("Administrator reset password for applicant %s (%s)", u.FullName, u.Email),
			At:            now,
		})

		// Queue email notification
		emailMsg := EmailMessage{
			ID:        fmt.Sprintf("EML-RESET-%d", now.UnixNano()),
			To:        u.Email,
			Subject:   "Solex Business Grant — Security Notice: Password Reset",
			Body:      fmt.Sprintf("Hello %s,\n\nYour account password has been reset by an administrator.\n\nYour temporary password is: %s\n\nPlease log in at https://solex.ng/#login and immediately change your password.\n\nBest regards,\nSolex Operations Desk", u.FullName, tempPass),
			CreatedAt: now,
			SentAt:    &now,
			Status:    "saved_locally",
		}
		db.Emails = append(db.Emails, emailMsg)
		db.Outbox = append(db.Outbox, Notification{
			ID:            emailMsg.ID,
			ApplicationID: applicationID,
			Channel:       "email",
			Status:        "queued",
			Message:       "Password reset notification queued for " + u.Email,
		})

		return nil
	})
	if err != nil {
		return "", User{}, err
	}

	return tempPass, u, nil
}

func (s *Service) ChangePassword(userID, currentPassword, newPassword, confirmPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.db.Users[userID]
	if !ok {
		return ErrUserNotFound
	}

	if !CheckPassword(u.PasswordHash, currentPassword) {
		return errors.New("current password is incorrect")
	}

	if err := ValidatePassword(newPassword, confirmPassword); err != nil {
		return err
	}

	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	now := s.now().UTC()
	return s.mutate(func(db *database) error {
		u.PasswordHash = newHash
		u.UpdatedAt = now
		db.Users[u.ID] = u

		db.Events = append(db.Events, Event{
			UserID:  u.ID,
			Actor:   "user:" + u.ID,
			Action:  "password_changed",
			Details: "Applicant updated account password",
			At:      now,
		})
		return nil
	})
}

// ---------------- APPLICANT PROFILE METHODS ----------------

func (s *Service) GetProfile(userID string) (ApplicantProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.db.Profiles[userID]
	if !ok {
		u, ok := s.db.Users[userID]
		if ok {
			return ApplicantProfile{
				UserID:   userID,
				FullName: u.FullName,
				Email:    u.Email,
				Phone:    u.Phone,
			}, nil
		}
		return ApplicantProfile{}, ErrUserNotFound
	}
	return p, nil
}

func (s *Service) SaveProfile(userID string, p ApplicantProfile) (ApplicantProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.db.Users[userID]
	if !ok {
		return ApplicantProfile{}, ErrUserNotFound
	}

	p.UserID = userID
	p.FullName = strings.TrimSpace(p.FullName)
	if p.FullName == "" {
		p.FullName = u.FullName
	}
	if p.Email == "" {
		p.Email = u.Email
	}
	if p.Phone == "" {
		p.Phone = u.Phone
	}

	// Derive account holder name from applicant's full legal name
	p.AccountName = p.FullName

	if err := ValidateApplicantProfile(p, s.now()); err != nil {
		return ApplicantProfile{}, err
	}

	// Duplicate CAC check across all profiles and applications
	if p.BusinessCategory == CategoryRegistered && p.CACNumber != "" {
		for otherUserID, otherProf := range s.db.Profiles {
			if otherUserID != userID && strings.EqualFold(otherProf.CACNumber, p.CACNumber) {
				return ApplicantProfile{}, ErrDuplicateCAC
			}
		}
	}

	now := s.now().UTC()
	p.UpdatedAt = now

	err := s.mutate(func(db *database) error {
		if p.FullName != "" && p.FullName != u.FullName {
			u.FullName = p.FullName
			u.UpdatedAt = now
			db.Users[userID] = u
		}

		db.Profiles[userID] = p

		// Sync with existing draft or awaiting_payment application if present
		for appID, app := range db.Applications {
			if app.UserID == userID && (app.Status == StatusDraft || app.Status == StatusAwaitingPayment) {
				app.Profile = p
				app.Category = p.BusinessCategory
				grantAmt, _ := CalculateGrantAmount(p.BusinessCategory)
				app.GrantAmountKobo = grantAmt
				app.UpdatedAt = now
				db.Applications[appID] = app
			}
		}

		db.Events = append(db.Events, Event{
			UserID: userID,
			Actor:  "user:" + userID,
			Action: "profile_saved",
			At:     now,
		})
		return nil
	})
	if err != nil {
		return ApplicantProfile{}, err
	}

	return p, nil
}

// ---------------- APPLICATION METHODS ----------------

func (s *Service) GetCurrentApplication(userID string) (Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, a := range s.db.Applications {
		if a.UserID == userID {
			return a, nil
		}
	}
	return Application{}, ErrNotFound
}

func (s *Service) StartApplication(userID, category string) (Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Prevent duplicate applications
	for _, a := range s.db.Applications {
		if a.UserID == userID && a.Status != StatusRejected {
			return a, ErrActiveAppExists
		}
	}

	prof, ok := s.db.Profiles[userID]
	if !ok || prof.NIN == "" {
		return Application{}, ErrProfileIncomplete
	}

	if category == "" {
		category = prof.BusinessCategory
	}
	if category != CategoryUnregistered && category != CategoryRegistered {
		category = CategoryUnregistered
	}

	if category == CategoryRegistered && (prof.CACNumber == "" || prof.RegisteredBusinessName == "") {
		return Application{}, ErrCACRequired
	}

	grantAmt, err := CalculateGrantAmount(category)
	if err != nil {
		return Application{}, err
	}

	appID, err := randomID()
	if err != nil {
		return Application{}, ErrStorage
	}

	refCode, err := generateReferenceCode()
	if err != nil {
		return Application{}, ErrStorage
	}

	now := s.now().UTC()
	app := Application{
		ID:                  appID,
		UserID:              userID,
		ReferenceCode:       refCode,
		Category:            category,
		GrantAmountKobo:     grantAmt,
		RegistrationFeeKobo: RegistrationFeeKobo,
		Status:              StatusDraft,
		Profile:             prof,
		Checks:              ApplicationChecks{},
		Documents:           []Document{},
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	err = s.mutate(func(db *database) error {
		db.Applications[appID] = app
		db.Events = append(db.Events, Event{
			ApplicationID: appID,
			UserID:        userID,
			Actor:         "user:" + userID,
			Action:        "application_started",
			At:            now,
		})
		return nil
	})
	if err != nil {
		return Application{}, err
	}

	return app, nil
}

func (s *Service) TransitionToAwaitingPayment(userID, appID string) (Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.db.Applications[appID]
	if !ok || app.UserID != userID {
		return Application{}, ErrNotFound
	}

	if app.Status != StatusDraft {
		return app, ErrConflict
	}

	okDocs, msg := HasRequiredDocuments(app.Documents, app.Category)
	if !okDocs {
		return Application{}, errors.New("cannot proceed to payment: " + msg)
	}

	now := s.now().UTC()
	app.Status = StatusAwaitingPayment
	app.UpdatedAt = now

	err := s.mutate(func(db *database) error {
		db.Applications[appID] = app
		db.Events = append(db.Events, Event{
			ApplicationID: appID,
			UserID:        userID,
			Actor:         "user:" + userID,
			Action:        "state_changed_awaiting_payment",
			At:            now,
		})
		return nil
	})
	if err != nil {
		return Application{}, err
	}

	return app, nil
}

func (s *Service) SubmitApplication(userID, appID string) (Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.db.Applications[appID]
	if !ok || app.UserID != userID {
		return Application{}, ErrNotFound
	}

	if app.Status != StatusPaymentConfirmed {
		if app.Status == StatusSubmitted {
			return app, nil // Idempotent return
		}
		return Application{}, ErrPaymentRequired
	}

	okDocs, msg := HasRequiredDocuments(app.Documents, app.Category)
	if !okDocs {
		return Application{}, errors.New("cannot submit application: " + msg)
	}

	now := s.now().UTC()
	app.Status = StatusSubmitted
	app.UpdatedAt = now

	// Prepare email confirmation
	subject, body := FormatSubmissionConfirmationEmail(
		app.Profile.FullName,
		app.ReferenceCode,
		app.Category,
		app.GrantAmountKobo,
	)

	emailID, _ := randomID()
	emailMsg := EmailMessage{
		ID:        emailID,
		To:        app.Profile.Email,
		Subject:   subject,
		Body:      body,
		Status:    "saved_locally",
		CreatedAt: now,
	}

	err := s.mutate(func(db *database) error {
		db.Applications[appID] = app
		db.Emails = append(db.Emails, emailMsg)
		db.Events = append(db.Events, Event{
			ApplicationID: appID,
			UserID:        userID,
			Actor:         "user:" + userID,
			Action:        "application_submitted",
			Details:       "Reference: " + app.ReferenceCode,
			At:            now,
		})
		return nil
	})
	if err != nil {
		return Application{}, err
	}

	// Trigger sender in background if configured
	if s.emailSender != nil {
		go func() {
			_ = s.emailSender.Send(context.Background(), emailMsg)
		}()
	}

	return app, nil
}

// ---------------- PAYMENT METHODS ----------------

func (s *Service) InitPayment(userID, appID string) (Payment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.db.Applications[appID]
	if !ok || app.UserID != userID {
		return Payment{}, ErrNotFound
	}

	if app.Status != StatusDraft && app.Status != StatusAwaitingPayment && app.Status != StatusPaymentConfirmed {
		return Payment{}, ErrConflict
	}

	// Check if payment already exists
	for _, p := range s.db.Payments {
		if p.ApplicationID == appID {
			return p, nil
		}
	}

	payID, err := randomID()
	if err != nil {
		return Payment{}, ErrStorage
	}

	ref, err := generatePaymentReference()
	if err != nil {
		return Payment{}, ErrStorage
	}

	now := s.now().UTC()
	payment := Payment{
		ID:            payID,
		ApplicationID: appID,
		UserID:        userID,
		Reference:     ref,
		AmountKobo:    RegistrationFeeKobo,
		Currency:      Currency,
		Status:        PaymentStatusPending,
		Provider:      "sandbox",
		CreatedAt:     now,
	}

	err = s.mutate(func(db *database) error {
		db.Payments[payID] = payment
		a := db.Applications[appID]
		a.PaymentID = payID
		if a.Status == StatusDraft {
			a.Status = StatusAwaitingPayment
		}
		a.UpdatedAt = now
		db.Applications[appID] = a
		db.Events = append(db.Events, Event{
			ApplicationID: appID,
			UserID:        userID,
			Actor:         "user:" + userID,
			Action:        "payment_initiated",
			Details:       "Reference: " + ref,
			At:            now,
		})
		return nil
	})
	if err != nil {
		return Payment{}, err
	}

	return payment, nil
}

func (s *Service) VerifyPayment(userID, ref, simulatorAction string, isProduction bool) (Payment, Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var payment *Payment
	for _, p := range s.db.Payments {
		if p.Reference == ref {
			pCopy := p
			payment = &pCopy
			break
		}
	}

	if payment == nil {
		return Payment{}, Application{}, ErrPaymentNotFound
	}

	if payment.UserID != userID {
		return Payment{}, Application{}, ErrUnauthorized
	}

	app, ok := s.db.Applications[payment.ApplicationID]
	if !ok {
		return Payment{}, Application{}, ErrNotFound
	}

	// Idempotency: if already successful, return current state
	if payment.Status == PaymentStatusSuccessful {
		return *payment, app, nil
	}

	// Sandbox simulation handling
	if simulatorAction != "" {
		if isProduction {
			return Payment{}, Application{}, ErrSandboxNotAllowed
		}
		if simulatorAction == "simulate_failure" {
			now := s.now().UTC()
			payment.Status = PaymentStatusFailed
			err := s.mutate(func(db *database) error {
				db.Payments[payment.ID] = *payment
				db.Events = append(db.Events, Event{
					ApplicationID: app.ID,
					UserID:        userID,
					Actor:         "user:" + userID,
					Action:        "payment_failed_sandbox",
					Details:       "Reference: " + ref,
					At:            now,
				})
				return nil
			})
			return *payment, app, err
		}
	}

	// Mark payment successful
	now := s.now().UTC()
	payment.Status = PaymentStatusSuccessful
	payment.PaidAt = &now
	payment.ProviderReference = "SBX-" + ref[10:]

	app.Status = StatusPaymentConfirmed
	app.UpdatedAt = now

	err := s.mutate(func(db *database) error {
		db.Payments[payment.ID] = *payment
		db.Applications[app.ID] = app
		db.Events = append(db.Events, Event{
			ApplicationID: app.ID,
			UserID:        userID,
			Actor:         "user:" + userID,
			Action:        "payment_verified",
			Details:       fmt.Sprintf("Reference: %s, Amount: %d kobo", ref, payment.AmountKobo),
			At:            now,
		})
		return nil
	})
	if err != nil {
		return Payment{}, Application{}, err
	}

	return *payment, app, nil
}

// ---------------- ADMIN METHODS ----------------

func (s *Service) AdminMetrics() AdminMetrics {
	s.mu.Lock()
	defer s.mu.Unlock()

	var m AdminMetrics
	m.TotalUsers = len(s.db.Users)
	m.TotalApplications = len(s.db.Applications)

	for _, a := range s.db.Applications {
		if a.Category == CategoryRegistered {
			m.RegisteredBusinessApps++
		} else {
			m.UnregisteredBusinessApps++
		}

		switch a.Status {
		case StatusAwaitingPayment:
			m.ApplicationsAwaitingPayment++
		case StatusSubmitted:
			m.SubmittedApplications++
		case StatusUnderReview:
			m.ApplicationsUnderReview++
		case StatusApproved:
			m.ApprovedApplications++
			m.ApprovedGrantCommitmentsKobo += a.GrantAmountKobo
		case StatusRejected:
			m.RejectedApplications++
		case StatusDisbursed:
			m.DisbursedApplications++
			m.ApprovedGrantCommitmentsKobo += a.GrantAmountKobo
			m.RecordedDisbursementsKobo += a.GrantAmountKobo
		}

		if a.Status == StatusApproved || a.Status == StatusDisbursed {
			normBatch := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(a.DisbursementBatch, "_", " ")))
			if normBatch == "batch a" || normBatch == "a" {
				m.BatchACount++
				m.BatchAAmountKobo += a.GrantAmountKobo
			} else if normBatch == "batch b" || normBatch == "b" {
				m.BatchBCount++
				m.BatchBAmountKobo += a.GrantAmountKobo
			}
		}
	}

	for _, p := range s.db.Payments {
		if p.Status == PaymentStatusSuccessful {
			m.VerifiedFeesCollectedKobo += p.AmountKobo
		}
	}

	return m
}

func (s *Service) AdminListApplications(q, stateFilter, catFilter, statusFilter, batchFilter string) []Application {
	s.mu.Lock()
	defer s.mu.Unlock()

	q = strings.ToLower(strings.TrimSpace(q))
	batchFilter = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(batchFilter, "_", " ")))
	var out []Application

	for _, a := range s.db.Applications {
		if statusFilter != "" && a.Status != statusFilter {
			continue
		}
		if catFilter != "" && a.Category != catFilter {
			continue
		}
		if batchFilter != "" {
			aBatch := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(a.DisbursementBatch, "_", " ")))
			if aBatch != batchFilter {
				continue
			}
		}
		if stateFilter != "" && !strings.EqualFold(a.Profile.StateOfOperation, stateFilter) && !strings.EqualFold(a.Profile.StateOfOrigin, stateFilter) && !strings.EqualFold(a.State, stateFilter) {
			continue
		}
		if q != "" {
			combined := strings.ToLower(fmt.Sprintf("%s %s %s %s %s %s %s %s %s",
				a.Profile.FullName, a.FullName,
				a.Profile.Email, a.Phone,
				a.Profile.BusinessName, a.BusinessName,
				a.DisbursementBatch, a.DisbursementDate,
				a.ReferenceCode))
			if !strings.Contains(combined, q) {
				continue
			}
		}
		aCopy := a
		aCopy.Documents = append([]Document{}, a.Documents...)
		out = append(out, aCopy)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (s *Service) AdminGetApplication(id string) (Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.db.Applications[id]
	if !ok {
		return Application{}, ErrNotFound
	}
	a.Documents = append([]Document{}, a.Documents...)
	return a, nil
}

func (s *Service) AdminSetChecks(id string, checks ApplicationChecks) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.mutate(func(db *database) error {
		a, ok := db.Applications[id]
		if !ok {
			return ErrNotFound
		}
		if a.Status != StatusSubmitted && a.Status != StatusUnderReview {
			return ErrConflict
		}
		a.Checks = checks
		if a.Status == StatusSubmitted {
			a.Status = StatusUnderReview
		}
		a.UpdatedAt = s.now().UTC()
		db.Applications[id] = a
		db.Events = append(db.Events, Event{
			ApplicationID: id,
			Actor:         "admin",
			Action:        "admin_checks_updated",
			At:            s.now().UTC(),
		})
		return nil
	})
}

func (s *Service) AdminApprove(id, batch, disbursementDate string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanBatch, err := NormalizeBatch(batch)
	if err != nil {
		return err
	}

	dTime, err := ValidateDisbursementDate(disbursementDate)
	if err != nil {
		return err
	}
	formattedDate := dTime.Format("2006-01-02")

	var approvalEmail EmailMessage
	var shouldSendEmail bool

	err = s.mutate(func(db *database) error {
		a, ok := db.Applications[id]
		if !ok {
			return ErrNotFound
		}
		if a.Status != StatusSubmitted && a.Status != StatusUnderReview {
			return ErrConflict
		}

		if !a.Checks.Complete(a.Category) {
			return errors.New("all verification checks must be complete prior to approval")
		}

		okDocs, docMsg := HasRequiredDocuments(a.Documents, a.Category)
		if !okDocs {
			return errors.New("missing required documents: " + docMsg)
		}

		// Enforce beneficiary capacity
		approvedCount := 0
		for _, other := range db.Applications {
			if other.Status == StatusApproved || other.Status == StatusDisbursed {
				approvedCount++
			}
		}
		if approvedCount >= s.capacity {
			return ErrCapacity
		}

		now := s.now().UTC()
		a.Status = StatusApproved
		a.DisbursementBatch = cleanBatch
		a.DisbursementDate = formattedDate
		a.DisbursementStatus = "scheduled"
		a.UpdatedAt = now

		// Create single one-time instalment for legacy record representation
		a.Instalments = []Instalment{
			{
				Number:     1,
				AmountKobo: a.GrantAmountKobo,
				DueDate:    formattedDate,
				Status:     "scheduled",
			},
		}

		db.Applications[id] = a
		db.Events = append(db.Events, Event{
			ApplicationID: id,
			UserID:        a.UserID,
			Actor:         "admin",
			Action:        "approved",
			Details:       fmt.Sprintf("Category: %s, Amount: %d kobo, Batch: %s, Date: %s", a.Category, a.GrantAmountKobo, cleanBatch, formattedDate),
			At:            now,
		})

		// Idempotent email generation
		emailID := "EML-APPROVAL-" + id
		alreadyQueued := false
		for _, em := range db.Emails {
			if em.ID == emailID {
				alreadyQueued = true
				break
			}
		}

		if !alreadyQueued {
			subject, body := FormatApprovalEmail(
				a.Profile.FullName,
				a.ReferenceCode,
				a.Category,
				cleanBatch,
				formattedDate,
				a.GrantAmountKobo,
			)

			initialStatus := EmailStatusQueued
			if _, isDev := s.emailSender.(*DevelopmentOutboxSender); isDev {
				initialStatus = EmailStatusSavedLocally
			}

			approvalEmail = EmailMessage{
				ID:        emailID,
				To:        a.Profile.Email,
				Subject:   subject,
				Body:      body,
				Status:    initialStatus,
				CreatedAt: now,
			}
			db.Emails = append(db.Emails, approvalEmail)
			db.Outbox = append(db.Outbox, Notification{
				ID:            approvalEmail.ID,
				ApplicationID: id,
				Channel:       "email",
				Status:        initialStatus,
				Message:       fmt.Sprintf("Approval notification queued for %s (%s, %s, ₦%s)", a.Profile.Email, cleanBatch, formattedDate, formatNaira(a.GrantAmountKobo)),
			})
			shouldSendEmail = true
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Dispatch email asynchronously / safely without failing the approval transaction
	if shouldSendEmail && s.emailSender != nil {
		if _, isDev := s.emailSender.(*DevelopmentOutboxSender); !isDev {
			sendErr := s.emailSender.Send(context.Background(), approvalEmail)
			_ = s.mutate(func(db *database) error {
				for i, em := range db.Emails {
					if em.ID == approvalEmail.ID {
						if sendErr == nil {
							sentNow := s.now().UTC()
							db.Emails[i].Status = EmailStatusSent
							db.Emails[i].SentAt = &sentNow
							db.Emails[i].LastError = ""
							db.Emails[i].Attempts++
						} else {
							db.Emails[i].Status = EmailStatusFailed
							db.Emails[i].LastError = sendErr.Error()
							db.Emails[i].Attempts++
						}
						break
					}
				}
				return nil
			})
		}
	}

	return nil
}

func (s *Service) AdminUpdateDisbursementSchedule(id, batch, disbursementDate, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return errors.New("reason for schedule update is required")
	}

	cleanBatch, err := NormalizeBatch(batch)
	if err != nil {
		return err
	}

	dTime, err := ValidateDisbursementDate(disbursementDate)
	if err != nil {
		return err
	}
	formattedDate := dTime.Format("2006-01-02")

	var updateEmail EmailMessage
	var shouldSendEmail bool

	err = s.mutate(func(db *database) error {
		a, ok := db.Applications[id]
		if !ok {
			return ErrNotFound
		}
		if a.Status != StatusApproved {
			return errors.New("only approved applications can have their disbursement schedule updated")
		}

		oldBatch := a.DisbursementBatch
		oldDate := a.DisbursementDate
		if oldBatch == cleanBatch && oldDate == formattedDate {
			return nil // No material change
		}

		now := s.now().UTC()
		a.DisbursementBatch = cleanBatch
		a.DisbursementDate = formattedDate
		a.UpdatedAt = now
		if len(a.Instalments) > 0 {
			a.Instalments[0].DueDate = formattedDate
		}
		db.Applications[id] = a

		trimmedReason := strings.TrimSpace(reason)
		if trimmedReason == "" {
			trimmedReason = "Administrative batch / schedule adjustment"
		}

		db.Events = append(db.Events, Event{
			ApplicationID: id,
			UserID:        a.UserID,
			Actor:         "admin",
			Action:        "disbursement_schedule_updated",
			Details:       fmt.Sprintf("Updated from %s (%s) to %s (%s). Reason: %s", oldBatch, oldDate, cleanBatch, formattedDate, trimmedReason),
			At:            now,
		})

		// Queue email notice to applicant
		subject, body := FormatScheduleChangeEmail(
			a.Profile.FullName,
			a.ReferenceCode,
			cleanBatch,
			formattedDate,
			trimmedReason,
			a.GrantAmountKobo,
		)

		initialStatus := EmailStatusQueued
		if _, isDev := s.emailSender.(*DevelopmentOutboxSender); isDev {
			initialStatus = EmailStatusSavedLocally
		}

		updateEmail = EmailMessage{
			ID:        fmt.Sprintf("EML-SCHED-%s-%d", id, now.UnixNano()),
			To:        a.Profile.Email,
			Subject:   subject,
			Body:      body,
			Status:    initialStatus,
			CreatedAt: now,
		}
		db.Emails = append(db.Emails, updateEmail)
		db.Outbox = append(db.Outbox, Notification{
			ID:            updateEmail.ID,
			ApplicationID: id,
			Channel:       "email",
			Status:        initialStatus,
			Message:       fmt.Sprintf("Schedule update email queued for %s (%s -> %s, %s -> %s)", a.Profile.Email, oldBatch, cleanBatch, oldDate, formattedDate),
		})
		shouldSendEmail = true

		return nil
	})
	if err != nil {
		return err
	}

	// Dispatch email safely
	if shouldSendEmail && s.emailSender != nil {
		if _, isDev := s.emailSender.(*DevelopmentOutboxSender); !isDev {
			sendErr := s.emailSender.Send(context.Background(), updateEmail)
			_ = s.mutate(func(db *database) error {
				for i, em := range db.Emails {
					if em.ID == updateEmail.ID {
						if sendErr == nil {
							sentNow := s.now().UTC()
							db.Emails[i].Status = EmailStatusSent
							db.Emails[i].SentAt = &sentNow
							db.Emails[i].LastError = ""
							db.Emails[i].Attempts++
						} else {
							db.Emails[i].Status = EmailStatusFailed
							db.Emails[i].LastError = sendErr.Error()
							db.Emails[i].Attempts++
						}
						break
					}
				}
				return nil
			})
		}
	}

	return nil
}

func (s *Service) AdminRetryEmail(emailID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var targetEmail EmailMessage
	found := false
	for _, em := range s.db.Emails {
		if em.ID == emailID {
			targetEmail = em
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}

	if s.emailSender == nil {
		return errors.New("email sender not configured")
	}

	now := s.now().UTC()
	sendErr := s.emailSender.Send(context.Background(), targetEmail)

	_ = s.mutate(func(db *database) error {
		for i, em := range db.Emails {
			if em.ID == emailID {
				if sendErr == nil {
					db.Emails[i].Status = EmailStatusSent
					db.Emails[i].SentAt = &now
					db.Emails[i].LastError = ""
					db.Emails[i].Attempts++
				} else {
					db.Emails[i].Status = EmailStatusFailed
					db.Emails[i].LastError = sendErr.Error()
					db.Emails[i].Attempts++
				}
				break
			}
		}
		return nil
	})
	return sendErr
}

func (s *Service) AdminReject(id, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	reason = strings.TrimSpace(reason)
	if len(reason) < 5 || len(reason) > 500 {
		return errors.New("reason must contain 5 to 500 characters")
	}

	return s.mutate(func(db *database) error {
		a, ok := db.Applications[id]
		if !ok {
			return ErrNotFound
		}
		if a.Status != StatusSubmitted && a.Status != StatusUnderReview {
			return ErrConflict
		}
		now := s.now().UTC()
		a.Status = StatusRejected
		a.Reason = reason
		a.UpdatedAt = now
		db.Applications[id] = a
		db.Events = append(db.Events, Event{
			ApplicationID: id,
			Actor:         "admin",
			Action:        "rejected",
			Details:       "Reason: " + reason,
			At:            now,
		})
		return nil
	})
}

func (s *Service) AdminDisburse(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.mutate(func(db *database) error {
		a, ok := db.Applications[id]
		if !ok {
			return ErrNotFound
		}
		if a.Status != StatusApproved {
			return errors.New("only approved applications can be recorded as disbursed")
		}
		now := s.now().UTC()
		a.Status = StatusDisbursed
		a.DisbursementStatus = "disbursed"
		a.UpdatedAt = now
		if len(a.Instalments) > 0 {
			a.Instalments[0].Status = "paid"
		}
		db.Applications[id] = a
		db.Events = append(db.Events, Event{
			ApplicationID: id,
			Actor:         "admin",
			Action:        "disbursed",
			Details:       fmt.Sprintf("Amount: %d kobo", a.GrantAmountKobo),
			At:            now,
		})
		return nil
	})
}

func (s *Service) AdminExport() []ExportRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	var records []ExportRecord
	for _, a := range s.db.Applications {
		name := a.Profile.FullName
		if name == "" {
			name = a.FullName
		}
		phone := a.Profile.Phone
		if phone == "" {
			phone = a.Phone
		}
		email := a.Profile.Email
		state := a.Profile.StateOfOperation
		if state == "" {
			state = a.State
		}
		lga := a.Profile.LGAOfOperation
		if lga == "" {
			lga = a.LGA
		}
		bType := a.Profile.BusinessType
		if bType == "" {
			bType = a.BusinessType
		}

		records = append(records, ExportRecord{
			ApplicationID:      a.ID,
			ReferenceCode:      a.ReferenceCode,
			ApplicantName:      name,
			Phone:              phone,
			Email:              email,
			MaskedNIN:          MaskNIN(a.Profile.NIN),
			Category:           a.Category,
			CACNumber:          a.Profile.CACNumber,
			State:              state,
			LGA:                lga,
			BusinessType:       bType,
			Status:             a.Status,
			GrantAmountKobo:    a.GrantAmountKobo,
			PaymentStatus:      a.DisbursementStatus,
			DisbursementBatch:  a.DisbursementBatch,
			DisbursementDate:   a.DisbursementDate,
			DisbursementStatus: a.DisbursementStatus,
			CreatedAt:          a.CreatedAt.Format(time.RFC3339),
		})
	}
	return records
}

func (s *Service) AdminEmails() []EmailMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]EmailMessage{}, s.db.Emails...)
}

func (s *Service) Audit() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Event{}, s.db.Events...)
}

func (s *Service) Outbox() []Notification {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Notification{}, s.db.Outbox...)
}

// ---------------- LEGACY COMPATIBILITY METHODS ----------------

func validate(in Input, now time.Time) error {
	if err := validateProfile(in); err != nil {
		return err
	}
	if len(in.FullName) < 3 || len(in.FullName) > 150 {
		return errors.New("full_name must contain 3 to 150 characters")
	}
	if !phonePattern.MatchString(in.Phone) {
		return errors.New("phone must use Nigerian international format, such as +2348012345678")
	}
	dob, err := time.Parse("2006-01-02", in.DOB)
	if err != nil || dob.Before(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)) || dob.AddDate(18, 0, 0).After(day(now)) {
		return errors.New("a valid birth date and minimum age of 18 are required")
	}
	if !in.Consent {
		return errors.New("consent is required")
	}
	if !validState(in.State) {
		return errors.New("select a Nigerian state or FCT")
	}
	if len(in.LGA) < 2 || len(in.LGA) > 100 {
		return errors.New("lga must contain 2 to 100 characters")
	}
	if len(in.BusinessType) < 2 || len(in.BusinessType) > 80 {
		return errors.New("provide a valid business type")
	}
	if len(in.BusinessDescription) > 2000 {
		return errors.New("business description must not exceed 2000 characters")
	}
	return nil
}

func (s *Service) Create(in Input) (Application, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	in.FullName = strings.TrimSpace(in.FullName)
	if in.AccountName == "" && in.FullName != "" {
		in.AccountName = in.FullName
	}
	in.Phone = strings.TrimSpace(in.Phone)
	in.State = strings.TrimSpace(in.State)
	in.LGA = strings.TrimSpace(in.LGA)
	in.BusinessType = strings.TrimSpace(in.BusinessType)
	in.BusinessDescription = strings.TrimSpace(in.BusinessDescription)
	if err := validate(in, s.now()); err != nil {
		return Application{}, "", err
	}
	for _, a := range s.db.Applications {
		if a.Phone == in.Phone || (a.Profile.Phone != "" && a.Profile.Phone == in.Phone) {
			return Application{}, "", ErrConflict
		}
	}
	id, err := randomID()
	if err != nil {
		return Application{}, "", err
	}
	token, err := randomID()
	if err != nil {
		return Application{}, "", err
	}
	ref, _ := generateReferenceCode()
	a := Application{
		ID:                  id,
		ReferenceCode:       ref,
		Category:            CategoryUnregistered,
		GrantAmountKobo:     GrantAmountUnregisteredKobo,
		RegistrationFeeKobo: RegistrationFeeKobo,
		Input:               in,
		Status:              StatusSubmitted,
		CreatedAt:           s.now().UTC(),
		UpdatedAt:           s.now().UTC(),
		Instalments:         []Instalment{},
	}
	err = s.mutate(func(db *database) error {
		db.Applications[id] = a
		db.TokenHashes[id] = tokenHash(token)
		db.Events = append(db.Events, Event{
			ApplicationID: id,
			Actor:         "legacy_create",
			Action:        "submitted",
			At:            s.now().UTC(),
		})
		return nil
	})
	if err != nil {
		return Application{}, "", err
	}
	return a, token, nil
}

func (s *Service) Own(id, token string) (Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.db.Applications[id]
	if !ok || token == "" || s.db.TokenHashes[id] != tokenHash(token) {
		return Application{}, ErrNotFound
	}
	a.Instalments = append([]Instalment{}, a.Instalments...)
	a.Documents = append([]Document{}, a.Documents...)
	return a, nil
}

func (s *Service) List() []Application {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Application{}
	for _, a := range s.db.Applications {
		a.Instalments = append([]Instalment{}, a.Instalments...)
		a.Documents = append([]Document{}, a.Documents...)
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *Service) SetChecks(id string, c Checks) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mutate(func(db *database) error {
		a, ok := db.Applications[id]
		if !ok {
			return ErrNotFound
		}
		if a.Status != StatusSubmitted && a.Status != StatusUnderReview {
			return ErrConflict
		}
		a.Checks.Phone = c.Phone
		a.Checks.Identity = c.Identity
		a.Checks.Address = c.Address
		a.Checks.Business = c.Business
		a.Checks.Bank = c.Bank
		db.Applications[id] = a
		db.Events = append(db.Events, Event{
			ApplicationID: id,
			Actor:         "legacy_set_checks",
			Action:        "sandbox_checks_updated",
			At:            s.now().UTC(),
		})
		return nil
	})
}

func monthDate(first time.Time, offset int) time.Time {
	start := time.Date(first.Year(), first.Month()+time.Month(offset), 1, 0, 0, 0, 0, time.UTC)
	last := start.AddDate(0, 1, -1).Day()
	d := first.Day()
	if d > last {
		d = last
	}
	return start.AddDate(0, 0, d-1)
}

func (s *Service) Approve(id, firstDate string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	first, err := time.Parse("2006-01-02", firstDate)
	if err != nil || first.Before(day(s.now())) {
		return errors.New("first_due_date must be today or later")
	}
	return s.mutate(func(db *database) error {
		a, ok := db.Applications[id]
		if !ok {
			return ErrNotFound
		}
		if a.Status != StatusSubmitted && a.Status != StatusUnderReview {
			return ErrConflict
		}
		if !a.Checks.Complete(a.Category) {
			return errors.New("all verification checks must be complete")
		}
		if err := validateProfile(a.Input); err != nil && a.Profile.NIN == "" {
			return err
		}
		okDocs, docMsg := HasRequiredDocuments(a.Documents, a.Category)
		if !okDocs {
			return errors.New("passport photograph and an ID card or proof of address are required before approval: " + docMsg)
		}
		approved := 0
		for _, v := range db.Applications {
			if v.Status == StatusApproved || v.Status == StatusDisbursed {
				approved++
			}
		}
		if approved >= s.capacity {
			return ErrCapacity
		}
		if a.ReferenceCode == "" {
			code, err := randomID()
			if err != nil {
				return err
			}
			a.ReferenceCode = "GRANT-" + strings.ToUpper(code[:20])
		}
		a.Status = StatusApproved
		a.DisbursementDate = firstDate
		a.DisbursementStatus = "scheduled"
		a.Instalments = []Instalment{
			{Number: 1, AmountKobo: a.GrantAmountKobo, DueDate: firstDate, Status: "scheduled"},
		}
		db.Applications[id] = a
		db.Events = append(db.Events, Event{
			ApplicationID: id,
			Actor:         "legacy_approve",
			Action:        "approved",
			At:            s.now().UTC(),
		})
		db.Outbox = append(db.Outbox, Notification{
			ID:            a.ReferenceCode,
			ApplicationID: id,
			Channel:       "sms",
			Status:        "pending_provider_configuration",
			Message:       fmt.Sprintf("Your application is approved. Reference: %s. Amount: NGN %s. No fee is required. This reference does not authorise payment.", a.ReferenceCode, formatNaira(a.GrantAmountKobo)),
		})
		return nil
	})
}

func (s *Service) Reject(id, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	reason = strings.TrimSpace(reason)
	if len(reason) < 5 || len(reason) > 500 {
		return errors.New("reason must contain 5 to 500 characters")
	}
	return s.mutate(func(db *database) error {
		a, ok := db.Applications[id]
		if !ok {
			return ErrNotFound
		}
		if a.Status != StatusSubmitted && a.Status != StatusUnderReview {
			return ErrConflict
		}
		a.Status = StatusRejected
		a.Reason = reason
		db.Applications[id] = a
		db.Events = append(db.Events, Event{
			ApplicationID: id,
			Actor:         "legacy_reject",
			Action:        "rejected",
			At:            s.now().UTC(),
		})
		return nil
	})
}

func validState(name string) bool {
	for _, s := range States {
		if name == s {
			return true
		}
	}
	return false
}

var States = []string{
	"Abia", "Adamawa", "Akwa Ibom", "Anambra", "Bauchi", "Bayelsa", "Benue", "Borno",
	"Cross River", "Delta", "Ebonyi", "Edo", "Ekiti", "Enugu", "FCT", "Gombe", "Imo",
	"Jigawa", "Kaduna", "Kano", "Katsina", "Kebbi", "Kogi", "Kwara", "Lagos", "Nasarawa",
	"Niger", "Ogun", "Ondo", "Osun", "Oyo", "Plateau", "Rivers", "Sokoto", "Taraba",
	"Yobe", "Zamfara",
}
