package httpapi

import (
	"net/http"
	"os"

	"grantplatform/internal/grants"
)

func (s *Server) feeDisclosure(w http.ResponseWriter, r *http.Request) {
	isProd := os.Getenv("APP_MODE") == "production"
	disclosure := grants.GetFeeDisclosure(isProd)
	respond(w, http.StatusOK, disclosure)
}

func (s *Server) initPayment(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	if user == nil {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorised"})
		return
	}

	var in grants.PaymentInitInput
	if !decode(w, r, &in) {
		return
	}

	payment, err := s.service.InitPayment(user.ID, in.ApplicationID)
	if err != nil {
		fail(w, err)
		return
	}

	isProd := os.Getenv("APP_MODE") == "production"
	respond(w, http.StatusOK, map[string]any{
		"payment": payment,
		"disclosure": grants.GetFeeDisclosure(isProd),
		"sandbox_active": !isProd,
	})
}

func (s *Server) verifyPayment(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	if user == nil {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorised"})
		return
	}

	var in grants.PaymentVerifyInput
	if !decode(w, r, &in) {
		return
	}

	isProd := os.Getenv("APP_MODE") == "production"
	payment, app, err := s.service.VerifyPayment(user.ID, in.Reference, in.SimulatorAction, isProd)
	if err != nil {
		fail(w, err)
		return
	}

	respond(w, http.StatusOK, map[string]any{
		"payment":     payment,
		"application": app,
		"message":     "Payment verified successfully.",
	})
}
