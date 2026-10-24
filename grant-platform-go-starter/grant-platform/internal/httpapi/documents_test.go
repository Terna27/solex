package httpapi

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"grantplatform/internal/grants"
)

func TestPrivateDocumentWorkflow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "grants.json")
	s, err := grants.New(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	in := grants.Input{FullName: "Sample Applicant", DOB: "1990-01-01", Phone: "+2348012345678", State: "Benue", LGA: "Makurdi", BusinessType: "Retail", BusinessDescription: "Fictional test shop only.", ResidentialAddress: "12 Sample Street, Test Town", BusinessName: "Sample Retail", BusinessAddress: "12 Sample Market", BankName: "Example Bank", AccountName: "Example Applicant", AccountNumber: "0123456789", Consent: true}
	a, token, err := s.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	admin := strings.Repeat("a", 40)
	h := New(s, admin)
	var raw bytes.Buffer
	if err = png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 10, 10))); err != nil {
		t.Fatal(err)
	}
	upload := func(credential, kind string, content []byte) *httptest.ResponseRecorder {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		_ = mw.WriteField("kind", kind)
		f, err := mw.CreateFormFile("file", "../../untrusted.png")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write(content)
		_ = mw.Close()
		r := httptest.NewRequest("POST", "/api/v1/applications/"+a.ID+"/documents", &body)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		r.Header.Set("Authorization", "Bearer "+credential)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := upload("wrong", "government_id", raw.Bytes()); w.Code != 404 {
		t.Fatal("unauthorised upload accepted")
	}
	if w := upload(token, "government_id", []byte("<html>not an image</html>")); w.Code != 400 {
		t.Fatal("spoofed image accepted")
	}
	if w := upload(token, "unknown", raw.Bytes()); w.Code != 400 {
		t.Fatal("invalid kind accepted")
	}
	if w := upload(token, "government_id", bytes.Repeat([]byte("x"), grants.MaxDocumentBytes+1)); w.Code != 400 {
		t.Fatal("oversize image accepted")
	}
	w := upload(token, "government_id", raw.Bytes())
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var doc grants.Document
	if err = json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	docPath := "/api/v1/applications/" + a.ID + "/documents/" + doc.ID
	if w = request(h, "GET", docPath, "", ""); w.Code != 404 {
		t.Fatal("unauthenticated download accepted")
	}
	in.Phone = "+2348012345679"
	_, other, err := s.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	if w = request(h, "GET", docPath, other, ""); w.Code != 404 {
		t.Fatal("other applicant accessed image")
	}
	if w = request(h, "GET", docPath, token, ""); w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("owner download failed")
	}
	adminPath := "/api/v1/admin/applications/" + a.ID + "/documents/" + doc.ID
	if w = request(h, "GET", adminPath, token, ""); w.Code != 401 {
		t.Fatal("applicant gained admin access")
	}
	if w = request(h, "GET", adminPath, admin, ""); w.Code != 200 {
		t.Fatal("admin could not review document")
	}
	if w = request(h, "GET", "/data/documents/"+doc.ID, "", ""); w.Code != 404 {
		t.Fatal("file public")
	}
	if err = s.SetChecks(a.ID, grants.Checks{Phone: true, Identity: true, Address: true, Business: true, Bank: true}); err != nil {
		t.Fatal(err)
	}
	if w = upload(token, "government_id", raw.Bytes()); w.Code != 201 {
		t.Fatal("replacement failed")
	}
	updated, err := s.Own(a.ID, token)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Documents) != 1 || updated.Checks.Identity {
		t.Fatal("replacement did not invalidate prior identity review")
	}
	if w = request(h, "GET", docPath, token, ""); w.Code != 404 {
		t.Fatal("superseded document still accessible")
	}
	restored, err := grants.New(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	restoredHandler := New(restored, admin)
	if w = request(restoredHandler, "GET", "/api/v1/applications/"+a.ID+"/documents/"+updated.Documents[0].ID, token, ""); w.Code != 200 {
		t.Fatal("document lost on restart")
	}
	if err = s.SetChecks(a.ID, grants.Checks{Phone: true, Identity: true, Address: true, Business: true, Bank: true}); err != nil {
		t.Fatal(err)
	}
	if w = upload(token, "passport_photo", raw.Bytes()); w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	if err = s.SetChecks(a.ID, grants.Checks{Phone: true, Identity: true, Address: true, Business: true, Bank: true}); err != nil {
		t.Fatal(err)
	}
	if err = s.Approve(a.ID, "2099-01-01"); err != nil {
		t.Fatal(err)
	}
	if w = upload(token, "proof_of_address", raw.Bytes()); w.Code != 409 {
		t.Fatal("approved document modification accepted")
	}
}
