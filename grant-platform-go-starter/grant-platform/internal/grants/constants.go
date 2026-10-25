package grants

import (
	"errors"
	"os"
)

// Grant amounts in kobo (integer arithmetic, no float rounding errors)
const (
	GrantAmountUnregisteredKobo int64 = 25000000 // ₦250,000
	GrantAmountRegisteredKobo   int64 = 50000000 // ₦500,000
	RegistrationFeeKobo         int64 = 100000   // ₦1,000
	Currency                          = "NGN"

	CategoryUnregistered = "unregistered"
	CategoryRegistered   = "registered"

	DefaultCommencementDate = "2026-12-10"
	DefaultDisbursementDate = "December 2026"

	// Session duration
	DefaultSessionDurationHours = 24
)

// Application Statuses
const (
	StatusDraft            = "draft"
	StatusAwaitingPayment  = "awaiting_payment"
	StatusPaymentConfirmed = "payment_confirmed"
	StatusSubmitted        = "submitted"
	StatusUnderReview      = "under_review"
	StatusApproved         = "approved"
	StatusRejected         = "rejected"
	StatusDisbursed        = "disbursed"
)

// Document Kinds
const (
	DocPassportPhoto   = "passport_photo"
	DocGovernmentID    = "government_id"
	DocProofOfAddress  = "proof_of_address"
	DocCACCertificate  = "cac_certificate"
)

// Payment Statuses
const (
	PaymentStatusPending    = "pending"
	PaymentStatusSuccessful = "successful"
	PaymentStatusFailed     = "failed"
)

// Common Domain Errors
var (
	ErrNotFound            = errors.New("application not found")
	ErrUserNotFound        = errors.New("user not found")
	ErrConflict            = errors.New("application cannot be changed in its current state")
	ErrCapacity            = errors.New("funded beneficiary capacity reached")
	ErrStorage             = errors.New("could not save changes")
	ErrUnauthorized        = errors.New("unauthorised")
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrDuplicateEmail      = errors.New("an account with this email already exists")
	ErrDuplicatePhone      = errors.New("an account with this phone number already exists")
	ErrDuplicateCAC        = errors.New("an application with this CAC number already exists")
	ErrActiveAppExists     = errors.New("you already have an active application")
	ErrInvalidTransition   = errors.New("invalid application state transition")
	ErrPaymentRequired     = errors.New("verified payment of registration fee is required before submission")
	ErrProfileIncomplete   = errors.New("profile must be completed before applying")
	ErrCACRequired         = errors.New("CAC registration number and certificate are required for registered businesses")
	ErrInvalidNIN          = errors.New("NIN must contain exactly 11 digits")
	ErrInvalidAccount      = errors.New("account_number must contain exactly 10 digits")
	ErrInvalidBank         = errors.New("select a supported Nigerian bank")
	ErrInvalidPassword     = errors.New("password must be at least 8 characters long")
	ErrPasswordMismatch    = errors.New("passwords do not match")
	ErrInvalidEmail        = errors.New("enter a valid email address")
	ErrInvalidPhone        = errors.New("phone must use Nigerian international format, such as +2348012345678")
	ErrRateLimited         = errors.New("too many attempts; please try again later")
)

// PurposeOption represents a valid purpose of grant option.
type PurposeOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

var GrantPurposeOptions = []PurposeOption{
	{Value: "working_capital", Label: "Business working capital"},
	{Value: "equipment", Label: "Purchase of equipment or machinery"},
	{Value: "inventory", Label: "Purchase of goods or stock"},
	{Value: "expansion", Label: "Business expansion"},
	{Value: "startup", Label: "Starting a new business"},
	{Value: "agriculture", Label: "Agricultural production or farming"},
	{Value: "renovation", Label: "Business premises improvement"},
	{Value: "technology", Label: "Technology and digital tools"},
	{Value: "other", Label: "Other business-related purpose"},
}

var ValidGrantPurposes = func() map[string]string {
	m := make(map[string]string, len(GrantPurposeOptions))
	for _, opt := range GrantPurposeOptions {
		m[opt.Value] = opt.Label
	}
	return m
}()

func FormatGrantPurpose(value string) string {
	if label, ok := ValidGrantPurposes[value]; ok {
		return label
	}
	if value == "" {
		return "Not specified"
	}
	return value
}

// ProgrammeConfig encapsulates programme metadata exposed via API.
type ProgrammeConfig struct {
	CommencementDate    string                  `json:"commencement_date"`
	DisbursementDate    string                  `json:"disbursement_date"`
	Currency            string                  `json:"currency"`
	RegistrationFeeKobo int64                   `json:"registration_fee_kobo"`
	MinimumAge          int                     `json:"minimum_age"`
	States              []string                `json:"states"`
	Banks               []string                `json:"banks"`
	GrantPurposes       []PurposeOption         `json:"grant_purposes"`
	Categories          map[string]CategoryInfo `json:"categories"`
}

type CategoryInfo struct {
	Name        string `json:"name"`
	AmountKobo  int64  `json:"amount_kobo"`
	PaymentType string `json:"payment_type"`
	CACRequired bool   `json:"cac_required"`
	Description string `json:"description"`
}

func GetCommencementDate() string {
	if v := os.Getenv("PROGRAMME_COMMENCEMENT_DATE"); v != "" {
		return v
	}
	return DefaultCommencementDate
}

func GetDisbursementDate() string {
	if v := os.Getenv("PROGRAMME_DISBURSEMENT_DATE"); v != "" {
		return v
	}
	return DefaultDisbursementDate
}

func GetProgrammeConfig() ProgrammeConfig {
	return ProgrammeConfig{
		CommencementDate:    GetCommencementDate(),
		DisbursementDate:    GetDisbursementDate(),
		Currency:            Currency,
		RegistrationFeeKobo: RegistrationFeeKobo,
		MinimumAge:          18,
		States:              States,
		Banks:               Banks,
		GrantPurposes:       GrantPurposeOptions,
		Categories: map[string]CategoryInfo{
			CategoryUnregistered: {
				Name:        "Unregistered Business Grant",
				AmountKobo:  GrantAmountUnregisteredKobo,
				PaymentType: "one-time",
				CACRequired: false,
				Description: "One-time grant support of ₦250,000 for everyday informal businesses and traders without CAC registration.",
			},
			CategoryRegistered: {
				Name:        "Registered Business Grant",
				AmountKobo:  GrantAmountRegisteredKobo,
				PaymentType: "one-time",
				CACRequired: true,
				Description: "One-time grant support of ₦500,000 for formal Nigerian enterprises with valid CAC registration.",
			},
		},
	}
}
