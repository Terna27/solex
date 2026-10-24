package grants

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestProfileValidationAndOwnership(t *testing.T) {
	for _, number := range []string{"123", "12345678901", "012345678a", " 123456789"} {
		s, in := fixture(t, 1)
		in.AccountNumber = number
		if _, _, err := s.Create(in); err == nil {
			t.Fatalf("invalid account accepted: %q", number)
		}
	}
	s, in := fixture(t, 1)
	a, token, err := s.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetChecks(a.ID, allChecks()); err != nil {
		t.Fatal(err)
	}
	if err = s.Approve(a.ID, "2099-01-01"); err == nil {
		t.Fatal("approved without required documents")
	}
	p := Profile{ResidentialAddress: in.ResidentialAddress, BusinessName: in.BusinessName, BusinessAddress: in.BusinessAddress, BankName: in.BankName, AccountName: in.AccountName, AccountNumber: "0012345678"}
	if err = s.UpdateProfile(a.ID, "wrong", p); !errors.Is(err, ErrNotFound) {
		t.Fatal("unauthorised edit")
	}
	if err = s.UpdateProfile(a.ID, token, p); err != nil {
		t.Fatal(err)
	}
	got, err := s.Own(a.ID, token)
	if err != nil {
		t.Fatal(err)
	}
	if got.Checks.Bank || !got.Checks.Address || got.AccountNumber != "0012345678" {
		t.Fatal("bank edit failed to reset only bank check or lost leading zero")
	}
	p.ResidentialAddress = "99 New Sample Street"
	p.BusinessAddress = "99 New Sample Market"
	if err = s.UpdateProfile(a.ID, token, p); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Own(a.ID, token)
	if got.Checks.Address || got.Checks.Business {
		t.Fatal("address/business edits not invalidated")
	}
	addDocuments(t, s, a.ID, token)
	if err = s.SetChecks(a.ID, allChecks()); err != nil {
		t.Fatal(err)
	}
	if err = s.Approve(a.ID, "2099-01-01"); err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateProfile(a.ID, token, p); !errors.Is(err, ErrConflict) {
		t.Fatal("approved bank details mutable")
	}
}

func TestLegacyApplicationCanCompleteProfile(t *testing.T) {
	s, in := fixture(t, 1)
	a, token, err := s.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a persisted record created by the original prototype.
	s.mu.Lock()
	err = s.mutate(func(db *database) error {
		old := db.Applications[a.ID]
		old.BankName = ""
		old.AccountName = ""
		old.AccountNumber = ""
		old.ResidentialAddress = ""
		old.BusinessName = ""
		old.BusinessAddress = ""
		db.Applications[a.ID] = old
		return nil
	})
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := New(filepath.Clean(s.path), 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = restored.Own(a.ID, token); err != nil {
		t.Fatal("legacy record unreadable")
	}
	p := Profile{in.ResidentialAddress, in.BusinessName, in.BusinessAddress, in.BankName, in.AccountName, in.AccountNumber}
	if err = restored.UpdateProfile(a.ID, token, p); err != nil {
		t.Fatal(err)
	}
}
