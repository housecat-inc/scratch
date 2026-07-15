package flow

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/housecat-inc/scratch/pkg/elicit"
)

const contactNoteTimeout = 24 * time.Hour

type ContactNoteInput struct{}

type ContactNote struct {
	Contact string `json:"contact" jsonschema:"Which contact is this about?"`
	Notes   string `json:"notes" jsonschema:"What would you like to note?"`
}

func (e *Engine) contactNote(ctx dbos.DBOSContext, _ ContactNoteInput) (string, error) {
	note, action, err := elicit.Form(ctx, "contact-note", "Log a note about a contact", ContactNote{}, contactNoteTimeout)
	if err != nil {
		return "", errors.Wrap(err, "elicit contact note")
	}
	if action != elicit.ActionAccept {
		return "", nil
	}
	summary, err := act(ctx, "respond/contact-note", "Record note", note.Contact, func(context.Context) (string, error) {
		return "Logged a note for contact #" + note.Contact + ":\n\n" + note.Notes, nil
	})
	if err != nil {
		return "", errors.Wrap(err, "record contact note")
	}
	return summary, nil
}
