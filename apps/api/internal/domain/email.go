package domain

import "time"

// EmailTemplate represents an editable email template stored in the database.
type EmailTemplate struct {
	Key       string    `json:"key"`
	Subject   string    `json:"subject"`
	BodyHTML  string    `json:"body_html"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EmailMessage represents a fully rendered email ready for sending.
type EmailMessage struct {
	To       string
	Subject  string
	BodyHTML string
}

// SMTPConfig holds the resolved SMTP connection parameters.
type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"-"`
	From     string `json:"from"`
	UseTLS   bool   `json:"use_tls"`
}

// Configured returns true if enough SMTP configuration exists to attempt delivery.
func (c SMTPConfig) Configured() bool {
	return c.Host != ""
}
