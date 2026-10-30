package smtp

import (
	"errors"
	"io"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

func TestSendMailWithTimeout(t *testing.T) {
	for _, stage := range []string{"greeting", "hello", "TLS handshake"} {
		t.Run(stage, func(t *testing.T) {
			listener := newLocalListener(t)
			defer listener.Close()
			done := make(chan struct{})
			go func() {
				defer close(done)
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(5 * time.Second))
				text := textproto.NewConn(conn)
				if stage != "greeting" {
					text.PrintfLine("220 mx.example.com")
					if _, err := text.ReadLine(); err != nil {
						return
					}
				}
				if stage == "TLS handshake" {
					text.PrintfLine("250-mx.example.com")
					text.PrintfLine("250 STARTTLS")
					if _, err := text.ReadLine(); err != nil {
						return
					}
					text.PrintfLine("220 start TLS")
				}
				// Keep the connection open without responding until the client
				// times out and disconnects.
				io.Copy(io.Discard, conn)
			}()
			started := time.Now()
			err := SendMailWithTimeout(listener.Addr().String(), nil, "sender@example.com", []string{"to@example.com"}, []byte("test"), "relay.example.com", 100*time.Millisecond)
			var networkError net.Error
			if !errors.As(err, &networkError) || !networkError.Timeout() {
				t.Fatalf("error = %v, want a network timeout", err)
			}
			if elapsed := time.Since(started); elapsed > 2*time.Second {
				t.Fatalf("timeout took %s", elapsed)
			}
			<-done
		})
	}
}

func TestSendMailAcceptedBeforeQuitFailure(t *testing.T) {
	listener := newLocalListener(t)
	defer listener.Close()
	serverResult := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverResult <- err
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))
		text := textproto.NewConn(conn)
		text.PrintfLine("220 mx.example.com")
		for {
			line, err := text.ReadLine()
			if err != nil {
				serverResult <- err
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO "), strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
				text.PrintfLine("250 OK")
			case line == "DATA":
				text.PrintfLine("354 send data")
				if _, err := text.ReadDotBytes(); err != nil {
					serverResult <- err
					return
				}
				text.PrintfLine("250 accepted")
			case line == "QUIT":
				// Drop the connection without a 221 response, after accepting DATA.
				serverResult <- nil
				return
			default:
				serverResult <- errors.New("unexpected SMTP command: " + line)
				return
			}
		}
	}()
	if err := SendMail(listener.Addr().String(), nil, "sender@example.com", []string{"to@example.com"}, []byte("Subject: test\r\n\r\nhello\r\n"), "relay.example.com"); err != nil {
		t.Fatalf("accepted message must not be retried: %v", err)
	}
	if err := <-serverResult; err != nil {
		t.Fatal(err)
	}
}

func TestSendMailWithTimeoutRejectsNonPositiveTimeout(t *testing.T) {
	for _, timeout := range []time.Duration{0, -time.Second} {
		if err := SendMailWithTimeout("unused:25", nil, "", nil, nil, "", timeout); err == nil {
			t.Fatalf("expected an error for timeout %s", timeout)
		}
	}
}
