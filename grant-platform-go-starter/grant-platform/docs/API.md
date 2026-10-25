# Solex Business Grant Platform — API Guide

Base URL: `http://127.0.0.1:8080/api/v1`

## API Endpoints Overview

| Method | Path | Authentication | Description |
| --- | --- | --- | --- |
| `GET` | `/health` | None | Service status, mode, and commencement dates |
| `GET` | `/programme` | None | Programme categories, dates, bank catalogue, states |
| `GET` | `/fee-disclosure` | None | Statutory disclosure for the ₦1,000 application fee |
| `POST` | `/auth/register` | None | Create an applicant account |
| `POST` | `/auth/login` | None | Authenticate applicant and issue session cookie |
| `POST` | `/auth/logout` | Session | Invalidate active session and clear cookie |
| `GET` | `/auth/me` | Session | Get current authenticated user and profile summary |
| `GET` | `/profile` | Session | Fetch applicant's completed profile |
| `PUT` | `/profile` | Session | Save/update applicant personal, business, and bank details |
| `GET` | `/applications/current` | Session | Get applicant's active grant application |
| `POST` | `/applications/start` | Session | Start application with chosen category (`unregistered` or `registered`) |
| `POST` | `/applications/{id}/documents` | Session / Token | Upload supporting document (`passport_photo`, `government_id`, `proof_of_address`, `cac_certificate`) |
| `GET` | `/applications/{id}/documents/{docID}` | Session / Token | View private uploaded image |
| `POST` | `/applications/ready-for-payment` | Session | Verify uploaded documents and advance to `awaiting_payment` |
| `POST` | `/payments/init` | Session | Initialize ₦1,000 payment record |
| `POST` | `/payments/verify` | Session | Server-side payment verification (with sandbox simulator support) |
| `POST` | `/applications/submit` | Session | Finalize submission; queue confirmation email |
| `GET` | `/admin/metrics` | Admin Token | High-level metrics across users, categories, fees, and grants |
| `GET` | `/admin/applications` | Admin Token | Filtered list of applications (`q`, `state`, `category`, `status`) |
| `GET` | `/admin/applications/{id}` | Admin Token | Complete application dossier with documents |
| `PUT` | `/admin/applications/{id}/checks` | Admin Token | Update verification checklist (Phone, NIN/Age, Address, Business, Bank, CAC) |
| `POST` | `/admin/applications/{id}/approve` | Admin Token | Approve application for one-time grant allocation |
| `POST` | `/admin/applications/{id}/reject` | Admin Token | Reject application with reason |
| `POST` | `/admin/applications/{id}/disburse` | Admin Token | Record disbursement for approved application |
| `GET` | `/admin/export` | Admin Token | Export privacy-preserving records (JSON or CSV with masked NIN) |
| `GET` | `/admin/emails` | Admin Token | Inspect queued/sent applicant notification emails |
| `GET` | `/admin/audit` | Admin Token | View immutable administrative audit trail |

---

## Authentication Modes

1. **Applicant Sessions**:
   - `Set-Cookie: solex_session=<token>; HttpOnly; SameSite=Lax; Path=/`
   - Alternatively: `Authorization: Bearer <session_token>`
2. **Admin Token**:
   - Header: `Authorization: Bearer <ADMIN_TOKEN>`
