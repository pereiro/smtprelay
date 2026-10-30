package main

import (
	"net"
	"reflect"
	"smtprelay/smtpd"
	"strings"
	"testing"
	"time"
)

// Each server records connections and can reject either the connection (421)
// or the recipient (other error codes), or accept the complete message (250).
func deliveryTestServer(t *testing.T, name string, code int, attempts chan string, delivered chan smtpd.Envelope) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &smtpd.Server{
		Hostname: "mx.example.com",
		ConnectionChecker: func(smtpd.Peer) error {
			attempts <- name
			if code == 421 {
				return smtpd.Error{Code: code, Message: "server unavailable"}
			}
			return nil
		},
		RecipientChecker: func(smtpd.Peer, string) error {
			if code != 250 {
				return smtpd.Error{Code: code, Message: "recipient rejected"}
			}
			return nil
		},
		Handler: func(_ smtpd.Peer, envelope smtpd.Envelope) error {
			delivered <- envelope
			return nil
		},
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		server.Serve(listener)
	}()
	t.Cleanup(func() {
		listener.Close()
		<-done
	})
	return listener.Addr().String()
}

func testQueueEntry(servers []string) QueueEntry {
	return QueueEntry{
		MailServers: servers,
		Sender:      "sender@example.com", Recipients: []string{"recipient@example.com"},
		RecipientDomain: "localhost", // DNS refresh fails immediately; keep the supplied MX list.
		Data:            []byte("From: sender@example.com\r\nTo: recipient@example.com\r\nSubject: MX test\r\n\r\nhello\r\n"),
	}
}

func checkDeliveryAttempts(t *testing.T, attempts chan string, want []string) {
	t.Helper()
	got := make([]string, 0, len(attempts))
	for len(attempts) > 0 {
		got = append(got, <-attempts)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("attempts = %v, want %v", got, want)
	}
}

func TestSendMailToServers(t *testing.T) {
	for _, tt := range []struct {
		name     string
		codes    []int
		attempts []string
		wantCode int
	}{
		{"first succeeds", []int{250, 250, 250}, []string{"first"}, 0},
		{"third succeeds", []int{421, 451, 250}, []string{"first", "second", "third"}, 0},
		{"all unavailable", []int{421, 451, 421}, []string{"first", "second", "third"}, 421},
		{"permanent rejection", []int{550, 250, 250}, []string{"first"}, 550},
		{"no servers", nil, []string{}, 450},
	} {
		t.Run(tt.name, func(t *testing.T) {
			attempts := make(chan string, 10)
			delivered := make(chan smtpd.Envelope, 10)
			var servers []string
			names := []string{"first", "second", "third"}
			for i, code := range tt.codes {
				servers = append(servers, deliveryTestServer(t, names[i], code, attempts, delivered))
			}
			entry := testQueueEntry(servers)
			err := sendMailToServers(entry, entry.Data, "relay.example.com")
			if tt.wantCode == 0 {
				if err != nil {
					t.Fatal(err)
				}
				if len(delivered) != 1 {
					t.Fatalf("delivered %d messages, want 1", len(delivered))
				}
				envelope := <-delivered
				if envelope.Sender != entry.Sender || !reflect.DeepEqual(envelope.Recipients, entry.Recipients) || string(envelope.Data) != strings.ReplaceAll(string(entry.Data), "\r\n", "\n") {
					t.Fatalf("message changed during failover: %+v", envelope)
				}
			} else if err == nil || ParseOutcomingError(err.Error()).Code != tt.wantCode {
				t.Fatalf("error = %v, want SMTP %d", err, tt.wantCode)
			} else if len(delivered) != 0 {
				t.Fatal("rejected message was delivered")
			}
			checkDeliveryAttempts(t, attempts, tt.attempts)
		})
	}
}

func TestSendMailToServersConnectionRefused(t *testing.T) {
	attempts := make(chan string, 10)
	delivered := make(chan smtpd.Envelope, 10)
	working := deliveryTestServer(t, "working", 250, attempts, delivered)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	refused := listener.Addr().String()
	listener.Close()
	entry := testQueueEntry([]string{refused, working})
	if err := sendMailToServers(entry, entry.Data, "relay.example.com"); err != nil {
		t.Fatal(err)
	}
	checkDeliveryAttempts(t, attempts, []string{"working"})
	if len(delivered) != 1 {
		t.Fatal("did not deliver to backup MX after connection refusal")
	}
}

func setupSenderTest(t *testing.T) {
	t.Helper()
	oldConf, oldLimiter := conf, SenderLimiter
	oldSenders, oldSent, oldDropped := MailSendersChannel, MailSentChannel, MailDroppedChannel
	oldMaxQueue, oldMail, oldErrors := MailMaxQueueChannel, MailChannel, ErrorChannel
	t.Cleanup(func() {
		conf, SenderLimiter = oldConf, oldLimiter
		MailSendersChannel, MailSentChannel, MailDroppedChannel = oldSenders, oldSent, oldDropped
		MailMaxQueueChannel, MailChannel, ErrorChannel = oldMaxQueue, oldMail, oldErrors
	})
	conf = &Conf{ServerHostName: "relay.example.com", DeferredMailDelay: 60, DeferredMailMaxErrors: 3}
	SenderLimiter = make(chan interface{}, 1)
	MailSendersChannel, MailSentChannel, MailDroppedChannel = make(chan int, 10), make(chan int, 10), make(chan int, 10)
	MailMaxQueueChannel = make(chan int, 10)
	MailChannel, ErrorChannel = make(chan QueueEntry, 10), make(chan QueueEntry, 10)
}

func TestSendMailDefersAfterAllMX(t *testing.T) {
	setupSenderTest(t)
	attempts := make(chan string, 10)
	delivered := make(chan smtpd.Envelope, 10)
	var servers []string
	for _, name := range []string{"first", "second", "third"} {
		servers = append(servers, deliveryTestServer(t, name, 421, attempts, delivered))
	}
	entry := testQueueEntry(servers)
	for round := 1; round <= 3; round++ {
		SenderLimiter <- 0
		SendMail(entry)
		checkDeliveryAttempts(t, attempts, []string{"first", "second", "third"})
		if round == 3 {
			if len(ErrorChannel) != 0 || len(MailDroppedChannel) != 1 {
				t.Fatal("message was not dropped at the deferred attempt limit")
			}
			continue
		}
		if len(ErrorChannel) != 1 || len(MailDroppedChannel) != 0 || len(MailSentChannel) != 0 {
			t.Fatal("message must be deferred once after exhausting all MX servers")
		}
		entry = <-ErrorChannel
		if entry.ErrorCount != round || !entry.UnqueueTime.After(time.Now()) || !reflect.DeepEqual(entry.MailServers, servers) {
			t.Fatalf("incorrect deferred entry: %+v", entry)
		}
	}
}

func TestSendMailRelayOnly(t *testing.T) {
	setupSenderTest(t)
	attempts := make(chan string, 10)
	delivered := make(chan smtpd.Envelope, 10)
	backup := deliveryTestServer(t, "domain MX", 250, attempts, delivered)
	conf.RelayModeEnabled = true
	conf.RelayServer = deliveryTestServer(t, "relay", 250, attempts, delivered)
	entry := testQueueEntry([]string{backup})
	SenderLimiter <- 0
	SendMail(entry)
	checkDeliveryAttempts(t, attempts, []string{"relay"})
	if len(MailSentChannel) != 1 || len(ErrorChannel) != 0 || len(MailDroppedChannel) != 0 {
		t.Fatal("relay delivery was not recorded as a success")
	}
}
