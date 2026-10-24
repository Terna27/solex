package grants

import (
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

// Amounts are integer kobo. No applicant fees are collected.
const MonthlyKobo int64 = 7500000

var ErrNotFound = errors.New("application not found")
var ErrConflict = errors.New("application cannot be changed in its current state")
var ErrCapacity = errors.New("funded beneficiary capacity reached")
var ErrStorage = errors.New("could not save changes")
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

func (c Checks) Complete() bool { return c.Phone && c.Identity && c.Address && c.Business && c.Bank }

type Instalment struct {
	Number     int    `json:"number"`
	AmountKobo int64  `json:"amount_kobo"`
	DueDate    string `json:"due_date"`
	Status     string `json:"status"`
}
type Application struct {
	ID string `json:"id"`
	Input
	Status        string       `json:"status"`
	Checks        Checks       `json:"checks"`
	ReferenceCode string       `json:"reference_code,omitempty"`
	Reason        string       `json:"reason,omitempty"`
	Instalments   []Instalment `json:"instalments"`
	CreatedAt     time.Time    `json:"created_at"`
	Documents     []Document   `json:"documents"`
}
type Event struct {
	ApplicationID string    `json:"application_id"`
	Action        string    `json:"action"`
	At            time.Time `json:"at"`
}
type Notification struct {
	ID            string `json:"id"`
	ApplicationID string `json:"application_id"`
	Channel       string `json:"channel"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}
type database struct {
	Applications map[string]Application `json:"applications"`
	TokenHashes  map[string]string      `json:"token_hashes"`
	Events       []Event                `json:"events"`
	Outbox       []Notification         `json:"outbox"`
}
type Service struct {
	mu       sync.Mutex
	path     string
	capacity int
	db       database
	now      func() time.Time
}

func New(path string, capacity int) (*Service, error) {
	if capacity < 1 {
		return nil, errors.New("capacity must be positive")
	}
	s := &Service{path: path, capacity: capacity, now: time.Now, db: database{Applications: map[string]Application{}, TokenHashes: map[string]string{}, Events: []Event{}, Outbox: []Notification{}}}
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err == nil {
			if err = json.Unmarshal(b, &s.db); err != nil {
				return nil, err
			}
			if s.db.Applications == nil || s.db.TokenHashes == nil {
				return nil, errors.New("invalid data store")
			}
		}
	}
	return s, nil
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
	// Caller holds mutex. Commit in memory only after atomic file replacement succeeds.
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
func day(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC) }
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
	if len(in.BusinessType) < 2 || len(in.BusinessType) > 80 || len(in.BusinessDescription) < 10 || len(in.BusinessDescription) > 2000 {
		return errors.New("provide a business type and a description of 10 to 2000 characters")
	}
	return nil
}
func (s *Service) Create(in Input) (Application, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	in.FullName = strings.TrimSpace(in.FullName)
	in.Phone = strings.TrimSpace(in.Phone)
	in.State = strings.TrimSpace(in.State)
	in.LGA = strings.TrimSpace(in.LGA)
	in.BusinessType = strings.TrimSpace(in.BusinessType)
	in.BusinessDescription = strings.TrimSpace(in.BusinessDescription)
	if err := validate(in, s.now()); err != nil {
		return Application{}, "", err
	}
	for _, a := range s.db.Applications {
		if a.Phone == in.Phone {
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
	a := Application{ID: id, Input: in, Status: "submitted", CreatedAt: s.now().UTC(), Instalments: []Instalment{}}
	err = s.mutate(func(db *database) error {
		db.Applications[id] = a
		db.TokenHashes[id] = tokenHash(token)
		db.Events = append(db.Events, Event{id, "submitted", s.now().UTC()})
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
		if a.Status != "submitted" {
			return ErrConflict
		}
		a.Checks = c
		db.Applications[id] = a
		db.Events = append(db.Events, Event{id, "sandbox_checks_updated", s.now().UTC()})
		return nil
	})
}

// monthDate clamps to the last valid day (Jan 31 -> Feb 28 -> Mar 31).
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
		if a.Status != "submitted" {
			return ErrConflict
		}
		if !a.Checks.Complete() {
			return errors.New("all verification checks must be complete")
		}
		if err := validateProfile(a.Input); err != nil {
			return err
		}
		passport, support := false, false
		for _, d := range a.Documents {
			if d.Kind == "passport_photo" {
				passport = true
			}
			if d.Kind == "government_id" || d.Kind == "proof_of_address" {
				support = true
			}
		}
		if !passport || !support {
			return errors.New("passport photograph and an ID card or proof of address are required before approval")
		}
		approved := 0
		for _, v := range db.Applications {
			if v.Status == "approved" {
				approved++
			}
		}
		if approved >= s.capacity {
			return ErrCapacity
		}
		code, err := randomID()
		if err != nil {
			return err
		}
		a.ReferenceCode = "GRANT-" + strings.ToUpper(code[:20])
		a.Status = "approved"
		for i := 0; i < 3; i++ {
			a.Instalments = append(a.Instalments, Instalment{i + 1, MonthlyKobo, monthDate(first, i).Format("2006-01-02"), "scheduled"})
		}
		db.Applications[id] = a
		db.Events = append(db.Events, Event{id, "approved", s.now().UTC()})
		db.Outbox = append(db.Outbox, Notification{ID: code, ApplicationID: id, Channel: "sms", Status: "pending_provider_configuration", Message: fmt.Sprintf("Your application is approved. Reference: %s. NGN 75,000 per month for 3 months; first scheduled date %s. No fee is required. This reference does not authorise payment.", a.ReferenceCode, firstDate)})
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
		if a.Status != "submitted" {
			return ErrConflict
		}
		a.Status = "rejected"
		a.Reason = reason
		db.Applications[id] = a
		db.Events = append(db.Events, Event{id, "rejected", s.now().UTC()})
		return nil
	})
}
func (s *Service) Outbox() []Notification {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Notification{}, s.db.Outbox...)
}
func (s *Service) Audit() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Event{}, s.db.Events...)
}
func validState(name string) bool {
	for _, s := range States {
		if name == s {
			return true
		}
	}
	return false
}

var States = []string{"Abia", "Adamawa", "Akwa Ibom", "Anambra", "Bauchi", "Bayelsa", "Benue", "Borno", "Cross River", "Delta", "Ebonyi", "Edo", "Ekiti", "Enugu", "FCT", "Gombe", "Imo", "Jigawa", "Kaduna", "Kano", "Katsina", "Kebbi", "Kogi", "Kwara", "Lagos", "Nasarawa", "Niger", "Ogun", "Ondo", "Osun", "Oyo", "Plateau", "Rivers", "Sokoto", "Taraba", "Yobe", "Zamfara"}
