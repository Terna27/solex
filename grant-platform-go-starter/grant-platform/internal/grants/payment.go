package grants

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type FeeDisclosure struct {
	FeeAmountKobo          int64  `json:"fee_amount_kobo"`
	Currency               string `json:"currency"`
	Purpose                string `json:"purpose"`
	OperatorIdentity       string `json:"operator_identity"`
	RefundPolicy           string `json:"refund_policy"`
	NoGuaranteeNotice      string `json:"no_guarantee_notice"`
	SelectionProcess       string `json:"selection_process"`
	ContactInformation     string `json:"contact_information"`
	PolicyStatus           string `json:"policy_status"`
	SandboxSimulatorActive bool   `json:"sandbox_simulator_active"`
}

func GetFeeDisclosure(isProduction bool) FeeDisclosure {
	return FeeDisclosure{
		FeeAmountKobo:    RegistrationFeeKobo,
		Currency:         Currency,
		Purpose:          "Covers administrative processing, identity and document verification review, and platform operation.",
		OperatorIdentity: "Solex Business Grant Programme Administration Committee",
		RefundPolicy:     "Application processing fees are non-refundable once administrative review has commenced.",
		NoGuaranteeNotice: "Payment of the ₦1,000 application fee does not guarantee grant approval or funding. All applications undergo merit, eligibility, and document review.",
		SelectionProcess: "Applications are assessed based on complete business information, verifiable Nigerian operations, and document authentication.",
		ContactInformation: "Email: support@solex.ng | Support Desk: 12 Broad Street, Lagos, Nigeria",
		PolicyStatus:       "Preliminary policy: requires administrator configuration and formal legal review prior to live payment collection.",
		SandboxSimulatorActive: !isProduction,
	}
}

type Payment struct {
	ID                string     `json:"id"`
	ApplicationID     string     `json:"application_id"`
	UserID            string     `json:"user_id"`
	Reference         string     `json:"reference"`
	AmountKobo        int64      `json:"amount_kobo"`
	Currency          string     `json:"currency"`
	Status            string     `json:"status"` // pending, successful, failed
	Provider          string     `json:"provider"`
	ProviderReference string     `json:"provider_reference,omitempty"`
	PaidAt            *time.Time `json:"paid_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

func generatePaymentReference() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "PAY-SOLEX-" + strings.ToUpper(hex.EncodeToString(b)), nil
}

type PaymentInitInput struct {
	ApplicationID string `json:"application_id"`
}

type PaymentVerifyInput struct {
	Reference string `json:"reference"`
	// For sandbox simulation:
	SimulatorAction string `json:"simulator_action,omitempty"` // "simulate_success" or "simulate_failure"
}

var (
	ErrPaymentNotFound    = errors.New("payment record not found")
	ErrPaymentAlreadyPaid = errors.New("payment has already been verified and processed")
	ErrInvalidPaymentRef  = errors.New("invalid payment reference")
	ErrSandboxNotAllowed  = errors.New("sandbox simulator is not available in production mode")
)
