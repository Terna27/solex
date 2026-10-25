package grants

import (
	"time"
)

type Event struct {
	ID            string    `json:"id,omitempty"`
	ApplicationID string    `json:"application_id,omitempty"`
	UserID        string    `json:"user_id,omitempty"`
	Actor         string    `json:"actor"` // "applicant:<id>", "admin", "system"
	Action        string    `json:"action"`
	Details       string    `json:"details,omitempty"`
	At            time.Time `json:"at"`
}

type AdminMetrics struct {
	TotalUsers                   int   `json:"total_users"`
	TotalApplications            int   `json:"total_applications"`
	RegisteredBusinessApps       int   `json:"registered_business_apps"`
	UnregisteredBusinessApps     int   `json:"unregistered_business_apps"`
	ApplicationsAwaitingPayment  int   `json:"applications_awaiting_payment"`
	SubmittedApplications        int   `json:"submitted_applications"`
	ApplicationsUnderReview      int   `json:"applications_under_review"`
	ApprovedApplications         int   `json:"approved_applications"`
	RejectedApplications         int   `json:"rejected_applications"`
	DisbursedApplications        int   `json:"disbursed_applications"`
	BatchACount                  int   `json:"batch_a_count"`
	BatchBCount                  int   `json:"batch_b_count"`
	BatchAAmountKobo             int64 `json:"batch_a_amount_kobo"`
	BatchBAmountKobo             int64 `json:"batch_b_amount_kobo"`
	VerifiedFeesCollectedKobo    int64 `json:"verified_fees_collected_kobo"`
	ApprovedGrantCommitmentsKobo int64 `json:"approved_grant_commitments_kobo"`
	RecordedDisbursementsKobo    int64 `json:"recorded_disbursements_kobo"`
}

type ExportRecord struct {
	ApplicationID      string `json:"application_id"`
	ReferenceCode      string `json:"reference_code"`
	ApplicantName      string `json:"applicant_name"`
	Phone              string `json:"phone"`
	Email              string `json:"email"`
	MaskedNIN          string `json:"masked_nin"`
	Category           string `json:"category"`
	CACNumber          string `json:"cac_number"`
	State              string `json:"state"`
	LGA                string `json:"lga"`
	BusinessType       string `json:"business_type"`
	Status             string `json:"status"`
	GrantAmountKobo    int64  `json:"grant_amount_kobo"`
	PaymentStatus      string `json:"payment_status"`
	DisbursementBatch  string `json:"disbursement_batch,omitempty"`
	DisbursementDate   string `json:"disbursement_date,omitempty"`
	DisbursementStatus string `json:"disbursement_status"`
	CreatedAt          string `json:"created_at"`
}

func MaskNIN(nin string) string {
	if len(nin) != 11 {
		return "***********"
	}
	return "*******" + nin[7:]
}

func MaskAccountNumber(acc string) string {
	if len(acc) != 10 {
		return "**********"
	}
	return "******" + acc[6:]
}
