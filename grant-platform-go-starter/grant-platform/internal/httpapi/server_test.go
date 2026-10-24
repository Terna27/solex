package httpapi

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"grantplatform/internal/grants"
)

func request(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestHTTPWorkflowAndPrivacy(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 1)
	if err != nil {
		t.Fatal(err)
	}
	admin := strings.Repeat("a", 40)
	h := New(s, admin)
	body := `{"full_name":"Example Applicant","date_of_birth":"1990-01-01","phone":"+2348012345678","nin":"01234567890","state":"FCT","lga":"AMAC","business_type":"Retail","business_description":"A fictional retail shop","consent":true,"residential_address":"12 Sample Street, Test Town","business_name":"Sample Retail","business_address":"12 Sample Market","bank_name":"Access Bank","account_name":"Example Applicant","account_number":"0123456789"}`
	w := request(h, "POST", "/api/v1/applications", "", body)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct {
		Application grants.Application `json:"application"`
		AccessToken string             `json:"upload_token"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/applications/" + result.Application.ID
	if w = request(h, "GET", path, "", ""); w.Code != 404 {
		t.Fatal("application exposed")
	}
	if w = request(h, "GET", path, result.AccessToken, ""); w.Code != 404 {
		t.Fatal("applicant lookup should be disabled")
	}
	if w = request(h, "GET", "/api/v1/admin/applications", result.AccessToken, ""); w.Code != 401 {
		t.Fatal("privilege escalation")
	}
	if w = request(h, "GET", "/api/v1/admin/applications", admin, ""); w.Code != 200 {
		t.Fatal("admin denied")
	}
	if bytes.Contains(w.Body.Bytes(), []byte(result.AccessToken)) {
		t.Fatal("access token leaked")
	}
	p := "/api/v1/admin/applications/" + result.Application.ID
	if w = request(h, "POST", p+"/approve", admin, `{"first_due_date":"2099-01-01"}`); w.Code != 400 {
		t.Fatal("unverified approved")
	}
	var picture bytes.Buffer
	if err = png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"passport_photo", "proof_of_address"} {
		if _, err = s.UploadDocument(result.Application.ID, result.AccessToken, kind, picture.Bytes()); err != nil {
			t.Fatal(err)
		}
	}
	if w = request(h, "PUT", p+"/checks", admin, `{"phone":true,"identity_and_age":true,"address":true,"business":true,"bank_account_ownership":true}`); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = request(h, "POST", p+"/approve", admin, `{"first_due_date":"2099-01-01"}`); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = request(h, "POST", p+"/approve", admin, `{"first_due_date":"2099-01-01"}`); w.Code != 409 {
		t.Fatal("duplicate approval")
	}
}
func TestStrictRequestBodies(t *testing.T) {
	s, _ := grants.New(filepath.Join(t.TempDir(), "grants.json"), 1)
	h := New(s, strings.Repeat("a", 40))
	for _, body := range []string{`{"status":"approved"}`, `{} {}`, strings.Repeat("x", 17000), `null`} {
		if w := request(h, "POST", "/api/v1/applications", "", body); w.Code != 400 {
			t.Fatal("bad input accepted")
		}
	}
}

func TestEmbeddedFrontend(t *testing.T) {
	s, _ := grants.New(filepath.Join(t.TempDir(), "grants.json"), 1)
	h := New(s, strings.Repeat("a", 40))
	for _, path := range []string{"/", "/style.css", "/app.js"} {
		w := request(h, "GET", path, "", "")
		if w.Code != 200 || w.Body.Len() == 0 {
			t.Fatalf("frontend asset %s: %d", path, w.Code)
		}
	}
	if w := request(h, "GET", "/data/grants.json", "", ""); w.Code != 404 {
		t.Fatal("data path exposed")
	}
}

func TestApprovalLinksArePrivateAndRevocable(t *testing.T) {
	s, err := grants.New(filepath.Join(t.TempDir(), "grants.json"), 2)
	if err != nil {
		t.Fatal(err)
	}
	admin := strings.Repeat("a", 40)
	h := New(s, admin)
	body := `{"full_name":"Example Applicant","date_of_birth":"1990-01-01","phone":"+2348012345678","nin":"01234567890","state":"FCT","lga":"AMAC","business_type":"Retail","business_description":"A fictional retail shop","consent":true,"residential_address":"12 Sample Street, Test Town","business_name":"Sample Retail","business_address":"12 Sample Market","bank_name":"Access Bank","account_name":"Example Applicant","account_number":"0123456789"}`
	w := request(h, "POST", "/api/v1/applications", "", body)
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	var created struct {
		Application struct {
			ID string `json:"id"`
		} `json:"application"`
		UploadToken string `json:"upload_token"`
		Reference   string `json:"reference"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(w.Body.String(), "access_token") {
		t.Fatal("legacy login credential exposed")
	}
	path := "/api/v1/admin/applications/" + created.Application.ID + "/approval-link"
	if w = request(h, "POST", path, admin, ""); w.Code != 409 {
		t.Fatal("unapproved link generated", w.Code)
	}
	var picture bytes.Buffer
	if err = png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"passport_photo", "government_id"} {
		if _, err = s.UploadDocument(created.Application.ID, created.UploadToken, kind, picture.Bytes()); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.SetChecks(created.Application.ID, grants.Checks{Phone: true, Identity: true, Address: true, Business: true, Bank: true}); err != nil {
		t.Fatal(err)
	}
	if err = s.Approve(created.Application.ID, "2099-01-01"); err != nil {
		t.Fatal(err)
	}
	if w = request(h, "POST", path, "", ""); w.Code != 401 {
		t.Fatal("unauthenticated link generation")
	}
	if w = request(h, "POST", path, admin, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var first struct {
		Token string `json:"token"`
		Path  string `json:"path"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if w = request(h, "GET", "/api/v1/approval/"+first.Token, "", ""); w.Code != 200 {
		t.Fatal("approval notice missing")
	}
	if bytes.Contains(w.Body.Bytes(), []byte("01234567890")) || bytes.Contains(w.Body.Bytes(), []byte("0123456789")) || bytes.Contains(w.Body.Bytes(), []byte("Example Applicant")) {
		t.Fatal("private data leaked")
	}
	if w = request(h, "POST", path, admin, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = request(h, "GET", "/api/v1/approval/"+first.Token, "", ""); w.Code != 404 {
		t.Fatal("old link not revoked")
	}
}
