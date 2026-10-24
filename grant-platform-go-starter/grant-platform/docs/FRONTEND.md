# Frontend update

The application now includes a responsive frontend embedded directly in the Go
binary. There is no npm install, separate frontend server, or CORS configuration.

## Install this update

1. Stop your running server with Ctrl+C.
2. Extract `solex-frontend-update.zip` into the existing folder containing `go.mod`.
   Allow replacement of the source files in this update. Keep your `data` folder.
3. From that same folder and terminal, run `go run ./cmd/api`.
4. Open http://127.0.0.1:8080 in your browser. Refresh if necessary.

The update contains no data file, credentials, or environment settings. Existing
applications remain in `data/grants.json` when the server runs from the same folder.

## Screens

- Overview: programme details, amount, eligibility, and application entry points.
- Apply for support: submits to the existing Go application API.
- My application: accepts application ID and private access token; shows actual
  status, rejection reason, reference code, and instalments returned by the API.
- Admin workspace: accepts the server's ADMIN_TOKEN, lists and filters applicants,
  records sandbox review checks, approves with a first due date, or rejects with a
  reason. Approval still requires the backend verification checks to be complete.

Tokens stay in page memory only, not localStorage, cookies, or URLs. Save the
applicant token when it is issued; refresh/sign-out clears the relevant UI session.
Existing API-created applications can be opened using their existing ID and token.

This is still a local prototype. Private sample-image uploads are supported;
no SMS is sent and no money moves. The UI labels these limitations and does not display a scheduled
instalment as a successful transfer. Admin review flags remain simulated.

Branding uses “Solex” as a working name based on the repository. All style assets
are local; the pages require no external font, image, or JavaScript service.

## Verification

Go route/domain tests and build checks cover the backend and embedded assets.
JavaScript syntax is checked with `node --check internal/httpapi/web/app.js`.
Interactive browser behaviour and layout should also be reviewed locally before
this prototype is extended or published.
