package grants

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T, capacity int) (*Service, Input) {
	t.Helper()
	s, err := New("", capacity)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC) }
	return s, Input{FullName: "Example Applicant", DOB: "1990-05-20", Phone: "+2348012345678", State: "Benue", LGA: "Makurdi", BusinessType: "Retail", BusinessDescription: "A fictional neighbourhood shop for testing.", Consent: true}
}
func allChecks() Checks { return Checks{true, true, true, true, true} }
func TestAgeBoundary(t *testing.T) {
	for _, tc := range []struct {
		dob   string
		valid bool
	}{{"2008-10-24", true}, {"2008-10-25", false}, {"2020-01-01", false}, {"bad", false}, {"1899-01-01", false}} {
		t.Run(tc.dob, func(t *testing.T) {
			s, in := fixture(t, 1)
			in.DOB = tc.dob
			_, _, err := s.Create(in)
			if (err == nil) != tc.valid {
				t.Fatalf("date %s: %v", tc.dob, err)
			}
		})
	}
}
func TestApprovalAndMonthClamping(t *testing.T) {
	s, in := fixture(t, 1)
	a, token, err := s.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Approve(a.ID, "2027-01-31"); err == nil {
		t.Fatal("unverified application approved")
	}
	if err = s.SetChecks(a.ID, allChecks()); err != nil {
		t.Fatal(err)
	}
	if err = s.Approve(a.ID, "2027-01-31"); err != nil {
		t.Fatal(err)
	}
	a, err = s.Own(a.ID, token)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2027-01-31", "2027-02-28", "2027-03-31"}
	if len(a.Instalments) != 3 {
		t.Fatal("incorrect instalment count")
	}
	var total int64
	for i, p := range a.Instalments {
		total += p.AmountKobo
		if p.DueDate != want[i] || p.Status != "scheduled" {
			t.Fatalf("bad instalment: %+v", p)
		}
	}
	if total != 22500000 {
		t.Fatalf("wrong total %d", total)
	}
	if a.ReferenceCode == "" {
		t.Fatal("missing reference")
	}
	if err = s.Approve(a.ID, "2027-01-31"); !errors.Is(err, ErrConflict) {
		t.Fatal("repeat approval permitted")
	}
	if len(s.Outbox()) != 1 || s.Outbox()[0].Status != "pending_provider_configuration" {
		t.Fatal("incorrect outbox")
	}
	if err = s.Reject(a.ID, "Changed mind"); !errors.Is(err, ErrConflict) {
		t.Fatal("approved application rejected")
	}
}
func TestOwnershipAndDuplicates(t *testing.T) {
	s, in := fixture(t, 1)
	a, token, err := s.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Own(a.ID, "wrong-token"); !errors.Is(err, ErrNotFound) {
		t.Fatal("private details exposed")
	}
	if _, err = s.Own(a.ID, token); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Create(in); !errors.Is(err, ErrConflict) {
		t.Fatal("duplicate phone accepted")
	}
}
func TestCapacityUnderConcurrency(t *testing.T) {
	s, in := fixture(t, 1)
	ids := []string{}
	for i := 0; i < 8; i++ {
		in.Phone = fmt.Sprintf("+23480123456%02d", i)
		a, _, err := s.Create(in)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, a.ID)
		if err = s.SetChecks(a.ID, allChecks()); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	var approved atomic.Int32
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			err := s.Approve(id, "2027-01-01")
			if err == nil {
				approved.Add(1)
			} else if !errors.Is(err, ErrCapacity) {
				t.Errorf("unexpected %v", err)
			}
		}(id)
	}
	wg.Wait()
	if approved.Load() != 1 {
		t.Fatal("capacity exceeded")
	}
}
func TestPersistenceAndFailedWriteRollback(t *testing.T) {
	s, in := fixture(t, 2)
	s.path = filepath.Join(t.TempDir(), "data.json")
	a, token, err := s.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := New(s.path, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = restored.Own(a.ID, token); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) == "" {
		t.Fatal("empty store")
	}
	blocker := filepath.Join(t.TempDir(), "file")
	if err = os.WriteFile(blocker, []byte("block"), 0600); err != nil {
		t.Fatal(err)
	}
	s.path = filepath.Join(blocker, "data.json")
	in.Phone = "+2348012345679"
	if _, _, err = s.Create(in); !errors.Is(err, ErrStorage) {
		t.Fatal("expected failed persistence")
	}
	if len(s.List()) != 1 {
		t.Fatal("failed write changed memory")
	}
}
func TestRejectionAndPastDate(t *testing.T) {
	s, in := fixture(t, 1)
	a, _, err := s.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Approve(a.ID, "2026-10-23"); err == nil {
		t.Fatal("past date accepted")
	}
	if err = s.Reject(a.ID, ""); err == nil {
		t.Fatal("missing reason accepted")
	}
	if err = s.Reject(a.ID, "Business evidence needs correction"); err != nil {
		t.Fatal(err)
	}
	if err = s.SetChecks(a.ID, allChecks()); !errors.Is(err, ErrConflict) {
		t.Fatal("terminal state modified")
	}
}
