package service

import (
	"bufio"
	"fmt"
	"net"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func TestSMTPVerificationDelivery(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	type delivery struct {
		from, to, message string
		err               error
	}
	result := make(chan delivery, 1)
	go func() {
		var d delivery
		defer func() { result <- d }()
		conn, err := listener.Accept()
		if err != nil {
			d.err = err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		reader := bufio.NewReader(conn)
		fmt.Fprint(conn, "220 localhost test SMTP\r\n")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				d.err = err
				return
			}
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "EHLO "):
				fmt.Fprint(conn, "250-localhost\r\n250 8BITMIME\r\n")
			case strings.HasPrefix(line, "MAIL FROM:"):
				d.from = line
				fmt.Fprint(conn, "250 OK\r\n")
			case strings.HasPrefix(line, "RCPT TO:"):
				d.to = line
				fmt.Fprint(conn, "250 OK\r\n")
			case line == "DATA":
				fmt.Fprint(conn, "354 Enter mail\r\n")
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						d.err = err
						return
					}
					if line == ".\r\n" {
						break
					}
					d.message += line
				}
				fmt.Fprint(conn, "250 queued\r\n")
			case line == "QUIT":
				fmt.Fprint(conn, "221 Bye\r\n")
				return
			default:
				d.err = fmt.Errorf("unexpected SMTP command")
				return
			}
		}
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	t.Setenv("APP_ENV", "production")
	t.Setenv("SMTP_HOST", host)
	t.Setenv("SMTP_PORT", port)
	t.Setenv("SMTP_USERNAME", "")
	t.Setenv("SMTP_PASSWORD", "")
	t.Setenv("SMTP_FROM_EMAIL", "notifications@example.test")
	t.Setenv("SMTP_TLS", "starttls")
	t.Setenv("SMTP_EHLO_NAME", "localhost")
	t.Setenv("RESEND_API_KEY", "")
	if err := NewEmailService().SendVerificationCode("employee@example.test", "123456"); err != nil {
		t.Fatal(err)
	}
	d := <-result
	if d.err != nil {
		t.Fatal(d.err)
	}
	if !strings.Contains(d.from, "<notifications@example.test>") || !strings.Contains(d.to, "<employee@example.test>") {
		t.Fatal("unexpected SMTP envelope")
	}
	message, err := mail.ReadMessage(strings.NewReader(d.message))
	if err != nil {
		t.Fatal(err)
	}
	if message.Header.Get("From") != "notifications@example.test" || !strings.Contains(d.message, "123456") || !strings.Contains(d.message, "10 minutes") {
		t.Fatal("verification message missing expected content")
	}
}

func TestSMTPConfigFromModelHub(t *testing.T) {
	t.Setenv("SMTP_HOST", "smtp.example.test")
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_USERNAME", "account@example.test")
	t.Setenv("SMTP_PASSWORD", "test-token")
	t.Setenv("SMTP_FROM_EMAIL", "")
	t.Setenv("SMTP_TLS", "")
	t.Setenv("RESEND_API_KEY", "")
	t.Setenv("RESEND_FROM_EMAIL", "fallback@example.test")
	s := NewEmailService()
	if !s.smtpTLSImplicit || s.fromEmail != "account@example.test" || s.smtpPassword != "test-token" {
		t.Fatal("SMTP account fallback or port 465 TLS config incorrect")
	}
	t.Setenv("SMTP_FROM_EMAIL", "sender@example.test")
	if NewEmailService().fromEmail != "sender@example.test" {
		t.Fatal("explicit SMTP sender should take precedence")
	}
}

func TestProductionRequiresEmailDelivery(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SMTP_HOST", "")
	t.Setenv("RESEND_API_KEY", "")
	if err := NewEmailService().SendVerificationCode("employee@example.test", "123456"); err == nil {
		t.Fatal("production must not silently log verification codes")
	}
}
