package chat

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db"
)

const (
	AccessFull = "full"
	AccessSafe = "safe"

	attachmentMaxBytes = 32 << 20
)

func (s *Service) AddAttachment(threadID int64, name, mimeType string, r io.Reader) (db.Attachment, error) {
	thread, err := s.store.GetThread(threadID)
	if err != nil {
		return db.Attachment{}, err
	}
	dir, err := attachmentsDir()
	if err != nil {
		return db.Attachment{}, err
	}
	token := make([]byte, 6)
	if _, err := rand.Read(token); err != nil {
		return db.Attachment{}, errors.Wrap(err, "attachment token")
	}
	name = sanitizeFilename(name)
	path := filepath.Join(dir, hex.EncodeToString(token)+"-"+name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return db.Attachment{}, errors.Wrap(err, "create attachment file")
	}
	size, err := io.Copy(file, io.LimitReader(r, attachmentMaxBytes))
	file.Close()
	if err != nil {
		_ = os.Remove(path)
		return db.Attachment{}, errors.Wrap(err, "write attachment file")
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	attachment, err := s.store.AddAttachment(db.NewAttachment{
		FilePath: path,
		MimeType: mimeType,
		Name:     name,
		Size:     size,
		ThreadID: thread.ID,
	})
	if err != nil {
		_ = os.Remove(path)
		return db.Attachment{}, err
	}
	return attachment, nil
}

func (s *Service) Attachment(id int64) (db.Attachment, error) {
	return s.store.GetAttachment(id)
}

func (s *Service) DeleteDraftAttachment(id int64) error {
	attachment, err := s.store.DeleteDraftAttachment(id)
	if err != nil {
		return err
	}
	return errors.Wrap(os.Remove(attachment.FilePath), "remove attachment file")
}

func (s *Service) ThreadAccess(thread db.Thread) string {
	var anchor struct {
		Access string `json:"access"`
	}
	_ = json.Unmarshal([]byte(thread.Anchor), &anchor)
	if anchor.Access == AccessSafe {
		return AccessSafe
	}
	return AccessFull
}

func (s *Service) ToggleThreadAccess(threadID int64) (string, error) {
	thread, err := s.store.GetThread(threadID)
	if err != nil {
		return "", err
	}
	anchor := map[string]any{}
	if err := json.Unmarshal([]byte(thread.Anchor), &anchor); err != nil {
		anchor = map[string]any{}
	}
	access := AccessSafe
	if s.ThreadAccess(thread) == AccessSafe {
		access = AccessFull
	}
	anchor["access"] = access
	data, err := json.Marshal(anchor)
	if err != nil {
		return "", errors.Wrap(err, "marshal anchor")
	}
	if err := s.store.SetThreadAnchor(threadID, string(data)); err != nil {
		return "", err
	}
	return access, nil
}

func attachmentsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "user home dir")
	}
	dir := filepath.Join(home, ".config", "scratch", "attachments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", errors.Wrap(err, "create attachments dir")
	}
	return dir, nil
}

func sanitizeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "attachment"
	}
	return name
}
