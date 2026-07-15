package db

import (
	"context"
	"database/sql"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db/internal/sqlite"
	"github.com/housecat-inc/scratch/pkg/ts"
)

type Contact struct {
	Company   string
	CreatedAt ts.Timestamp
	ID        int64
	JobTitle  string
	Linkedin  string
	Location  string
	Name      string
	Phone     string
	Status    string
	Twitter   string
	UpdatedAt ts.Timestamp
}

type ContactListItem struct {
	Contact
	PrimaryEmail string
}

type ContactNote struct {
	Body      string
	ContactID int64
	CreatedAt ts.Timestamp
	ID        int64
}

type Email struct {
	ContactID int64
	CreatedAt ts.Timestamp
	Email     string
	ID        int64
	IsPrimary bool
	UpdatedAt ts.Timestamp
}

type ContactStore interface {
	AddContact(name, company, jobTitle string) (Contact, error)
	AddContactNote(contactID int64, body string) (ContactNote, error)
	AddEmail(contactID int64, email string, isPrimary bool) (Email, error)
	CountContactNotes(contactID int64) (int, error)
	CountContacts() (int, error)
	DeleteContact(id int64) error
	DeleteEmail(id int64) error
	GetContact(id int64) (Contact, error)
	ListContactEmails(contactID int64) ([]Email, error)
	ListContactNotes(contactID int64) ([]ContactNote, error)
	ListContactsPage(limit, offset int) ([]ContactListItem, error)
	RecordContactNote(contactID int64, body string) error
	SearchContacts(query string, limit int) ([]ContactListItem, error)
	UpdateContact(id int64, name, company, jobTitle string) (Contact, error)
}

var ErrContactNotFound = errors.New("contact not found")

func IsContactNotFound(err error) bool { return errors.Is(err, ErrContactNotFound) }

func (d *DB) AddContact(name, company, jobTitle string) (Contact, error) {
	now := ts.Now()
	row, err := d.queries.AddContact(context.Background(), sqlite.AddContactParams{
		Company:   company,
		CreatedAt: now,
		JobTitle:  jobTitle,
		Name:      name,
		UpdatedAt: now,
	})
	if err != nil {
		return Contact{}, errors.Wrap(err, "insert contact")
	}
	return fromSqliteContact(row), nil
}

func (d *DB) AddEmail(contactID int64, email string, isPrimary bool) (Email, error) {
	now := ts.Now()
	row, err := d.queries.AddEmail(context.Background(), sqlite.AddEmailParams{
		ContactID: contactID,
		CreatedAt: now,
		Email:     email,
		IsPrimary: boolToInt(isPrimary),
		UpdatedAt: now,
	})
	if err != nil {
		return Email{}, errors.Wrap(err, "insert email")
	}
	return fromSqliteEmail(row), nil
}

func (d *DB) CountContacts() (int, error) {
	n, err := d.queries.CountContacts(context.Background())
	if err != nil {
		return 0, errors.Wrap(err, "count contacts")
	}
	return int(n), nil
}

func (d *DB) DeleteContact(id int64) error {
	_, err := d.queries.DeleteContact(context.Background(), id)
	return errors.Wrap(err, "delete contact")
}

func (d *DB) DeleteEmail(id int64) error {
	_, err := d.queries.DeleteEmail(context.Background(), id)
	return errors.Wrap(err, "delete email")
}

func (d *DB) GetContact(id int64) (Contact, error) {
	row, err := d.queries.GetContact(context.Background(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return Contact{}, ErrContactNotFound
	}
	if err != nil {
		return Contact{}, errors.Wrap(err, "get contact")
	}
	return fromSqliteContact(row), nil
}

func (d *DB) ListContactEmails(contactID int64) ([]Email, error) {
	rows, err := d.queries.ListContactEmails(context.Background(), contactID)
	if err != nil {
		return nil, errors.Wrap(err, "list contact emails")
	}
	out := make([]Email, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromSqliteEmail(r))
	}
	return out, nil
}

