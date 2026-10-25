package grants

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// Banks is the maintained catalogue of supported deposit-money banks for this prototype.
// Review against the CBN register before production use; fintech wallets (OPay, PalmPay) are excluded.
var Banks = []string{
	"Access Bank", "Citibank Nigeria", "Ecobank Nigeria", "Fidelity Bank", "First Bank of Nigeria",
	"First City Monument Bank", "Globus Bank", "Guaranty Trust Bank", "Keystone Bank", "Optimus Bank",
	"Parallex Bank", "Polaris Bank", "PremiumTrust Bank", "Providus Bank", "Signature Bank",
	"Stanbic IBTC Bank", "Standard Chartered Bank", "Sterling Bank", "SunTrust Bank", "Titan Trust Bank",
	"Union Bank", "United Bank for Africa", "Unity Bank", "Wema Bank", "Zenith Bank",
}

func validBank(name string) bool {
	for _, bank := range Banks {
		if name == bank {
			return true
		}
	}
	return false
}

var (
	accountPattern = regexp.MustCompile(`^[0-9]{10}$`)
	ninPattern     = regexp.MustCompile(`^[0-9]{11}$`)
)

func validateProfile(in Input) error {
	for _, field := range []struct {
		name, value string
		min, max    int
	}{
		{"residential_address", in.ResidentialAddress, 10, 500},
		{"business_name", in.BusinessName, 2, 150},
		{"business_address", in.BusinessAddress, 5, 500},
		{"bank_name", in.BankName, 2, 100},
		{"account_name", in.AccountName, 3, 150},
	} {
		v := strings.TrimSpace(field.value)
		if len(v) < field.min || len(v) > field.max {
			return errors.New("provide a valid " + field.name)
		}
	}
	if !validBank(strings.TrimSpace(in.BankName)) {
		return ErrInvalidBank
	}
	if !accountPattern.MatchString(in.AccountNumber) {
		return ErrInvalidAccount
	}
	return nil
}

// Legacy Profile structure
type Profile struct {
	ResidentialAddress string `json:"residential_address"`
	BusinessName       string `json:"business_name"`
	BusinessAddress    string `json:"business_address"`
	BankName           string `json:"bank_name"`
	AccountName        string `json:"account_name"`
	AccountNumber      string `json:"account_number"`
}

// ApplicantProfile represents the full profile for registered accounts.
type ApplicantProfile struct {
	UserID                 string    `json:"user_id"`
	FullName               string    `json:"full_name"`
	DOB                    string    `json:"date_of_birth"`
	Phone                  string    `json:"phone"`
	Email                  string    `json:"email"`
	NIN                    string    `json:"nin"`
	StateOfOrigin          string    `json:"state_of_origin"`
	LGA                    string    `json:"lga"`
	ResidentialAddress     string    `json:"residential_address"`
	BusinessCategory       string    `json:"business_category"` // "unregistered" or "registered"
	BusinessName           string    `json:"business_name"`
	BusinessType           string    `json:"business_type"`
	BusinessDescription    string    `json:"business_description,omitempty"`
	BusinessAddress        string    `json:"business_address"`
	StateOfOperation       string    `json:"state_of_operation"`
	LGAOfOperation         string    `json:"lga_of_operation"`
	OperatingHistory       string    `json:"operating_history"`
	GrantPurpose           string    `json:"grant_purpose"`
	IntendedUse            string    `json:"intended_use,omitempty"`
	CACNumber              string    `json:"cac_number,omitempty"`
	RegisteredBusinessName string    `json:"registered_business_name,omitempty"`
	BankName               string    `json:"bank_name"`
	AccountNumber          string    `json:"account_number"`
	AccountName            string    `json:"account_name"`
	UpdatedAt              time.Time `json:"updated_at"`
}

