package httpapi

import (
	"net/http"

	"grantplatform/internal/grants"
)

func (s *Server) getProfile(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	if user == nil {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorised"})
		return
	}

	profile, err := s.service.GetProfile(user.ID)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, profile)
}

func (s *Server) saveProfile(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	if user == nil {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorised"})
		return
	}

	var p grants.ApplicantProfile
	if !decode(w, r, &p) {
		return
	}

	updated, err := s.service.SaveProfile(user.ID, p)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"profile": updated,
		"message": "Profile saved successfully.",
	})
}

// Legacy profile update endpoint for existing tests
func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request) {
	var p grants.Profile
	if !decode(w, r, &p) {
		return
	}
	if err := s.service.UpdateProfile(r.PathValue("id"), bearer(r), p); err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "profile_updated"})
}
