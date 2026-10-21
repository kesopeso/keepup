package maildelivery

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func smtpSink(t *testing.T, mode string, advertiseTLS, rejectData bool) (SMTPConfig, *x509.CertPool, <-chan string, *atomic.Int32) {
	t.Helper()
	certificateServer := httptest.NewTLSServer(http.NotFoundHandler())
	tlsConfig := certificateServer.TLS.Clone()
	roots := x509.NewCertPool()
	roots.AddCert(certificateServer.Certificate())
	certificateServer.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	messages := make(chan string, 1)
	done := make(chan struct{})
	authCalls := &atomic.Int32{}
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(6 * time.Second):
			t.Error("SMTP test server did not stop")
		}
	})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		if deadlineErr := conn.SetDeadline(time.Now().Add(5 * time.Second)); deadlineErr != nil {
			return
		}
		encrypted := mode == "tls"
		if encrypted {
			conn = tls.Server(conn, tlsConfig)
		}
		protocol := textproto.NewConn(conn)
		if err := protocol.PrintfLine("220 smtp.example ESMTP"); err != nil {
			return
		}
		for {
			line, readErr := protocol.ReadLine()
			if readErr != nil {
				return
			}
			command := strings.SplitN(line, " ", 2)[0]
			switch command {
			case "EHLO":
				if err := protocol.PrintfLine("250-smtp.example"); err != nil {
					return
				}
				if !encrypted && advertiseTLS {
					if err := protocol.PrintfLine("250-STARTTLS"); err != nil {
						return
					}
				}
				if err := protocol.PrintfLine("250 AUTH PLAIN"); err != nil {
					return
				}
			case "STARTTLS":
				if err := protocol.PrintfLine("220 Ready"); err != nil {
					return
				}
				conn = tls.Server(conn, tlsConfig)
				protocol = textproto.NewConn(conn)
				encrypted = true
			case "AUTH":
				authCalls.Add(1)
				expected := "AUTH PLAIN " + base64.StdEncoding.EncodeToString([]byte("\x00user\x00secret"))
				if !encrypted || line != expected {
					if err := protocol.PrintfLine("535 Authentication failed"); err != nil {
						return
					}
				} else {
					if err := protocol.PrintfLine("235 Authenticated"); err != nil {
						return
					}
				}
			case "MAIL", "RCPT":
				if err := protocol.PrintfLine("250 OK"); err != nil {
					return
				}
			case "DATA":
				if err := protocol.PrintfLine("354 Send data"); err != nil {
					return
				}
				data, err := io.ReadAll(protocol.DotReader())
				if err != nil {
					return
				}
				if rejectData {
					if err := protocol.PrintfLine("451 Try later"); err != nil {
						return
					}
				} else {
					messages <- string(data)
					if err := protocol.PrintfLine("250 Queued"); err != nil {
						return
					}
				}
			case "QUIT":
				_ = protocol.PrintfLine("221 Bye")
				return
			default:
				_ = protocol.PrintfLine("500 Unsupported")
				return
			}
		}
	}()
	host, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	return SMTPConfig{Host: host, Port: port, From: "keepup@example.com", Username: "user", Password: "secret", TLSMode: mode, Timeout: time.Second}, roots, messages, authCalls
}

func TestSMTPEncryptedDelivery(t *testing.T) {
	for _, mode := range []string{"starttls", "tls"} {
		t.Run(mode, func(t *testing.T) {
			cfg, roots, messages, authCalls := smtpSink(t, mode, true, false)
			provider, err := NewSMTP(cfg)
			if err != nil {
				t.Fatal(err)
			}
			provider.tlsConfig.RootCAs = roots
			err = provider.Send(context.Background(), Message{ID: "stable-id", To: "user@example.com", Subject: "Verify KeepUp", Text: "Open https://keepup.example/auth/verify#token=synthetic\n"})
			if err != nil {
				t.Fatal(err)
			}
			select {
			case body := <-messages:
				if !strings.Contains(body, "Message-ID: <stable-id@example.com>") || !strings.Contains(body, "Content-Transfer-Encoding: quoted-printable") {
					t.Fatal("invalid mail encoding")
				}
			default:
				t.Fatal("mail not accepted")
			}
			if authCalls.Load() != 1 {
				t.Fatal("authentication count", authCalls.Load())
			}
		})
	}
}

func TestSMTPRefusesUnsafeDelivery(t *testing.T) {
	t.Run("no STARTTLS", func(t *testing.T) {
		cfg, _, _, authCalls := smtpSink(t, "starttls", false, false)
		provider, err := NewSMTP(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err = provider.Send(context.Background(), Message{ID: "id", To: "user@example.com"}); err == nil {
			t.Fatal("plaintext delivery accepted")
		}
		if authCalls.Load() != 0 {
			t.Fatal("credentials sent before TLS")
		}
	})
	t.Run("untrusted certificate", func(t *testing.T) {
		cfg, _, _, authCalls := smtpSink(t, "tls", true, false)
		provider, err := NewSMTP(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err = provider.Send(context.Background(), Message{ID: "id", To: "user@example.com"}); err == nil {
			t.Fatal("untrusted TLS accepted")
		}
		if authCalls.Load() != 0 {
			t.Fatal("credentials reached untrusted server")
		}
	})
	t.Run("temporary failure", func(t *testing.T) {
		cfg, roots, _, _ := smtpSink(t, "tls", true, true)
		provider, err := NewSMTP(cfg)
		if err != nil {
			t.Fatal(err)
		}
		provider.tlsConfig.RootCAs = roots
		if err = provider.Send(context.Background(), Message{ID: "id", To: "user@example.com", Text: "body"}); err == nil {
			t.Fatal("SMTP rejection treated as acceptance")
		}
	})
	t.Run("cancellation", func(t *testing.T) {
		cfg, _, _, _ := smtpSink(t, "tls", true, false)
		provider, err := NewSMTP(cfg)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err = provider.Send(ctx, Message{ID: "id", To: "user@example.com"}); err == nil {
			t.Fatal("cancelled delivery continued")
		}
	})
}

func TestSMTPConfigurationAndHeaders(t *testing.T) {
	cfg := SMTPConfig{Host: "smtp.example", Port: 587, From: "keepup@example.com", TLSMode: "starttls", Timeout: time.Second}
	for _, change := range []func(*SMTPConfig){
		func(c *SMTPConfig) { c.From = "bad\r\nBcc: victim@example.com" },
		func(c *SMTPConfig) { c.TLSMode = "none" }, func(c *SMTPConfig) { c.Username = "only-username" },
		func(c *SMTPConfig) { c.Port = 0 },
	} {
		bad := cfg
		change(&bad)
		if _, err := NewSMTP(bad); err == nil {
			t.Fatal("unsafe SMTP configuration accepted")
		}
	}
	provider, err := NewSMTP(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = provider.Send(context.Background(), Message{To: "user@example.com", Subject: "bad\r\nBcc: victim@example.com"}); err == nil {
		t.Fatal("header injection accepted")
	}
}
