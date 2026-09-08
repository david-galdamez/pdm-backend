package email

import (
	"context"
	"crypto/tls"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type Sender interface {
	Send(ctx context.Context, message Message) error
}

type Message struct {
	To       []string `json:"to"`
	Subject  string   `json:"subject"`
	HTMLBody string   `json:"html_body"`
}

type SmtpSender struct {
	host     string
	port     int
	username string
	password string
	from     string
}

func NewSMTPSender(host string, port int, username, password, from string) SmtpSender {
	return SmtpSender{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

func (s SmtpSender) Send(ctx context.Context, message Message) error {

	if len(message.To) == 0 {
		return nil
	}

	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(s.host, strconv.Itoa(s.port)))
	if err != nil {
		return err
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(30 * time.Second)
	}

	err = conn.SetDeadline(deadline)
	if err != nil {
		return err
	}

	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
			return
		}
	}()

	smtpClient, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return err
	}
	defer smtpClient.Close()

	smtpClient.StartTLS(&tls.Config{ServerName: s.host})
	smtpClient.Extension("STARTTLS")

	if s.username != "" {
		auth := smtp.PlainAuth("", s.username, s.password, s.host)
		if err := smtpClient.Auth(auth); err != nil {
			return err
		}
	}

	if err := smtpClient.Mail(s.from); err != nil {
		return err
	}

	for _, to := range message.To {
		err := smtpClient.Rcpt(to)
		if err != nil {
			return err
		}
	}

	writer, err := smtpClient.Data()
	if err != nil {
		return err
	}

	smtpMessage := "From: " + s.from + "\r\n" +
		"To: " + strings.Join(message.To, ", ") + "\r\n" +
		"Subject: " + message.Subject + "\r\n" +
		"Content-Type: text/html\r\n" +
		"MIME-Version: 1.0\r\n\r\n" +
		message.HTMLBody + "\r\n"

	_, err = writer.Write([]byte(smtpMessage))
	if err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}
	if err := smtpClient.Quit(); err != nil {
		return err
	}

	return nil
}
