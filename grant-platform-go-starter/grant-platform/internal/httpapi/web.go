package httpapi

import (
	"embed"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed web/*
var webFiles embed.FS

func frontend() http.Handler {
	files, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(files))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanPath := strings.TrimPrefix(r.URL.Path, "/")
		if cleanPath == "" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Never serve data or private folders through frontend handler
		if strings.HasPrefix(cleanPath, "data/") || strings.HasPrefix(cleanPath, "data") {
			http.NotFound(w, r)
			return
		}

		// If the file exists in the embedded filesystem, serve it directly
		if f, err := files.Open(cleanPath); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// If path has a file extension and was not found in files, it is a missing asset -> 404
		if strings.Contains(filepath.Base(cleanPath), ".") {
			http.NotFound(w, r)
			return
		}

		// SPA routes (e.g. /admin/login, /admin/dashboard, /admin, /login, /register) serve index.html
		if isSPARoute(cleanPath) {
			r2 := new(http.Request)
			*r2 = *r
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}

		http.NotFound(w, r)
	})
}

func isSPARoute(path string) bool {
	if strings.HasPrefix(path, "admin") {
		return true
	}
	switch path {
	case "login", "register", "dashboard", "apply", "about", "eligibility", "contact", "home":
		return true
	default:
		return false
	}
}
