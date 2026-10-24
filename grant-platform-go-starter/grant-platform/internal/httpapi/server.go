package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"grantplatform/internal/grants"
)

type Server struct {
	service   *grants.Service
	adminHash [32]byte
}

func New(service *grants.Service, adminToken string) http.Handler {
	s := &Server{service: service, adminHash: sha256.Sum256([]byte(adminToken))}
	m := http.NewServeMux()
	m.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]any{"status": "ok", "mode": "local_prototype", "live_transfers": false, "live_sms": false})
	})
	m.HandleFunc("GET /api/v1/programme", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]any{"monthly_amount_kobo": grants.MonthlyKobo, "months": 3, "total_amount_kobo": 3 * grants.MonthlyKobo, "currency": "NGN", "application_fee_kobo": 0, "minimum_age": 18, "states": grants.States, "lga_validation": "manual_review"})
	})
	m.HandleFunc("POST /api/v1/applications", s.create)
	m.HandleFunc("GET /api/v1/applications/{id}", s.own)
	m.Handle("GET /api/v1/admin/applications", s.admin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { respond(w, 200, s.service.List()) })))
	m.Handle("PUT /api/v1/admin/applications/{id}/checks", s.admin(http.HandlerFunc(s.checks)))
	m.Handle("POST /api/v1/admin/applications/{id}/approve", s.admin(http.HandlerFunc(s.approve)))
	m.Handle("POST /api/v1/admin/applications/{id}/reject", s.admin(http.HandlerFunc(s.reject)))
	m.Handle("GET /api/v1/admin/outbox", s.admin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { respond(w, 200, s.service.Outbox()) })))
	m.Handle("GET /api/v1/admin/audit", s.admin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { respond(w, 200, s.service.Audit()) })))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
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
	case errors.Is(err, grants.ErrNotFound):
		code = 404
	case errors.Is(err, grants.ErrConflict), errors.Is(err, grants.ErrCapacity):
		code = 409
	case errors.Is(err, grants.ErrStorage):
		code = 500
	}
	respond(w, code, map[string]string{"error": err.Error()})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		respond(w, 415, map[string]string{"error": "Content-Type must be application/json"})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		respond(w, 400, map[string]string{"error": "invalid JSON body or unknown fields"})
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
		token := bearer(r)
		hash := sha256.Sum256([]byte(token))
		if token == "" || subtle.ConstantTimeCompare(hash[:], s.adminHash[:]) != 1 {
			respond(w, 401, map[string]string{"error": "unauthorised"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
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
	respond(w, 201, map[string]any{"application": a, "access_token": t, "notice": "Local prototype: use fictional data. Save this private access token; it is shown only once. Phone and identity have not yet been verified."})
}
func (s *Server) own(w http.ResponseWriter, r *http.Request) {
	a, err := s.service.Own(r.PathValue("id"), bearer(r))
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, a)
}
func (s *Server) checks(w http.ResponseWriter, r *http.Request) {
	var c grants.Checks
	if !decode(w, r, &c) {
		return
	}
	if err := s.service.SetChecks(r.PathValue("id"), c); err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, map[string]string{"status": "sandbox_checks_recorded"})
}
func (s *Server) approve(w http.ResponseWriter, r *http.Request) {
	var in struct {
		FirstDueDate string `json:"first_due_date"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.service.Approve(r.PathValue("id"), in.FirstDueDate); err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, map[string]string{"status": "approved", "sms": "pending_provider_configuration", "transfers": "scheduled_only"})
}
func (s *Server) reject(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.service.Reject(r.PathValue("id"), in.Reason); err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, map[string]string{"status": "rejected"})
}
