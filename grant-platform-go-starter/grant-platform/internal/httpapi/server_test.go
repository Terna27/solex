package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	s, err := grants.New("", 1)
	if err != nil {
		t.Fatal(err)
	}
	admin := strings.Repeat("a", 40)
	h := New(s, admin)
	body := `{"full_name":"Example Applicant","date_of_birth":"1990-01-01","phone":"+2348012345678","state":"FCT","lga":"AMAC","business_type":"Retail","business_description":"A fictional retail shop","consent":true}`
	w := request(h, "POST", "/api/v1/applications", "", body)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct {
		Application grants.Application `json:"application"`
		AccessToken string             `json:"access_token"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/applications/" + result.Application.ID
	if w = request(h, "GET", path, "", ""); w.Code != 404 {
		t.Fatal("application exposed")
	}
	if w = request(h, "GET", path, result.AccessToken, ""); w.Code != 200 {
		t.Fatal("applicant denied")
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
	s, _ := grants.New("", 1)
	h := New(s, strings.Repeat("a", 40))
	for _, body := range []string{`{"status":"approved"}`, `{} {}`, strings.Repeat("x", 17000), `null`} {
		if w := request(h, "POST", "/api/v1/applications", "", body); w.Code != 400 {
			t.Fatal("bad input accepted")
		}
	}
}
