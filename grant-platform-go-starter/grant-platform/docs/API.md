# API guide

The latest required profile fields and approval document requirements are in
[COMPLETE_APPLICATION.md](COMPLETE_APPLICATION.md). Upload a passport photograph
and an ID/proof-of-address image before the approval example below.

Base URL: `http://127.0.0.1:8080/api/v1`

| Method | Path | Authentication | Purpose |
| --- | --- | --- | --- |
| GET | `/health` | None | Local prototype health and provider status |
| GET | `/programme` | None | Amounts, duration, free access, state options |
| POST | `/applications` | None | Submit application and obtain private access token |
| GET | `/applications/{id}` | Applicant bearer token | Read own application and schedule |
| GET | `/admin/applications` | Admin bearer token | Review applications |
| PUT | `/admin/applications/{id}/checks` | Admin bearer token | Replace all sandbox check flags |
| POST | `/admin/applications/{id}/approve` | Admin bearer token | Approve, create schedule, queue SMS |
| POST | `/admin/applications/{id}/reject` | Admin bearer token | Reject with reason |
| GET | `/admin/outbox` | Admin bearer token | See messages awaiting provider setup |
| GET | `/admin/audit` | Admin bearer token | See development audit events |

Send `Content-Type: application/json` on requests with JSON bodies. Unknown JSON
fields, oversized payloads, and multiple JSON values are rejected. Responses use
400 for validation failures, 401 for missing/invalid admin credentials, 404 for
missing or unauthorised applicant records, and 409 for conflicts/capacity limits.

## Review flow

After creating an application using the README example, set these values in a
terminal that has the same `ADMIN_TOKEN` used to start the API:

```bash
APPLICATION_ID='replace-with-application-id'
APPLICANT_TOKEN='replace-with-private-access-token'

curl "http://127.0.0.1:8080/api/v1/applications/$APPLICATION_ID" \
  -H "Authorization: Bearer $APPLICANT_TOKEN"

curl http://127.0.0.1:8080/api/v1/admin/applications \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Sandbox only: simulates completed checks; does not perform verification.
curl -X PUT "http://127.0.0.1:8080/api/v1/admin/applications/$APPLICATION_ID/checks" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"phone":true,"identity_and_age":true,"address":true,"business":true,"bank_account_ownership":true}'

# Choose an actual date today or later. The date below is only an example.
curl -X POST "http://127.0.0.1:8080/api/v1/admin/applications/$APPLICATION_ID/approve" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"first_due_date":"2027-01-31"}'

curl http://127.0.0.1:8080/api/v1/admin/outbox \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

That example schedules 31 January, 28 February, and 31 March 2027. Each amount is
7,500,000 kobo (NGN 75,000). Due dates are calendar dates, not promises that a bank
has processed funds. No funds move in this prototype.

The applicant's access token remains private. Do not use it in URLs, screenshots,
public logs, or SMS messages. The free approval reference is separate and grants
no authority to transfer funds.

To reject a different application still in `submitted` status:

```json
{"reason":"Business eligibility could not be established from the review."}
```

Submit that body to `POST /admin/applications/{id}/reject`. Approved and rejected
applications cannot be modified by this first version. Correction and appeal
workflows will be added in a later milestone.
