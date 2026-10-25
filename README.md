# Solex Business Grant Platform

A nationwide Nigerian small-business grant application and administration platform.

## Key Features

- **Category A — Unregistered Businesses**: **₦250,000** one-time grant support for everyday micro-businesses, traders, and artisans.
- **Category B — Registered Businesses**: **₦500,000** one-time grant support for formal enterprises with Corporate Affairs Commission (CAC) incorporation.
- **Programme Commencement**: **10 December 2026**; scheduled disbursements in December 2026.
- **Applicant Accounts**: Secure registration, Bcrypt password hashing, session management via HttpOnly cookies, and login rate limiting.
- **NIN & Bank Validation**: 11-digit NIN validation, Nigerian commercial bank catalogue, and 10-digit NUBAN checks.
- **CAC Registration**: Mandatory CAC number and certificate upload for Category B applications with administrative verification.
- **Application Processing Fee**: **₦1,000** fee with statutory pre-payment disclosure and local sandbox payment simulator.
- **Email Infrastructure**: Automated submission confirmations without exposing sensitive identifiers.
- **Administrator Dashboard**: Metrics tracking, applicant review checklist, decisions with recorded reasons, disbursements, audit logging, and privacy-preserving CSV export.
- **GitHub Pages Preview**: Static informational landing page in `docs/` compatible with GitHub Pages hosting.

---

## Directory Structure

```
solex/
├── docs/                                    # Public informational landing page for GitHub Pages
│   ├── index.html
│   ├── app.js
│   └── style.css
└── grant-platform-go-starter/
    └── grant-platform/                      # Main Go backend and embedded application
        ├── cmd/api/main.go
        ├── internal/
        │   ├── grants/                      # Domain logic, models, persistence, and audit
        │   └── httpapi/                     # HTTP routes, auth, payments, admin, and embedded UI
        │       └── web/                     # HTML, CSS, JavaScript frontend
        └── README.md                        # Application setup guide
```

---

## Quickstart

```bash
cd grant-platform-go-starter/grant-platform
export APP_MODE=local_prototype
export ADMIN_TOKEN="$(python3 -c 'import secrets; print(secrets.token_hex(32))')"
export BENEFICIARY_CAPACITY=10
go run ./cmd/api
```

Open `http://127.0.0.1:8080` in your web browser.

## Run Tests

```bash
cd grant-platform-go-starter/grant-platform
go test -v -race ./...
go vet ./...
node --check internal/httpapi/web/app.js
git diff --check
```
