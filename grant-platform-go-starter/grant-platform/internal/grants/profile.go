package grants

import (
	"errors"
	"regexp"
	"strings"
)

var accountPattern = regexp.MustCompile(`^[0-9]{10}$`)

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
	if !accountPattern.MatchString(in.AccountNumber) {
		return errors.New("account_number must contain exactly 10 digits")
	}
	return nil
}

type Profile struct {
	ResidentialAddress string `json:"residential_address"`
	BusinessName       string `json:"business_name"`
	BusinessAddress    string `json:"business_address"`
	BankName           string `json:"bank_name"`
	AccountName        string `json:"account_name"`
	AccountNumber      string `json:"account_number"`
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
		db.Events = append(db.Events, Event{id, "profile_updated", s.now().UTC()})
		return nil
	})
}
