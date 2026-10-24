# Complete application form

This update adds full residential address, business/trading name, business address,
bank name, account-holder name, and a 10-digit account number. Account numbers are
stored as strings so leading zeros are preserved. Bank names are entered as text;
bank selection and account-name resolution will need a provider integration.

The application page now requires a passport photograph (a portrait photo) and
one supporting image: government-issued ID OR proof of address such as a utility
bill. Images must be JPEG or PNG, at most 5 MB and 12 million pixels each. PDFs
are not supported. Passport means a photograph, not a passport booklet.

The applicant submits one form. The details are saved first, then images are
uploaded to the private endpoints. Credentials are displayed immediately after
details are saved. If an image fails, the page reports which upload failed and
directs the applicant to My application to retry, without registering again.
This is not one atomic submission; interrupted uploads can leave an incomplete
application. Approval is blocked until required profile fields and image types
are present, as well as all existing verification checks.

## Existing applications

Old records remain readable. Open My application with the original ID/token, then
use Complete or update your details to add the new fields. Upload the portrait and
supporting image below it. Changing bank information resets the bank-review flag;
changing residential/business addresses resets the corresponding review flag.
Approved and rejected records cannot be edited by this version.

Already approved legacy records are not retroactively changed. Administrators
should review them separately before any future live disbursement integration.

## Administrative review

The admin's application detail view includes the new bank, address and business
fields. The supporting-document section shows all uploaded images, including the
portrait. Presence of an image is not identity verification. A utility bill alone
may not establish age or identity; the reviewer must establish those separately.
No automatic bank verification, PIN, banking OTP, or BVN collection is implemented.

## API changes

New POST `/api/v1/applications` requests must include:

```json
{
  "residential_address": "12 Sample Street, Example Town",
  "business_name": "Example Retail Shop",
  "business_address": "12 Sample Market, Example Town",
  "bank_name": "Example Bank",
  "account_name": "Example Applicant",
  "account_number": "0123456789"
}
```

These fields are additional to the existing applicant fields. PUT
`/api/v1/applications/{id}/profile` accepts exactly these six fields, requires
the applicant bearer token and permits edits only while status is submitted.
Uploads additionally accept `kind=passport_photo`.

## Install

Stop the existing server, extract `solex-complete-application-update.zip` into
the folder containing go.mod, allow source replacements, and restart
`go run ./cmd/api` in the same terminal and directory. Refresh the browser.
Existing data and uploaded images are not included in the update ZIP and are
preserved in your data directory. No new setup variables are needed.

Tests cover required fields, leading zeros, invalid account numbers, approval
requirements, edit ownership, verification invalidation, and legacy records.
Use fictional data only: this local prototype does not encrypt bank details at
rest or provide production identity/transfer integrations.
