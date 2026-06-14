package mailer

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
)

// SMTPMailer sends mail over SMTP using STARTTLS (e.g. Gmail / Outlook on port 587).
type SMTPMailer struct {
	host     string
	port     string
	username string
	password string
	from     string
}

func NewSMTP(host, port, username, password, from string) *SMTPMailer {
	return &SMTPMailer{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

// Send delivers an HTML message. The context is accepted for interface
// consistency; net/smtp does not support cancellation.
func (m *SMTPMailer) Send(_ context.Context, to, subject, body string) error {
	addr := net.JoinHostPort(m.host, m.port)
	auth := smtp.PlainAuth("", m.username, m.password, m.host)
	msg := buildMessage(m.from, to, subject, body)
	if err := smtp.SendMail(addr, auth, m.from, []string{to}, msg); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return nil
}

func buildMessage(from, to, subject, body string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	return []byte(b.String())
}

// LogMailer prints the message to the server log instead of sending it. Used as
// a fallback when SMTP is not configured, so local development still works.
type LogMailer struct{}

func (LogMailer) Send(_ context.Context, to, subject, body string) error {
	log.Printf("[mailer:log] to=%s subject=%q\n%s", to, subject, body)
	return nil
}
