package httpapi

import (
	"io"
	"net/http"

	"grantplatform/internal/grants"
)

func (s *Server) uploadDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user := s.currentUser(r)

	// Check if legacy bearer token or user session is provided
	legacyToken := bearer(r)
	if user == nil && legacyToken == "" {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorised"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, grants.MaxDocumentBytes+65536)
	if err := r.ParseMultipartForm(grants.MaxDocumentBytes + 65536); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "upload one JPEG or PNG image, up to 5 MB"})
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	count := 0
	for _, files := range r.MultipartForm.File {
		count += len(files)
	}
	if count != 1 {
		respond(w, http.StatusBadRequest, map[string]string{"error": "upload exactly one file"})
		return
	}

	f, _, err := r.FormFile("file")
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "file is required"})
		return
	}
	defer f.Close()

	raw, err := io.ReadAll(io.LimitReader(f, grants.MaxDocumentBytes+1))
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": "could not read uploaded image"})
		return
	}

	kind := r.FormValue("kind")

	var doc grants.Document
	if user != nil {
		doc, err = s.service.UploadUserDocument(user.ID, id, kind, raw)
	} else {
		doc, err = s.service.UploadDocument(id, legacyToken, kind, raw)
	}

	if err != nil {
		fail(w, err)
		return
	}
	respond(w, http.StatusCreated, doc)
}

func (s *Server) applicantDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user := s.currentUser(r)
	legacyToken := bearer(r)

	if user != nil {
		app, err := s.service.GetCurrentApplication(user.ID)
		if err != nil || app.ID != id {
			respond(w, http.StatusNotFound, map[string]string{"error": "application not found"})
			return
		}
	} else {
		if _, err := s.service.Own(id, legacyToken); err != nil {
			fail(w, err)
			return
		}
	}

	s.document(w, r)
}

func (s *Server) document(w http.ResponseWriter, r *http.Request) {
	d, content, err := s.service.ReadDocument(r.PathValue("id"), r.PathValue("documentID"))
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", d.MIME)
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}
