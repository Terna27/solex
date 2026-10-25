# Solex Business Grant Platform — Go Backend & Modern Frontend

A secure, account-based platform for the nationwide Nigerian small-business grant programme.

## Programme Highlights

- **Category A — Unregistered Businesses**: **₦250,000** one-time grant support for everyday market traders, artisans, and informal micro-enterprises. (CAC certificate not required).
- **Category B — Registered Businesses**: **₦500,000** one-time grant support for formal Nigerian businesses with valid Corporate Affairs Commission (CAC) incorporation. (CAC number and certificate required).
- **Programme Commencement**: **10 December 2026**.
- **Scheduled Disbursement**: December 2026.
- **Application Processing Fee**: **₦1,000** per application.
- **Coverage**: All 36 Nigerian states and the Federal Capital Territory (FCT).
- **Eligibility**: Adults aged 18 and above; 11-digit NIN verification check; commercial Nigerian deposit-money banks (fintech wallets excluded).

---

## Architecture & Security

- **Backend**: Go standard library with `golang.org/x/crypto/bcrypt` for password hashing.
- **Frontend**: Clean, responsive multi-step SPA embedded directly into Go (`embed.FS`).
- **Data Persistence**: Schema-versioned JSON store with mutex locking, atomic disk writes (`.grants-*` temp file + rename), and seamless migration.
- **Session Management**: Cryptographically secure 256-bit random session tokens managed via `HttpOnly`, `SameSite=Lax` cookies with configurable `Secure` flag, plus Bearer token compatibility.
- **Rate Limiting**: In-memory IP/identifier rate limiting on authentication routes to mitigate brute-force and enumeration attempts.
- **Document Handling**: In-memory JPEG/PNG decoding and re-encoding up to 5 MB and 12 million pixels, stripping untrusted EXIF metadata and isolating private applicant documents under `0700` directories.
- **Payment Architecture**: Server-side ₦1,000 fee calculation, statutory fee disclosures, idempotent verification, and sandbox simulation strictly disabled in production.
- **Email Infrastructure**: Automated submission confirmation emails sent with application reference and potential grant amount; development outbox and pluggable SMTP interface; strict redaction of sensitive identifiers (NIN, full bank details).
- **Administrative Auditing**: Complete event trail for administrative reviews, approvals, rejections, and disbursements without storing sensitive values.

---

## Local Development & Startup

Requires Go 1.22 or newer.

```bash
cd grant-platform-go-starter/grant-platform
export APP_MODE=local_prototype
export ADMIN_EMAIL="admin@solex.ng"
export ADMIN_PASSWORD_HASH="$(python3 -c 'import bcrypt; print(bcrypt.hashpw(b"AdminP@ssw0rd2026!", bcrypt.gensalt()).decode())')"
export BENEFICIARY_CAPACITY=10
export PROGRAMME_COMMENCEMENT_DATE="2026-12-10"
export PROGRAMME_DISBURSEMENT_DATE="December 2026"

go run ./cmd/api
```

The server listens on `http://127.0.0.1:8080`.

Open your browser to:
- **`http://127.0.0.1:8080/#home`**: Programme overview & category details
- **`http://127.0.0.1:8080/#register`**: Create applicant account
- **`http://127.0.0.1:8080/#login`**: Sign in to applicant portal
- **`http://127.0.0.1:8080/#dashboard`**: Applicant tracking dashboard
- **`http://127.0.0.1:8080/#apply`**: Multi-step grant application journey
- **`http://127.0.0.1:8080/admin/login`**: Dedicated administrator login portal
- **`http://127.0.0.1:8080/admin/dashboard`**: Protected administrator review workspace

---

## Testing & Quality Assurance

Run the comprehensive automated test suite:

```bash
cd grant-platform-go-starter/grant-platform
go test -v -race ./...
go vet ./...
node --check internal/httpapi/web/app.js
git diff --check
```

Tests cover:
- Applicant account registration and duplicate prevention (email, phone, CAC).
- Secure password hashing, invalid password handling, and enumeration prevention.
- Session expiration and ownership isolation across tenants.
- 11-digit NIN validation string checks.
- Category A (₦250k) vs Category B (₦500k) grant calculations.
- Registered business CAC number and certificate requirements.
- Application state transitions (`draft` → `awaiting_payment` → `payment_confirmed` → `submitted` → `under_review` → `approved` / `rejected` → `disbursed`).
- ₦1,000 fee payment initiation, sandbox verification, and idempotency.
- Document upload re-encoding, mime validation, and private document endpoint protection.
- Admin metrics, checklist verification, approval, rejection, and CSV export masking.
- Dedicated administrator login route (`/admin/login`), session creation, cookie management, rate limiting, and applicant exclusion.
- Legacy backward-compatible application endpoints.
