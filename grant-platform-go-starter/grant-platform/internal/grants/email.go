package grants

import (
	"context"
	"errors"
	"fmt"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Email delivery statuses
const (
	EmailStatusSavedLocally = "saved_locally"
	EmailStatusQueued       = "queued"
	EmailStatusSent         = "sent"
	EmailStatusFailed       = "failed"
)

type EmailMessage struct {
	ID          string     `json:"id"`
	To          string     `json:"to"`
	From        string     `json:"from,omitempty"`
	Subject     string     `json:"subject"`
	Body        string     `json:"body"`
	Status      string     `json:"status"` // "saved_locally", "queued", "sent", "failed"
	Attempts    int        `json:"attempts"`
	LastError   string     `json:"last_error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	SentAt      *time.Time `json:"sent_at,omitempty"`
	NextRetryAt *time.Time `json:"next_retry_at,omitempty"`
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

// SMTPEmailSender provides production SMTP email delivery using environment configuration.
type SMTPEmailSender struct {
	Host       string
	Port       int
	Username   string
	Password   string
	From       string
	FromName   string
	SkipVerify bool
}

func (s *SMTPEmailSender) Send(ctx context.Context, msg EmailMessage) error {
	if s.Host == "" {
		return errors.New("SMTP_HOST is not configured")
	}
	port := s.Port
	if port == 0 {
		port = 587
	}
	addr := fmt.Sprintf("%s:%d", s.Host, port)
	from := s.From
	if from == "" {
		from = "notifications@solex.ng"
	}
	fromHeader := from
	if s.FromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", s.FromName, from)
	}

	headers := make(map[string]string)
	headers["From"] = fromHeader
	headers["To"] = msg.To
	headers["Subject"] = msg.Subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/plain; charset=UTF-8"

	var message strings.Builder
	for k, v := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")
	message.WriteString(msg.Body)

	var auth smtp.Auth
	if s.Username != "" && s.Password != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, s.Host)
	}

	return smtp.SendMail(addr, auth, from, []string{msg.To}, []byte(message.String()))
}

// MockEmailSender enables deterministic testing of temporary delivery failures and retries.
type MockEmailSender struct {
	mu        sync.Mutex
	FailNext  int
	SendCount int
	SentMsgs  []EmailMessage
	Err       error
}

func NewMockEmailSender() *MockEmailSender {
	return &MockEmailSender{}
}

func (m *MockEmailSender) SetFailNext(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FailNext = n
}

func (m *MockEmailSender) SetErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Err = err
}

func (m *MockEmailSender) GetSendCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.SendCount
}

func (m *MockEmailSender) GetSentMsgs() []EmailMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]EmailMessage{}, m.SentMsgs...)
}

func (m *MockEmailSender) Send(ctx context.Context, msg EmailMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SendCount++
	m.SentMsgs = append(m.SentMsgs, msg)
	if m.FailNext > 0 {
		m.FailNext--
		if m.Err != nil {
			return m.Err
		}
		return errors.New("temporary SMTP connection timeout (503 Service Unavailable)")
	}
	return nil
}

func NewEmailSenderFromEnv() EmailSender {
	provider := strings.ToLower(os.Getenv("EMAIL_PROVIDER"))
	from := os.Getenv("EMAIL_FROM")
	if from == "" {
		from = "notifications@solex.ng"
	}
	fromName := os.Getenv("EMAIL_FROM_NAME")
	if fromName == "" {
		fromName = "Solex Business Grant Programme"
	}

	if provider == "smtp" {
		port, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
		if port == 0 {
			port = 587
		}
		return &SMTPEmailSender{
			Host:       os.Getenv("SMTP_HOST"),
			Port:       port,
			Username:   os.Getenv("SMTP_USERNAME"),
			Password:   os.Getenv("SMTP_PASSWORD"),
			From:       from,
			FromName:   fromName,
			SkipVerify: os.Getenv("SMTP_SKIP_VERIFY") == "true",
		}
	}

	// Default: development local outbox
	return NewDevelopmentOutboxSender(nil)
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

func FormatApprovalEmail(applicantName, refCode, category, batch, disbursementDate string, grantAmountKobo int64) (string, string) {
	subject := fmt.Sprintf("Solex Business Grant — Application Approved (Ref: %s)", refCode)

	categoryLabel := "Unregistered Business"
	if category == CategoryRegistered {
		categoryLabel = "Registered Business"
	}
	amountFormatted := fmt.Sprintf("₦%s", formatNaira(grantAmountKobo))

	body := fmt.Sprintf(`Dear %s,

We are pleased to inform you that your application for the Solex Business Grant Programme has been officially approved!

Approval Details:
- Applicant Legal Name: %s
- Programme: Solex Business Grant Programme
- Application Reference: %s
- Business Category: %s
- Approved Grant Amount: %s (One-time, non-repayable grant)
- Assigned Disbursement Batch: %s
- Scheduled Disbursement Date: %s

Next Steps:
Please log in to your applicant dashboard at https://solex.ng/#dashboard to check your payment status and view disbursement records.

Important Notice:
The scheduled disbursement date is subject to standard banking processing and clearing timelines. You will receive an update once funds transfer commences.

Congratulations on your selection.

Best regards,
Solex Business Grant Committee
Solex Operations Desk`,
		strings.TrimSpace(applicantName),
		strings.TrimSpace(applicantName),
		refCode,
		categoryLabel,
		amountFormatted,
		batch,
		disbursementDate,
	)

	return subject, body
}

func FormatScheduleChangeEmail(applicantName, refCode, batch, disbursementDate, reason string, grantAmountKobo int64) (string, string) {
	subject := fmt.Sprintf("Solex Business Grant Programme — Disbursement Schedule Update (Ref: %s)", refCode)

	amountFormatted := fmt.Sprintf("₦%s", formatNaira(grantAmountKobo))
	if reason == "" {
		reason = "Administrative scheduling adjustment"
	}

	body := fmt.Sprintf(`Dear %s,

This is an official notification regarding your approved application for the Solex Business Grant Programme (Ref: %s).

Your disbursement schedule has been updated by the programme administrator:
- Approved Grant Amount: %s
- Assigned Disbursement Batch: %s
- Scheduled Disbursement Date: %s
- Reason for Update: %s

Please log in to your dashboard at https://solex.ng/#dashboard to review your updated application details.

Please note that actual payment delivery remains subject to banking settlement processes.

Best regards,
Solex Business Grant Committee
Solex Operations Desk`,
		strings.TrimSpace(applicantName),
		refCode,
		amountFormatted,
		batch,
		disbursementDate,
		reason,
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
