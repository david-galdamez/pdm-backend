package email

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTP is just enough of an SMTP server to accept one message and report
// what it was told, so SMTPSender can be judged on the conversation it has
// rather than on a real mailbox.
type fakeSMTP struct {
	listener net.Listener

	mu       sync.Mutex
	from     string
	to       []string
	data     string
	rejectAt string // command verb to answer with a 5xx, e.g. "RCPT"
}

func startFakeSMTP(t *testing.T, rejectAt string) *fakeSMTP {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}

	server := &fakeSMTP{listener: listener, rejectAt: rejectAt}

	go server.serve()

	t.Cleanup(func() { listener.Close() })

	return server
}

func (s *fakeSMTP) addr() string { return s.listener.Addr().String() }

func (s *fakeSMTP) host() string {
	host, _, _ := net.SplitHostPort(s.addr())
	return host
}

func (s *fakeSMTP) port() int {
	_, port, _ := net.SplitHostPort(s.addr())

	var p int
	fmt.Sscanf(port, "%d", &p)

	return p
}

func (s *fakeSMTP) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		go s.handle(conn)
	}
}

func (s *fakeSMTP) handle(conn net.Conn) {
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))

	reader := bufio.NewReader(conn)
	write := func(line string) { fmt.Fprintf(conn, "%s\r\n", line) }

	write("220 fake.test ESMTP")

	inData := false
	var body strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		line = strings.TrimRight(line, "\r\n")

		if inData {
			if line == "." {
				inData = false

				s.mu.Lock()
				s.data = body.String()
				s.mu.Unlock()

				write("250 2.0.0 Ok: queued")
				continue
			}

			body.WriteString(line)
			body.WriteString("\r\n")
			continue
		}

		verb := strings.ToUpper(strings.SplitN(line, " ", 2)[0])

		if s.rejectAt != "" && verb == strings.ToUpper(s.rejectAt) {
			write("550 5.1.1 rejected by the test")
			continue
		}

		switch verb {
		case "EHLO":
			write("250-fake.test")
			write("250 AUTH PLAIN LOGIN")
		case "HELO":
			write("250 fake.test")
		case "AUTH":
			write("235 2.7.0 accepted")
		case "MAIL":
			s.mu.Lock()
			s.from = line
			s.mu.Unlock()
			write("250 2.1.0 Ok")
		case "RCPT":
			s.mu.Lock()
			s.to = append(s.to, line)
			s.mu.Unlock()
			write("250 2.1.5 Ok")
		case "DATA":
			inData = true
			write("354 End data with <CR><LF>.<CR><LF>")
		case "QUIT":
			write("221 2.0.0 Bye")
			return
		case "RSET", "NOOP":
			write("250 2.0.0 Ok")
		default:
			write("502 5.5.2 not implemented")
		}
	}
}

func (s *fakeSMTP) received() (string, []string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.from, append([]string(nil), s.to...), s.data
}

func TestSMTPSenderDeliversTheMessage(t *testing.T) {
	server := startFakeSMTP(t, "")

	var sender Sender = NewSMTPSender(server.host(), server.port(), "", "", "noreply@example.test")

	message := Message{
		To:       []string{"ana@example.test", "david@example.test"},
		Subject:  "A transaction was recorded",
		HTMLBody: "<p>Groceries</p>",
	}

	if err := sender.Send(context.Background(), message); err != nil {
		t.Fatalf("sending: %v", err)
	}

	from, to, data := server.received()

	if !strings.Contains(from, "noreply@example.test") {
		t.Errorf("MAIL FROM = %q, want it to carry noreply@example.test", from)
	}

	if len(to) != 2 {
		t.Fatalf("server saw %d RCPT commands, want 2: %v", len(to), to)
	}

	for _, want := range []string{"Subject: A transaction was recorded", "<p>Groceries</p>"} {
		if !strings.Contains(data, want) {
			t.Errorf("message body is missing %q:\n%s", want, data)
		}
	}

	// Without this the client renders the markup as plain text.
	if !strings.Contains(strings.ToLower(data), "text/html") {
		t.Errorf("message is not declared as HTML:\n%s", data)
	}

	// The To header has to match the envelope, otherwise clients show the
	// message as addressed to nobody. Keeping the membership list out of each
	// other's inboxes is the worker's job: it sends one message per recipient.
	if !strings.Contains(data, "To: ana@example.test, david@example.test") {
		t.Errorf("To header does not list the recipients:\n%s", data)
	}
}

// A rejection is what puts the message on the dead-letter queue, so it has to
// come back as an error rather than a silent success.
func TestSMTPSenderReportsARejection(t *testing.T) {
	server := startFakeSMTP(t, "RCPT")

	sender := NewSMTPSender(server.host(), server.port(), "", "", "noreply@example.test")

	err := sender.Send(context.Background(), Message{
		To:       []string{"ana@example.test"},
		Subject:  "A transaction was recorded",
		HTMLBody: "<p>Groceries</p>",
	})
	if err == nil {
		t.Fatal("Send reported success after the server rejected the recipient")
	}
}

// The worker resolves recipients from the database and may legitimately find
// none (a finance whose only other member left). That is not an SMTP problem.
func TestSMTPSenderSkipsAMessageWithNoRecipients(t *testing.T) {
	server := startFakeSMTP(t, "")

	sender := NewSMTPSender(server.host(), server.port(), "", "", "noreply@example.test")

	if err := sender.Send(context.Background(), Message{Subject: "x", HTMLBody: "<p>x</p>"}); err != nil {
		t.Fatalf("Send with no recipients returned %v, want nil", err)
	}

	if _, to, _ := server.received(); len(to) != 0 {
		t.Errorf("server saw %d RCPT commands, want 0", len(to))
	}
}

// A dead SMTP host must fail the delivery within the deadline instead of
// parking the worker's prefetch slot indefinitely.
func TestSMTPSenderHonoursContextDeadline(t *testing.T) {
	// Port 1 is reserved and refuses or blackholes connections.
	sender := NewSMTPSender("127.0.0.1", 1, "", "", "noreply@example.test")

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- sender.Send(ctx, Message{
			To:       []string{"ana@example.test"},
			Subject:  "x",
			HTMLBody: "<p>x</p>",
		})
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Send reported success against a dead host")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Send ignored the context deadline and hung")
	}
}