func ValidateApplicantProfile(p ApplicantProfile, now time.Time) error {
	p.FullName = strings.TrimSpace(p.FullName)
	if len(p.FullName) < 3 || len(p.FullName) > 150 {
		return errors.New("full legal name must contain 3 to 150 characters")
	}

	dob, err := time.Parse("2006-01-02", p.DOB)
	if err != nil || dob.Before(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)) || dob.AddDate(18, 0, 0).After(day(now)) {
		return errors.New("a valid birth date and minimum age of 18 are required")
	}

	if _, err := NormalizeEmail(p.Email); err != nil {
		return err
	}
	if _, err := NormalizePhone(p.Phone); err != nil {
		return err
	}

	p.NIN = strings.TrimSpace(p.NIN)
	if !ninPattern.MatchString(p.NIN) {
		return ErrInvalidNIN
	}

	if !validState(strings.TrimSpace(p.StateOfOrigin)) {
		return errors.New("select a valid Nigerian state of origin")
	}

	p.LGA = strings.TrimSpace(p.LGA)
	if len(p.LGA) < 2 || len(p.LGA) > 100 {
		return errors.New("lga must contain 2 to 100 characters")
	}

	p.ResidentialAddress = strings.TrimSpace(p.ResidentialAddress)
	if len(p.ResidentialAddress) < 10 || len(p.ResidentialAddress) > 500 {
		return errors.New("residential address must contain 10 to 500 characters")
	}

	// Business validation
	cat := strings.TrimSpace(p.BusinessCategory)
	if cat != CategoryUnregistered && cat != CategoryRegistered {
		return errors.New("business category must be either 'unregistered' or 'registered'")
	}

	p.BusinessName = strings.TrimSpace(p.BusinessName)
	if len(p.BusinessName) < 2 || len(p.BusinessName) > 150 {
		return errors.New("business name must contain 2 to 150 characters")
	}

	p.BusinessType = strings.TrimSpace(p.BusinessType)
	if len(p.BusinessType) < 2 || len(p.BusinessType) > 80 {
		return errors.New("select or enter a valid business type")
	}

	p.BusinessDescription = strings.TrimSpace(p.BusinessDescription)
	if len(p.BusinessDescription) > 2000 {
		return errors.New("business description must not exceed 2000 characters")
	}

	p.BusinessAddress = strings.TrimSpace(p.BusinessAddress)
	if len(p.BusinessAddress) < 5 || len(p.BusinessAddress) > 500 {
		return errors.New("business address must contain 5 to 500 characters")
	}

	if !validState(strings.TrimSpace(p.StateOfOperation)) {
		return errors.New("select a valid Nigerian state of operation")
	}

	p.LGAOfOperation = strings.TrimSpace(p.LGAOfOperation)
	if len(p.LGAOfOperation) < 2 || len(p.LGAOfOperation) > 100 {
		return errors.New("lga of operation must contain 2 to 100 characters")
	}

	p.OperatingHistory = strings.TrimSpace(p.OperatingHistory)
	if len(p.OperatingHistory) < 2 || len(p.OperatingHistory) > 100 {
		return errors.New("operating history must contain 2 to 100 characters")
	}

	p.GrantPurpose = strings.TrimSpace(p.GrantPurpose)
	if _, ok := ValidGrantPurposes[p.GrantPurpose]; !ok {
		return errors.New("select a valid purpose of grant from the options provided")
	}

	p.IntendedUse = strings.TrimSpace(p.IntendedUse)
	if len(p.IntendedUse) > 1000 {
		return errors.New("intended use of funds must not exceed 1000 characters")
	}

	if cat == CategoryRegistered {
		p.CACNumber = strings.TrimSpace(p.CACNumber)
		if len(p.CACNumber) < 4 || len(p.CACNumber) > 50 {
			return ErrCACRequired
		}
		p.RegisteredBusinessName = strings.TrimSpace(p.RegisteredBusinessName)
		if len(p.RegisteredBusinessName) < 2 || len(p.RegisteredBusinessName) > 150 {
			return errors.New("registered business name is required for registered businesses")
		}
	}

	// Bank validation
	p.BankName = strings.TrimSpace(p.BankName)
	if !validBank(p.BankName) {
		return ErrInvalidBank
	}

	p.AccountNumber = strings.TrimSpace(p.AccountNumber)
	if !accountPattern.MatchString(p.AccountNumber) {
		return ErrInvalidAccount
	}

	p.AccountName = strings.TrimSpace(p.AccountName)
	if len(p.AccountName) < 3 || len(p.AccountName) > 150 {
		return errors.New("account name must contain 3 to 150 characters")
	}

	return nil
}

func (s *Service) UpdateProfile(id, token string, p Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if token == "" || s.db.TokenHashes[id] != tokenHash(token) {
		return ErrNotFound
	}
	return s.mutate(func(db *database) error {
		a, ok := db.Applications[id]
		if !ok {
			return ErrNotFound
		}
		if a.Status != "submitted" {
			return ErrConflict
		}
		old := a.Input
		a.ResidentialAddress = strings.TrimSpace(p.ResidentialAddress)
		a.BusinessName = strings.TrimSpace(p.BusinessName)
		a.BusinessAddress = strings.TrimSpace(p.BusinessAddress)
		a.BankName = strings.TrimSpace(p.BankName)
		a.AccountName = strings.TrimSpace(p.AccountName)
		a.AccountNumber = strings.TrimSpace(p.AccountNumber)
		if err := validateProfile(a.Input); err != nil {
			return err
		}
		if a.ResidentialAddress != old.ResidentialAddress {
			a.Checks.Address = false
		}
		if a.BusinessName != old.BusinessName || a.BusinessAddress != old.BusinessAddress {
			a.Checks.Business = false
		}
		if a.BankName != old.BankName || a.AccountName != old.AccountName || a.AccountNumber != old.AccountNumber {
			a.Checks.Bank = false
		}
		db.Applications[id] = a
		db.Events = append(db.Events, Event{
			ApplicationID: id,
			Action:        "profile_updated",
			Actor:         "legacy_token",
			At:            s.now().UTC(),
		})
		return nil
	})
}
