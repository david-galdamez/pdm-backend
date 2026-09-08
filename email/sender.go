package email

import "context"

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

	if message.To == nil {
		return nil
	}

	return nil
}
