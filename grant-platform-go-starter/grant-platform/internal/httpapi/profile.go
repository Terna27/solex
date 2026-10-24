package httpapi

import (
	"grantplatform/internal/grants"
	"net/http"
)

func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request) {
	var p grants.Profile
	if !decode(w, r, &p) {
		return
	}
	if err := s.service.UpdateProfile(r.PathValue("id"), bearer(r), p); err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, map[string]string{"status": "profile_updated"})
}