func (d *DB) ListContactsPage(limit, offset int) ([]ContactListItem, error) {
	rows, err := d.queries.ListContactsPage(context.Background(), sqlite.ListContactsPageParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, errors.Wrap(err, "list contacts page")
	}
	out := make([]ContactListItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, ContactListItem{
			Contact: Contact{
				Company:   r.Company,
				CreatedAt: r.CreatedAt,
				ID:        r.ID,
				JobTitle:  r.JobTitle,
				Linkedin:  r.Linkedin,
				Location:  r.Location,
				Name:      r.Name,
				Phone:     r.Phone,
				Status:    r.Status,
				Twitter:   r.Twitter,
				UpdatedAt: r.UpdatedAt,
			},
			PrimaryEmail: asString(r.PrimaryEmail),
		})
	}
	return out, nil
}

func (d *DB) SearchContacts(query string, limit int) ([]ContactListItem, error) {
	pattern := "%" + query + "%"
	rows, err := d.queries.SearchContacts(context.Background(), sqlite.SearchContactsParams{
		Email: pattern,
		Limit: int64(limit),
		Name:  pattern,
	})
	if err != nil {
		return nil, errors.Wrap(err, "search contacts")
	}
	out := make([]ContactListItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, ContactListItem{
			Contact: Contact{
				Company:   r.Company,
				CreatedAt: r.CreatedAt,
				ID:        r.ID,
				JobTitle:  r.JobTitle,
				Linkedin:  r.Linkedin,
				Location:  r.Location,
				Name:      r.Name,
				Phone:     r.Phone,
				Status:    r.Status,
				Twitter:   r.Twitter,
				UpdatedAt: r.UpdatedAt,
			},
			PrimaryEmail: asString(r.PrimaryEmail),
		})
	}
	return out, nil
}

func (d *DB) UpdateContact(id int64, name, company, jobTitle string) (Contact, error) {
	_, err := d.queries.UpdateContact(context.Background(), sqlite.UpdateContactParams{
		Company:   company,
		ID:        id,
		JobTitle:  jobTitle,
		Name:      name,
		UpdatedAt: ts.Now(),
	})
	if err != nil {
		return Contact{}, errors.Wrap(err, "update contact")
	}
	return d.GetContact(id)
}

func (d *DB) AddContactNote(contactID int64, body string) (ContactNote, error) {
	row, err := d.queries.AddContactNote(context.Background(), sqlite.AddContactNoteParams{
		Body:      body,
		ContactID: contactID,
		CreatedAt: ts.Now(),
	})
	if err != nil {
		return ContactNote{}, errors.Wrap(err, "insert contact note")
	}
	return ContactNote{Body: row.Body, ContactID: row.ContactID, CreatedAt: row.CreatedAt, ID: row.ID}, nil
}

func (d *DB) RecordContactNote(contactID int64, body string) error {
	_, err := d.AddContactNote(contactID, body)
	return err
}

func (d *DB) CountContactNotes(contactID int64) (int, error) {
	n, err := d.queries.CountContactNotes(context.Background(), contactID)
	if err != nil {
		return 0, errors.Wrap(err, "count contact notes")
	}
	return int(n), nil
}

func (d *DB) ListContactNotes(contactID int64) ([]ContactNote, error) {
	rows, err := d.queries.ListContactNotes(context.Background(), contactID)
	if err != nil {
		return nil, errors.Wrap(err, "list contact notes")
	}
	out := make([]ContactNote, 0, len(rows))
	for _, r := range rows {
		out = append(out, ContactNote{Body: r.Body, ContactID: r.ContactID, CreatedAt: r.CreatedAt, ID: r.ID})
	}
	return out, nil
}

func asString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return ""
	}
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func fromSqliteContact(c sqlite.Contact) Contact {
	return Contact{
		Company:   c.Company,
		CreatedAt: c.CreatedAt,
		ID:        c.ID,
		JobTitle:  c.JobTitle,
		Linkedin:  c.Linkedin,
		Location:  c.Location,
		Name:      c.Name,
		Phone:     c.Phone,
		Status:    c.Status,
		Twitter:   c.Twitter,
		UpdatedAt: c.UpdatedAt,
	}
}

func fromSqliteEmail(e sqlite.Email) Email {
	return Email{
		ContactID: e.ContactID,
		CreatedAt: e.CreatedAt,
		Email:     e.Email,
		ID:        e.ID,
		IsPrimary: e.IsPrimary != 0,
		UpdatedAt: e.UpdatedAt,
	}
}
