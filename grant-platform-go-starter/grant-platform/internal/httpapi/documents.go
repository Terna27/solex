package httpapi

import (
	"grantplatform/internal/grants"
	"io"
	"net/http"
)

func (s *Server) uploadDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.service.Own(id, bearer(r)); err != nil {
		fail(w, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, grants.MaxDocumentBytes+65536)
	if err := r.ParseMultipartForm(grants.MaxDocumentBytes + 65536); err != nil {
		respond(w, 400, map[string]string{"error": "upload one JPEG or PNG image, up to 5 MB"})
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
		respond(w, 400, map[string]string{"error": "upload exactly one file"})
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		respond(w, 400, map[string]string{"error": "file is required"})
		return
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, grants.MaxDocumentBytes+1))
	if err != nil {
		respond(w, 400, map[string]string{"error": "could not read uploaded image"})
		return
	}
	doc, err := s.service.UploadDocument(id, bearer(r), r.FormValue("kind"), raw)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 201, doc)
}
func (s *Server) applicantDocument(w http.ResponseWriter, r *http.Request) {
	if _, err := s.service.Own(r.PathValue("id"), bearer(r)); err != nil {
		fail(w, err)
		return
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
	w.WriteHeader(200)
	_, _ = w.Write(content)
}
