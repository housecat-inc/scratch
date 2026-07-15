package flow

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/housecat-inc/scratch/pkg/elicit"
)

const contactNoteTimeout = 24 * time.Hour

type ContactNoteRecorder interface {
	RecordContactNote(contactID int64, body string) error
}

type noopContactNotes struct{}

func (noopContactNotes) RecordContactNote(int64, string) error { return nil }

type ContactNoteInput struct{}

type ContactNote struct {
	Contact string `json:"contact" jsonschema:"Which contact is this about?"`
	Notes   string `json:"notes" jsonschema:"What would you like to note?"`
}

func (e *Engine) contactNote(ctx dbos.DBOSContext, _ ContactNoteInput) (string, error) {
	note, action, err := elicit.Form(ctx, "contact-note", "Log a note about a contact", ContactNote{}, contactNoteTimeout,
		elicit.WithAccept("Save note"), elicit.WithDecline("Cancel"))
	if err != nil {
		return "", errors.Wrap(err, "elicit contact note")
	}
	if action != elicit.ActionAccept {
		return "", nil
	}
	contactID, _ := strconv.ParseInt(strings.TrimSpace(note.Contact), 10, 64)
	body := strings.TrimSpace(note.Notes)
	summary, err := act(ctx, "respond/contact-note", "Record note", note.Contact, func(context.Context) (string, error) {
		if contactID <= 0 {
			return "No contact selected; nothing recorded.", nil
		}
		if err := e.contactNotes.RecordContactNote(contactID, body); err != nil {
			return "", err
		}
		return "Saved a note for contact #" + strconv.FormatInt(contactID, 10) + ".", nil
	})
	if err != nil {
		return "", errors.Wrap(err, "record contact note")
	}
	return summary, nil
}
