package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"grantplatform/internal/grants"
)

type Server struct {
	service      *grants.Service
	adminHash    [32]byte
	adminToken   string
	loginLimiter *RateLimiter
	adminLimiter *RateLimiter
}

func New(service *grants.Service, adminToken string) http.Handler {
	s := &Server{
		service:      service,
		adminHash:    sha256.Sum256([]byte(adminToken)),
		adminToken:   adminToken,
		loginLimiter: NewRateLimiter(10, 1*time.Minute),
		adminLimiter: NewRateLimiter(5, 1*time.Minute),
	}

	m := http.NewServeMux()

	// Static embedded frontend
	m.Handle("GET /", frontend())

	// Public programme & health endpoints
	m.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		mode := os.Getenv("APP_MODE")
		if mode == "" {
			mode = "local_prototype"
		}
		respond(w, 200, map[string]any{
			"status":              "ok",
			"mode":                mode,
			"commencement_date":   grants.GetCommencementDate(),
			"disbursement_date":   grants.GetDisbursementDate(),
			"live_transfers":      false,
			"live_sms":            false,
		})
	})

	m.HandleFunc("GET /api/v1/programme", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, grants.GetProgrammeConfig())
	})

	m.HandleFunc("GET /api/v1/fee-disclosure", s.feeDisclosure)

	// Authentication
	m.HandleFunc("POST /api/v1/auth/register", s.register)
	m.HandleFunc("POST /api/v1/auth/login", s.login)
	m.HandleFunc("POST /api/v1/auth/logout", s.logout)
	m.HandleFunc("GET /api/v1/auth/me", s.me)

	// Applicant Profile
	m.HandleFunc("GET /api/v1/profile", s.requireAuth(s.getProfile))
	m.HandleFunc("PUT /api/v1/profile", s.requireAuth(s.saveProfile))

	// Applicant Applications
	m.HandleFunc("GET /api/v1/applications/current", s.requireAuth(s.getCurrentApplication))
	m.HandleFunc("POST /api/v1/applications/start", s.requireAuth(s.startApplication))
	m.HandleFunc("POST /api/v1/applications/ready-for-payment", s.requireAuth(s.readyForPayment))
	m.HandleFunc("POST /api/v1/applications/submit", s.requireAuth(s.submitApplication))

	// Documents
	m.HandleFunc("POST /api/v1/applications/{id}/documents", s.uploadDocument)
	m.HandleFunc("GET /api/v1/applications/{id}/documents/{documentID}", s.applicantDocument)

	// Payments
	m.HandleFunc("POST /api/v1/payments/init", s.requireAuth(s.initPayment))
	m.HandleFunc("POST /api/v1/payments/verify", s.requireAuth(s.verifyPayment))

	// Admin Authentication
	m.HandleFunc("POST /api/v1/admin/auth/login", s.adminLogin)
	m.HandleFunc("POST /api/v1/admin/auth/logout", s.adminLogout)
	m.HandleFunc("GET /api/v1/admin/auth/me", s.adminMe)

	// Admin workspace
	m.Handle("GET /api/v1/admin/metrics", s.admin(http.HandlerFunc(s.adminMetrics)))
	m.Handle("GET /api/v1/admin/applications", s.admin(http.HandlerFunc(s.adminList)))
	m.Handle("GET /api/v1/admin/applications/{id}", s.admin(http.HandlerFunc(s.adminGetApp)))
	m.Handle("PUT /api/v1/admin/applications/{id}/checks", s.admin(http.HandlerFunc(s.adminSetChecks)))
	m.Handle("POST /api/v1/admin/applications/{id}/approve", s.admin(http.HandlerFunc(s.adminApproveApp)))
	m.Handle("POST /api/v1/admin/applications/{id}/reject", s.admin(http.HandlerFunc(s.adminRejectApp)))
	m.Handle("POST /api/v1/admin/applications/{id}/disburse", s.admin(http.HandlerFunc(s.adminDisburseApp)))
	m.Handle("GET /api/v1/admin/applications/{id}/documents/{documentID}", s.admin(http.HandlerFunc(s.document)))
	m.Handle("GET /api/v1/admin/export", s.admin(http.HandlerFunc(s.adminExport)))
	m.Handle("GET /api/v1/admin/emails", s.admin(http.HandlerFunc(s.adminEmails)))
	m.Handle("GET /api/v1/admin/outbox", s.admin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { respond(w, 200, s.service.Outbox()) })))
	m.Handle("GET /api/v1/admin/audit", s.admin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { respond(w, 200, s.service.Audit()) })))

	// Legacy endpoints for backward compatibility with existing tests
	m.HandleFunc("POST /api/v1/applications", s.create)
	m.HandleFunc("GET /api/v1/applications/{id}", s.own)
	m.HandleFunc("PUT /api/v1/applications/{id}/profile", s.updateProfile)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS support for static hosting / separate frontends
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-CSRF-Token")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		m.ServeHTTP(w, r)
	})
}

func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, err error) {
	code := 400
	switch {
	case errors.Is(err, grants.ErrNotFound), errors.Is(err, grants.ErrUserNotFound), errors.Is(err, grants.ErrPaymentNotFound):
		code = 404
	case errors.Is(err, grants.ErrUnauthorized), errors.Is(err, grants.ErrInvalidCredentials):
		code = 401
	case errors.Is(err, grants.ErrConflict), errors.Is(err, grants.ErrCapacity),
		errors.Is(err, grants.ErrDuplicateEmail), errors.Is(err, grants.ErrDuplicatePhone),
		errors.Is(err, grants.ErrDuplicateCAC), errors.Is(err, grants.ErrActiveAppExists),
		errors.Is(err, grants.ErrPaymentAlreadyPaid):
		code = 409
	case errors.Is(err, grants.ErrStorage):
		code = 500
	case errors.Is(err, grants.ErrRateLimited):
		code = 429
	}
	respond(w, code, map[string]string{"error": err.Error()})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		respond(w, 415, map[string]string{"error": "Content-Type must be application/json"})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		respond(w, 400, map[string]string{"error": "invalid JSON body: " + err.Error()})
		return false
	}
	if d.Decode(new(any)) != io.EOF {
		respond(w, 400, map[string]string{"error": "send exactly one JSON object"})
		return false
	}
	return true
}

func bearer(r *http.Request) string {
	v := r.Header.Get("Authorization")
	if !strings.HasPrefix(v, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(v, "Bearer ")
}

func (s *Server) admin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.extractAdminSessionToken(r)
		if token == "" {
			respond(w, 401, map[string]string{"error": "unauthorised"})
			return
		}

		// Check legacy admin token
		legacyHash := sha256.Sum256([]byte(token))
		isLegacy := subtle.ConstantTimeCompare(legacyHash[:], s.adminHash[:]) == 1

		// Check active admin session
		_, err := s.service.AuthenticateAdminSession(token)
		isAdminSession := err == nil

		if !isLegacy && !isAdminSession {
			respond(w, 401, map[string]string{"error": "unauthorised"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Legacy application submission handler
func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var in grants.Input
	if !decode(w, r, &in) {
		return
	}
	a, t, err := s.service.Create(in)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 201, map[string]any{
		"application":  a,
		"access_token": t,
		"notice":       "Local prototype: use fictional data. Save this private access token; it is shown only once. Phone and identity have not yet been verified.",
	})
}

func (s *Server) own(w http.ResponseWriter, r *http.Request) {
	a, err := s.service.Own(r.PathValue("id"), bearer(r))
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, a)
}
