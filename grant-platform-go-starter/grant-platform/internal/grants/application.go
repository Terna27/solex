package grants

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type ApplicationChecks struct {
	Phone    bool `json:"phone"`
	Identity bool `json:"identity_and_age"`
	Address  bool `json:"address"`
	Business bool `json:"business"`
	Bank     bool `json:"bank_account_ownership"`
	CAC      bool `json:"cac_registration"` // Required for registered businesses
}

func (c ApplicationChecks) Complete(category string) bool {
	base := c.Phone && c.Identity && c.Address && c.Business && c.Bank
	if category == CategoryRegistered {
		return base && c.CAC
	}
	return base
}

const (
	DisbursementBatchA = "Batch A"
	DisbursementBatchB = "Batch B"
)

func NormalizeBatch(b string) (string, error) {
	b = strings.TrimSpace(b)
	clean := strings.ToLower(strings.ReplaceAll(b, "_", " "))
	switch clean {
	case "batch a", "a":
		return DisbursementBatchA, nil
	case "batch b", "b":
		return DisbursementBatchB, nil
	default:
		return "", errors.New("invalid disbursement batch: must be Batch A or Batch B")
	}
}

func ValidateDisbursementDate(dStr string) (time.Time, error) {
	dStr = strings.TrimSpace(dStr)
	if dStr == "" {
		return time.Time{}, errors.New("scheduled disbursement date is required")
	}
	t, err := time.Parse("2006-01-02", dStr)
	if err != nil {
		return time.Time{}, errors.New("scheduled disbursement date must be a valid date in YYYY-MM-DD format")
	}
	if t.Year() != 2026 || t.Month() != time.December {
		return time.Time{}, errors.New("scheduled disbursement date must fall within December 2026 according to the programme schedule")
	}
	return t, nil
}

type Application struct {
	ID                  string            `json:"id"`
	UserID              string            `json:"user_id,omitempty"`
	ReferenceCode       string            `json:"reference_code,omitempty"`
	Category            string            `json:"category"`
	GrantAmountKobo     int64             `json:"grant_amount_kobo"`
	RegistrationFeeKobo int64             `json:"registration_fee_kobo"`
	Status              string            `json:"status"`
	Checks              ApplicationChecks `json:"checks"`
	Reason              string            `json:"reason,omitempty"`
	PaymentID           string            `json:"payment_id,omitempty"`
	DisbursementBatch   string            `json:"disbursement_batch,omitempty"`
	DisbursementDate    string            `json:"disbursement_date,omitempty"`
	DisbursementStatus  string            `json:"disbursement_status,omitempty"`
	Profile             ApplicantProfile  `json:"profile"`
	Documents           []Document        `json:"documents"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`

	// Anonymous embedded Input and legacy Instalments
	Input
	Instalments []Instalment `json:"instalments,omitempty"`
}

var validTransitions = map[string][]string{
	StatusDraft:            {StatusAwaitingPayment},
	StatusAwaitingPayment:  {StatusPaymentConfirmed, StatusDraft},
	StatusPaymentConfirmed: {StatusSubmitted},
	StatusSubmitted:        {StatusUnderReview, StatusApproved, StatusRejected},
	StatusUnderReview:      {StatusApproved, StatusRejected},
	StatusApproved:         {StatusDisbursed},
	StatusRejected:         {},
	StatusDisbursed:        {},
}

func CanTransition(from, to string) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

func CalculateGrantAmount(category string) (int64, error) {
	switch category {
	case CategoryUnregistered:
		return GrantAmountUnregisteredKobo, nil
	case CategoryRegistered:
		return GrantAmountRegisteredKobo, nil
	default:
		return 0, fmt.Errorf("invalid business category: %s", category)
	}
}

func HasRequiredDocuments(docs []Document, category string) (bool, string) {
	hasPassport := false
	hasSupport := false
	hasCAC := false

	for _, d := range docs {
		switch d.Kind {
		case DocPassportPhoto:
			hasPassport = true
		case DocGovernmentID, DocProofOfAddress:
			hasSupport = true
		case DocCACCertificate:
			hasCAC = true
		}
	}

	if !hasPassport {
		return false, "a passport photograph is required"
	}
	if !hasSupport {
		return false, "a government-issued ID or proof of address is required"
	}
	if category == CategoryRegistered && !hasCAC {
		return false, "a CAC registration certificate is required for registered businesses"
	}

	return true, ""
}

func generateReferenceCode() (string, error) {
	code, err := randomID()
	if err != nil {
		return "", err
	}
	return "SOLEX-2026-" + strings.ToUpper(code[:8]), nil
}
