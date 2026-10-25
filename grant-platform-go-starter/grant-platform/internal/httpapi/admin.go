package httpapi

import (
	"encoding/csv"
	"fmt"
	"net/http"

	"grantplatform/internal/grants"
)

func (s *Server) adminMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := s.service.AdminMetrics()
	respond(w, http.StatusOK, metrics)
}

func (s *Server) adminList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	state := r.URL.Query().Get("state")
	category := r.URL.Query().Get("category")
	status := r.URL.Query().Get("status")

	apps := s.service.AdminListApplications(q, state, category, status)
	respond(w, http.StatusOK, apps)
}

func (s *Server) adminGetApp(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	app, err := s.service.AdminGetApplication(id)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, app)
}

func (s *Server) adminSetChecks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var checks grants.ApplicationChecks
	if !decode(w, r, &checks) {
		return
	}

	if err := s.service.AdminSetChecks(id, checks); err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "verification_checks_updated"})
}

func (s *Server) adminApproveApp(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		DisbursementDate string `json:"disbursement_date"`
		FirstDueDate     string `json:"first_due_date"` // Legacy compatibility
	}
	if !decode(w, r, &in) {
		return
	}

	date := in.DisbursementDate
	if date == "" {
		date = in.FirstDueDate
	}

	if err := s.service.AdminApprove(id, date); err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "approved"})
}

func (s *Server) adminRejectApp(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &in) {
		return
	}

	if err := s.service.AdminReject(id, in.Reason); err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "rejected"})
}

func (s *Server) adminDisburseApp(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.service.AdminDisburse(id); err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "disbursed"})
}

func (s *Server) adminExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	records := s.service.AdminExport()

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=solex_applications_export.csv")
		writer := csv.NewWriter(w)
		_ = writer.Write([]string{
			"Application ID", "Reference Code", "Applicant Name", "Phone", "Email",
			"Masked NIN", "Category", "CAC Number", "State", "LGA", "Business Type",
			"Status", "Grant Amount (Kobo)", "Disbursement Status", "Created At",
		})
		for _, rec := range records {
			_ = writer.Write([]string{
				rec.ApplicationID, rec.ReferenceCode, rec.ApplicantName, rec.Phone, rec.Email,
				rec.MaskedNIN, rec.Category, rec.CACNumber, rec.State, rec.LGA, rec.BusinessType,
				rec.Status, fmt.Sprintf("%d", rec.GrantAmountKobo), rec.DisbursementStatus, rec.CreatedAt,
			})
		}
		writer.Flush()
		return
	}

	respond(w, http.StatusOK, records)
}

func (s *Server) adminEmails(w http.ResponseWriter, r *http.Request) {
	emails := s.service.AdminEmails()
	respond(w, http.StatusOK, emails)
}

func (s *Server) adminResetApplicantPassword(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tempPassword, user, err := s.service.AdminResetApplicantPassword(id)
	if err != nil {
		fail(w, err)
		return
	}

	respond(w, http.StatusOK, map[string]any{
		"status":             "ok",
		"temporary_password": tempPassword,
		"user_id":            user.ID,
		"email":              user.Email,
		"full_name":          user.FullName,
		"applicant_name":     user.FullName,
		"applicant_email":    user.Email,
		"applicant_phone":    user.Phone,
		"message":            "Temporary password generated. Previous applicant sessions have been revoked.",
	})
}
