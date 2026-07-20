-- +goose Up
CREATE TABLE contacts (
  company    TEXT      NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL,
  id         INTEGER   PRIMARY KEY AUTOINCREMENT,
  job_title  TEXT      NOT NULL DEFAULT '',
  name       TEXT      NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
CREATE INDEX idx_contacts_name ON contacts(name);

CREATE TABLE emails (
  contact_id INTEGER   NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
  created_at TIMESTAMP NOT NULL,
  email      TEXT      NOT NULL,
  id         INTEGER   PRIMARY KEY AUTOINCREMENT,
  is_primary INTEGER   NOT NULL DEFAULT 0,
  updated_at TIMESTAMP NOT NULL
);
CREATE UNIQUE INDEX idx_emails_contact_id_email ON emails(contact_id, email);
CREATE INDEX idx_emails_contact_id ON emails(contact_id);

-- +goose Down
DROP INDEX IF EXISTS idx_emails_contact_id;
DROP INDEX IF EXISTS idx_emails_contact_id_email;
DROP TABLE IF EXISTS emails;
DROP INDEX IF EXISTS idx_contacts_name;
DROP TABLE IF EXISTS contacts;
