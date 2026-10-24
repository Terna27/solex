# Private document uploads

## Install

Stop the server with Ctrl+C. Extract `solex-document-upload-update.zip` into the
existing folder containing `go.mod`, allow source-file replacements, and restart
with `go run ./cmd/api`. Keep the same DATA_FILE and working directory to preserve
existing applications. No new environment variables or dependencies are required.

## Applicant

Open My application and enter your existing application ID and access token.
Below the status summary, choose Passport photograph, Government-issued ID or Proof of address and
select a sample JPEG/PNG. Click Upload document. Use fictional samples only.

- Maximum source file size: 5 MB. Maximum image size: 12 million pixels.
- Files are validated by decoding, not by their filename or claimed MIME type.
- Images are re-encoded to remove metadata and appended content.
- One active image per document type. Re-uploading replaces it and resets the
  matching identity/address review flag. Previously approved/rejected applications
  cannot change their documents in this version.
- Uploading does not verify age or identity and does not approve an application.

## Administrator

Open Admin workspace, enter the same ADMIN_TOKEN used by the running server,
select an application, and scroll to Supporting documents. Use View image to
open a private preview. Review the image before changing the relevant check.
Live verification, OCR and automated bank-account verification are not connected.
Approval now requires a passport photograph and one ID-card or proof-of-address
image, complete bank/address details, and all review checks.

## Storage and access

Files are stored under `documents/` beside the configured data JSON file (normally
`data/documents`). Random server-generated filenames are used. Original filenames
are neither trusted nor stored. Directory permissions are 0700; files are 0600.
Back up the entire data directory together. Do not commit it to Git.

Metadata is included in the authorised application response. Image bytes are
available only through authenticated endpoints. An applicant token may access only
its own application; admin routes require ADMIN_TOKEN. No public upload URL exists.
The browser creates a temporary image URL for viewing and revokes it on closing.

POST `/api/v1/applications/{id}/documents`: applicant bearer token; multipart fields
`kind` (`passport_photo`, `government_id` or `proof_of_address`) and `file`.

GET `/api/v1/applications/{id}/documents/{documentID}`: applicant bearer token.

GET `/api/v1/admin/applications/{id}/documents/{documentID}`: admin bearer token.

There is no PDF support, cloud storage, malware scanning, retention/deletion job,
or encryption at rest in this prototype. Review these before collecting real
documents. A process crash between file and metadata persistence can leave an
unreferenced private file; production storage needs orphan cleanup and backups.

## Validation

Tests exercise upload and retrieval, restart persistence, MIME spoofing, size
limits, document replacement, review invalidation, terminal-state protection,
anonymous access denial, cross-applicant access denial, and admin access.

The current milestone remains local-only. No SMS or transfers are enabled.
