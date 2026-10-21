package maildelivery

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// SMTPConfig supports mandatory STARTTLS or implicit TLS, never cleartext auth.
type SMTPConfig struct {
	Host                              string
	Port                              int
	Username, Password, From, TLSMode string
	Timeout                           time.Duration
}

// Validate rejects incomplete credentials and unsafe header/address configuration.
func (c SMTPConfig) Validate() error {
	if c.Host == "" || strings.ContainsAny(c.Host, "\r\n /:") || c.Port < 1 || c.Port > 65535 || c.Timeout <= 0 || !bareAddress(c.From) {
		return fmt.Errorf("invalid SMTP host, port, sender or timeout")
	}
	if c.TLSMode != "starttls" && c.TLSMode != "tls" {
		return fmt.Errorf("SMTP TLS mode must be starttls or tls")
	}
	if (c.Username == "") != (c.Password == "") {
		return fmt.Errorf("SMTP username and password must be configured together")
	}
	return nil
}

func bareAddress(value string) bool {
	if strings.ContainsAny(value, "\r\n") || strings.LastIndex(value, "@") < 1 {
		return false
	}
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value
}

// SMTP implements Sender over a verified encrypted connection.
type SMTP struct {
	config    SMTPConfig
	tlsConfig *tls.Config
}

// NewSMTP validates provider settings. TLS certificate verification cannot be disabled.
func NewSMTP(c SMTPConfig) (*SMTP, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &SMTP{config: c, tlsConfig: &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12}}, nil
}

// Send honors a deadline throughout greeting, TLS, authentication and DATA.
func (s *SMTP) Send(ctx context.Context, m Message) error {
	if !bareAddress(m.To) || strings.ContainsAny(m.Subject+m.ID, "\r\n") {
		return fmt.Errorf("invalid mail headers")
	}
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	addr := net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port))
	dialer := &net.Dialer{Timeout: s.config.Timeout}
	var connection net.Conn
	var err error
	if s.config.TLSMode == "tls" {
		connection, err = (&tls.Dialer{NetDialer: dialer, Config: s.tlsConfig}).DialContext(ctx, "tcp", addr)
	} else {
		connection, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("connect SMTP: %w", err)
	}
	defer func() { _ = connection.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		if err = connection.SetDeadline(deadline); err != nil {
			return fmt.Errorf("SMTP deadline: %w", err)
		}
	}
	client, err := smtp.NewClient(connection, s.config.Host)
	if err != nil {
		return fmt.Errorf("SMTP greeting: %w", err)
	}
	defer func() { _ = client.Close() }()
	if s.config.TLSMode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("SMTP server requires STARTTLS support")
		}
		if err = client.StartTLS(s.tlsConfig); err != nil {
			return fmt.Errorf("SMTP TLS: %w", err)
		}
	}
	if s.config.Username != "" {
		if err = client.Auth(smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)); err != nil {
			return fmt.Errorf("SMTP authentication: %w", err)
		}
	}
	if err = client.Mail(s.config.From); err != nil {
		return fmt.Errorf("SMTP sender: %w", err)
	}
	if err = client.Rcpt(m.To); err != nil {
		return fmt.Errorf("SMTP recipient: %w", err)
	}
	body, err := encodeMessage(s.config.From, m)
	if err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA: %w", err)
	}
	if _, err = writer.Write(body); err != nil {
		_ = writer.Close()
		return fmt.Errorf("SMTP body: %w", err)
	}
	if err = writer.Close(); err != nil {
		return fmt.Errorf("SMTP acceptance: %w", err)
	}
	// DATA success is delivery acceptance; a failed QUIT must not force a retry.
	_ = client.Quit()
	return nil
}

func encodeMessage(from string, m Message) ([]byte, error) {
	var buffer bytes.Buffer
	domain := from[strings.LastIndex(from, "@")+1:]
	fmt.Fprintf(&buffer, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%s@%s>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", from, m.To, mime.QEncoding.Encode("UTF-8", m.Subject), time.Now().UTC().Format(time.RFC1123Z), m.ID, domain)
	writer := quotedprintable.NewWriter(&buffer)
	if _, err := writer.Write([]byte(m.Text)); err != nil {
		return nil, fmt.Errorf("encode email: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("finish email: %w", err)
	}
	return buffer.Bytes(), nil
}
