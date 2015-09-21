package main

import (
	"fmt"
	"smtprelay/smtpd"
	"strings"
	"time"
)

const MAX_ERROR_BUFFER_SIZE = 1000000
const MAX_MAIL_BUFFER_SIZE = 1000000

var (
	MailChannel  chan QueueEntry
	ErrorChannel chan QueueEntry
)

type QueueEntry struct {
	MailServer      string
	Sender          string
	Recipients      []string
	SenderDomain    string
	RecipientDomain string
	MessageId       string
	Data            []byte
	Error           smtpd.Error
	ErrorCount      int
	QueueTime       time.Time
	UnqueueTime     time.Time
}

func (e QueueEntry) String() string {
	return fmt.Sprintf("(message-id:%s;from:%s;to:%s)", e.MessageId, e.Sender, strings.Join(e.Recipients, ";"))
}

func InitQueues() error {
	MailChannel = make(chan QueueEntry, MAX_MAIL_BUFFER_SIZE)
	ErrorChannel = make(chan QueueEntry, MAX_ERROR_BUFFER_SIZE)

	return nil
}

func PushMail(entry QueueEntry) {
	MailQueueCheckMax()
	MailChannel <- entry
	return
}

func PopMail() (entry QueueEntry) {
	entry = <-MailChannel
	return
}

func PushError(entry QueueEntry) {
	MailQueueCheckMax()
	ErrorChannel <- entry
	return
}

func ExtractError() (entry QueueEntry) {

	entry = <-ErrorChannel
	for entry.UnqueueTime.After(time.Now()) {
		time.Sleep(1 * time.Second)
	}
	return entry
}

func FlushErrors() {
	for len(ErrorChannel) > 0 {
		entry := <-ErrorChannel
		log.Error("msg %s FLUSHED from error queue. Error counter is forced to %d", entry.String(), entry.ErrorCount)
		entry.ErrorCount = conf.DeferredMailMaxErrors
		MailChannel <- entry
	}
}
