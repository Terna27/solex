package grants

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type EmailMessage struct {
	ID        string     `json:"id"`
	To        string     `json:"to"`
	Subject   string     `json:"subject"`
	Body      string     `json:"body"`
	Status    string     `json:"status"` // "saved_locally", "sent", "failed"
	CreatedAt time.Time  `json:"created_at"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
}

type EmailSender interface {
	Send(ctx context.Context, msg EmailMessage) error
}

// DevelopmentOutboxSender stores emails in the service database and outbox.
type DevelopmentOutboxSender struct {
	onSend func(msg EmailMessage) error
}

func NewDevelopmentOutboxSender(onSend func(msg EmailMessage) error) *DevelopmentOutboxSender {
	return &DevelopmentOutboxSender{onSend: onSend}
}

func (d *DevelopmentOutboxSender) Send(ctx context.Context, msg EmailMessage) error {
	if d.onSend != nil {
		return d.onSend(msg)
	}
	return nil
}

func FormatSubmissionConfirmationEmail(applicantName, refCode, category string, grantAmountKobo int64) (string, string) {
	subject := "Solex Grant Programme — Application Successfully Submitted"

	categoryLabel := "Unregistered Business"
	if category == CategoryRegistered {
		categoryLabel = "Registered Business"
	}

	amountFormatted := fmt.Sprintf("₦%s", formatNaira(grantAmountKobo))

	body := fmt.Sprintf(`Dear %s,

Thank you for applying for the Solex Business Grant Programme.

Your application has been successfully submitted and received for consideration.

Application Reference: %s
Business Category: %s
Potential Grant Amount: %s

Please note that successful submission does not mean your application has been approved. All applications are subject to review and eligibility verification.

You will receive further communication if additional information is required or a decision is made.

Solex Business Grant Programme`,
		strings.TrimSpace(applicantName),
		refCode,
		categoryLabel,
		amountFormatted,
	)

	return subject, body
}

func formatNaira(kobo int64) string {
	naira := kobo / 100
	s := fmt.Sprintf("%d", naira)
	n := len(s)
	if n <= 3 {
		return s
	}
	var res []string
	remainder := n % 3
	if remainder > 0 {
		res = append(res, s[:remainder])
	}
	for i := remainder; i < n; i += 3 {
		res = append(res, s[i:i+3])
	}
	return strings.Join(res, ",")
}
