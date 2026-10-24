package httpapi

import (
	"encoding/hex"
	"grantplatform/internal/grants"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestSingleAdminPasswordSession(t *testing.T) {
	salt := []byte("0123456789abcdef")
	encoded := "pbkdf2_sha256$100000$" + hex.EncodeToString(salt) + "$" + hex.EncodeToString(derive("correct-horse-password", salt, 100000))
	service, _ := grants.New(filepath.Join(t.TempDir(), "grants.json"), 2)
	h, err := NewWithAdmin(service, "admin@example.test", encoded)
	if err != nil {
		t.Fatal(err)
	}
	bad := request(h, "POST", "/api/v1/admin/login", "", `{"email":"admin@example.test","password":"wrong"}`)
	if bad.Code != 401 {
		t.Fatalf("bad login: %d", bad.Code)
	}
	old := request(h, "GET", "/api/v1/admin/applications", strings.Repeat("a", 40), "")
	if old.Code != 401 {
		t.Fatal("legacy bearer token accepted")
	}
	good := request(h, "POST", "/api/v1/admin/login", "", `{"email":"admin@example.test","password":"correct-horse-password"}`)
	if good.Code != 200 {
		t.Fatalf("login: %d %s", good.Code, good.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range good.Result().Cookies() {
		if c.Name == sessionCookie {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly {
		t.Fatal("missing secure session cookie")
	}
	send := func(method, path string, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.AddCookie(cookie)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	if w := send("GET", "/api/v1/admin/applications", ""); w.Code != 200 {
		t.Fatalf("session denied: %d", w.Code)
	}
	if w := send("POST", "/api/v1/admin/logout", "http://attacker.example"); w.Code != 403 {
		t.Fatal("cross origin logout accepted")
	}
	if w := send("POST", "/api/v1/admin/logout", ""); w.Code != 200 {
		t.Fatal("logout failed")
	}
	if w := send("GET", "/api/v1/admin/applications", ""); w.Code != 401 {
		t.Fatal("session still active after logout")
	}
}
