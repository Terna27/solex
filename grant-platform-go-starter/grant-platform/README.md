# Grant Platform — Go backend, milestone 1

A local prototype for a nationwide Nigerian small-business grant programme:

- NGN 75,000 per month for three months (NGN 225,000 per approved beneficiary).
- Adults aged 18 and above; all 36 states plus FCT.
- No application, verification, code-generation, or grant-access fee.
- Eligibility independent of party membership, voting preference, or campaign activity.

**This is a backend prototype, not a public launch. Use fictional data only.**
No real SMS, bank transfer, document upload, or automated identity check is connected.
An admin can mark sandbox verification checks complete to exercise the workflow.
These checkboxes are not evidence that a real person has been verified.

## Run locally

Requires Go 1.22 or newer. The project uses only the Go standard library.
Use a supported Go release before deployment.

In Linux, macOS, or Windows WSL:

```bash
cd grant-platform
export APP_MODE=local_prototype
export ADMIN_TOKEN="$(python3 -c 'import secrets; print(secrets.token_hex(32))')"
export BENEFICIARY_CAPACITY=10
go run ./cmd/api
```

The capacity of 10 above is a **local testing example**, not the programme's actual
funded beneficiary count. Set the confirmed count before approving applications.
Capacity is enforced during approval even if administrators act concurrently.
Each approved applicant reserves one beneficiary slot representing NGN 225,000.

The server listens only on `http://127.0.0.1:8080`. It deliberately does not expose
itself to other computers. The admin token must contain at least 32 characters;
use the randomly generated token, not a memorable password. Keep the same token
available to your API client. Never commit it or share it with applicants.

Data persists to `data/grants.json`. Set `DATA_FILE` to change the location.
Run only one process against a data file. The JSON store is for local development;
PostgreSQL will replace it before public use. Do not store real identity documents
or bank details in this prototype.

## Test

```bash
go test -race ./...
go vet ./...
go build ./cmd/api
```

Tests cover age boundaries, ownership, admin access, duplicate phones, approval
requirements, three instalments, month-end scheduling, repeat approvals, concurrent
capacity enforcement, rejected applications, restart persistence, and rollback on
failed storage writes.

## Try an application

```bash
curl http://127.0.0.1:8080/api/v1/health

curl -X POST http://127.0.0.1:8080/api/v1/applications \
  -H 'Content-Type: application/json' \
  -d '{
    "full_name": "Example Applicant",
    "date_of_birth": "1990-05-20",
    "phone": "+2348012345678",
    "state": "Benue",
    "lga": "Makurdi",
    "business_type": "Retail shop",
    "business_description": "Fictional shop used to test the application process.",
    "consent": true
  }'
```

Save `application.id` and `access_token` from the response. The access token is a
private bearer credential shown only once; only its hash is stored. It is **not**
the approval reference code. The reference code cannot log in or authorise payment.

The complete endpoint list and review examples are in [docs/API.md](docs/API.md).

## Implemented

1. Application submission with validation, age check, consent, and state selection.
2. Private applicant status lookup using the application's access token.
3. Admin application listing and sandbox verification review.
4. Approval or rejection with explicit transition rules and rejection reasons.
5. Three monthly scheduled instalments of 7,500,000 kobo each.
6. Random free reference codes; a persistent SMS outbox awaiting a provider.
7. Basic audit events and atomic local file persistence.

Approval requires all five checks: phone, identity/age, address, business, and bank
account ownership. Setting a flag is currently an administrator's sandbox action;
no external checks actually occur. Approval and its outbox message are saved in
one local transaction. Repeating an approval returns HTTP 409 rather than issuing
another schedule or message.

LGA is collected as text and requires manual review. An authoritative state-to-LGA
dataset must be integrated before production; current validation checks only the
state and text length. Shared-phone applicants and duplicate flags need a support
and appeal process before launch.

## Next milestones

1. PostgreSQL repository and migrations, verified geography catalogue, per-staff
   authentication, role separation, applicant phone OTP login and recovery.
2. Private document uploads, file type and malware checks, signed access links,
   reviewer evidence, correction requests, and retention/deletion rules.
3. Applicant pages and admin dashboard. The proposed frontend is Next.js;
   all business rules and payment integrations remain in Go.
4. SMS provider adapter, durable delivery worker, bounded retries, and delivery
   receipts. A queued message must never be displayed as delivered.
5. Bank-account verification and a transfer provider sandbox. Implement signature
   validation, payment idempotency keys per applicant/instalment, reconciliation,
   dual approval, and webhook replay protection before enabling live money movement.
6. Confirm programme name, funded capacity, actual disbursement dates, eligibility
   and selection criteria. Obtain appropriate programme/privacy/election legal
   review before collecting real applicant data or launching publicly.

Future payment states should distinguish scheduled, processing, confirmed paid,
failed, and reversed. This build intentionally offers no endpoint to pretend a
transfer succeeded. All instalments remain `scheduled`.

## Production limitations

This milestone has no frontend, public hosting, applicant OTP, staff accounts,
password recovery, MFA, pagination, rate limiting, private uploads, provider
credentials, automatic transfers, or live delivery. The audit log is a local
development record, not an independently protected audit service. Stored data is
not encrypted by this application. Do not deploy this build publicly or use real
beneficiary data. Provider and infrastructure configuration belong in later phases.

## Project layout

```text
cmd/api/main.go                 Startup, configuration, shutdown
internal/grants/service.go     Application rules, approval, schedule, local storage
internal/grants/service_test.go
internal/httpapi/server.go     HTTP routes, JSON handling, access control
internal/httpapi/server_test.go
docs/API.md                    API examples
```
