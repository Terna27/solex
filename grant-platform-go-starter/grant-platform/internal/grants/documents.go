package grants

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"time"
)

const MaxDocumentBytes = 5 * 1024 * 1024

type Document struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	MIME       string    `json:"mime_type"`
	Size       int       `json:"size_bytes"`
	UploadedAt time.Time `json:"uploaded_at"`
}

// NormaliseDocument decodes and re-encodes images so filenames, EXIF metadata and
// appended content are never served. Size and pixel limits bound image decoding.
func NormaliseDocument(raw []byte) ([]byte, string, error) {
	if len(raw) == 0 || len(raw) > MaxDocumentBytes {
		return nil, "", errors.New("choose an image no larger than 5 MB")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "jpeg" && format != "png") {
		return nil, "", errors.New("only valid JPEG and PNG images are accepted")
	}
	if config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 12000000 {
		return nil, "", errors.New("image must contain no more than 12 million pixels")
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, "", errors.New("image could not be decoded")
	}
	var out bytes.Buffer
	mime := "image/jpeg"
	if format == "png" {
		err = png.Encode(&out, img)
		mime = "image/png"
	} else {
		err = jpeg.Encode(&out, img, &jpeg.Options{Quality: 90})
	}
	if err != nil || out.Len() > MaxDocumentBytes {
		return nil, "", errors.New("image is too large after processing; use a smaller image")
	}
	return out.Bytes(), mime, nil
}

func isValidDocKind(kind string) bool {
	return kind == DocPassportPhoto || kind == DocGovernmentID || kind == DocProofOfAddress || kind == DocCACCertificate
}

// UploadDocument supports legacy token-based document uploads.
func (s *Service) UploadDocument(id, token, kind string, raw []byte) (Document, error) {
	if !isValidDocKind(kind) {
		return Document{}, errors.New("choose passport_photo, government_id, proof_of_address, or cac_certificate")
	}
	if _, err := s.Own(id, token); err != nil {
		return Document{}, err
	}
	content, mime, err := NormaliseDocument(raw)
	if err != nil {
		return Document{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.db.Applications[id]
	if !ok || token == "" || s.db.TokenHashes[id] != tokenHash(token) {
		return Document{}, ErrNotFound
	}
	if a.Status != StatusSubmitted && a.Status != StatusDraft && a.Status != StatusAwaitingPayment {
		return Document{}, ErrConflict
	}
	if s.path == "" {
		return Document{}, ErrStorage
	}
	documentID, err := randomID()
	if err != nil {
		return Document{}, ErrStorage
	}
	dir := filepath.Join(filepath.Dir(s.path), "documents")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return Document{}, ErrStorage
	}
	path := filepath.Join(dir, documentID)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return Document{}, ErrStorage
	}
	_, err = f.Write(content)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		os.Remove(path)
		return Document{}, ErrStorage
	}
	doc := Document{documentID, kind, mime, len(content), s.now().UTC()}
	oldID := ""
	err = s.mutate(func(db *database) error {
		app := db.Applications[id]
		docs := []Document{}
		for _, d := range app.Documents {
			if d.Kind == kind {
				oldID = d.ID
			} else {
				docs = append(docs, d)
			}
		}
		app.Documents = append(docs, doc)
		if kind == DocPassportPhoto || kind == DocGovernmentID {
			app.Checks.Identity = false
		} else if kind == DocProofOfAddress {
			app.Checks.Address = false
		} else if kind == DocCACCertificate {
			app.Checks.CAC = false
		}
		db.Applications[id] = app
		db.Events = append(db.Events, Event{
			ApplicationID: id,
			Actor:         "legacy_token",
			Action:        "document_uploaded_" + kind,
			At:            s.now().UTC(),
		})
		return nil
	})
	if err != nil {
		os.Remove(path)
		return Document{}, err
	}
	if oldID != "" {
		_ = os.Remove(filepath.Join(dir, oldID))
	}
	return doc, nil
}

// UploadUserDocument uploads a document for an authenticated user's application.
func (s *Service) UploadUserDocument(userID, appID, kind string, raw []byte) (Document, error) {
	if !isValidDocKind(kind) {
		return Document{}, errors.New("choose passport_photo, government_id, proof_of_address, or cac_certificate")
	}
	content, mime, err := NormaliseDocument(raw)
	if err != nil {
		return Document{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.db.Applications[appID]
	if !ok || a.UserID != userID {
		return Document{}, ErrNotFound
	}
	if a.Status != StatusDraft && a.Status != StatusAwaitingPayment && a.Status != StatusSubmitted {
		return Document{}, ErrConflict
	}
	if s.path == "" {
		return Document{}, ErrStorage
	}

	documentID, err := randomID()
	if err != nil {
		return Document{}, ErrStorage
	}

	dir := filepath.Join(filepath.Dir(s.path), "documents")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return Document{}, ErrStorage
	}
	path := filepath.Join(dir, documentID)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return Document{}, ErrStorage
	}
	_, err = f.Write(content)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		os.Remove(path)
		return Document{}, ErrStorage
	}

	doc := Document{
		ID:         documentID,
		Kind:       kind,
		MIME:       mime,
		Size:       len(content),
		UploadedAt: s.now().UTC(),
	}

	oldID := ""
	err = s.mutate(func(db *database) error {
		app := db.Applications[appID]
		var remaining []Document
		for _, d := range app.Documents {
			if d.Kind == kind {
				oldID = d.ID
			} else {
				remaining = append(remaining, d)
			}
		}
		app.Documents = append(remaining, doc)
		if kind == DocPassportPhoto || kind == DocGovernmentID {
			app.Checks.Identity = false
		} else if kind == DocProofOfAddress {
			app.Checks.Address = false
		} else if kind == DocCACCertificate {
			app.Checks.CAC = false
		}
		app.UpdatedAt = s.now().UTC()
		db.Applications[appID] = app
		db.Events = append(db.Events, Event{
			ApplicationID: appID,
			UserID:        userID,
			Actor:         "user:" + userID,
			Action:        "document_uploaded_" + kind,
			At:            s.now().UTC(),
		})
		return nil
	})
	if err != nil {
		os.Remove(path)
		return Document{}, err
	}
	if oldID != "" {
		_ = os.Remove(filepath.Join(dir, oldID))
	}
	return doc, nil
}

// ReadDocument serves stored image content after authorization.
func (s *Service) ReadDocument(id, documentID string) (Document, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.db.Applications[id]
	if !ok {
		return Document{}, nil, ErrNotFound
	}
	for _, d := range a.Documents {
		if d.ID == documentID {
			content, err := os.ReadFile(filepath.Join(filepath.Dir(s.path), "documents", d.ID))
			if err != nil {
				return Document{}, nil, ErrStorage
			}
			return d, content, nil
		}
	}
	return Document{}, nil, ErrNotFound
}
