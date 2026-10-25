package httpapi

import (
	"net/http"
)

func (s *Server) getCurrentApplication(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	if user == nil {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorised"})
		return
	}

	app, err := s.service.GetCurrentApplication(user.ID)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, app)
}

func (s *Server) startApplication(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	if user == nil {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorised"})
		return
	}

	var in struct {
		Category string `json:"category"`
	}
	if !decode(w, r, &in) {
		return
	}

	app, err := s.service.StartApplication(user.ID, in.Category)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusCreated, app)
}

func (s *Server) readyForPayment(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	if user == nil {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorised"})
		return
	}

	var in struct {
		ApplicationID string `json:"application_id"`
	}
	if !decode(w, r, &in) {
		return
	}

	app, err := s.service.TransitionToAwaitingPayment(user.ID, in.ApplicationID)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, app)
}

func (s *Server) submitApplication(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	if user == nil {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorised"})
		return
	}

	var in struct {
		ApplicationID string `json:"application_id"`
	}
	if !decode(w, r, &in) {
		return
	}

	app, err := s.service.SubmitApplication(user.ID, in.ApplicationID)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"application":    app,
		"reference_code": app.ReferenceCode,
		"message":        "Your application has been successfully submitted.",
	})
}
