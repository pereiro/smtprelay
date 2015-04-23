package main

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"smtprelay/smtpd"
	"strings"
	"time"
)

const MAIL_BUCKET_NAME = "MAIL"
const ERROR_BUCKET_NAME = "ERROR"
const MAX_ERROR_BUFFER_SIZE = 1000000
const MAX_MAIL_BUFFER_SIZE = 1000
const MAX_TRANSACTION_LENGTH = 1000

var (
	errorDb           *bolt.DB
	mailDb            *bolt.DB
	MailDirectChannel chan QueueEntry
	ErrorQueueChannel chan QueueEntry
)

type QueueEntry struct {
	MailServer   string
	Sender       string
	Recipients   []string
	SenderDomain string
	MessageId    string
	Data         []byte
	Error        smtpd.Error
	ErrorCount   int
	QueueTime    time.Time
	UnqueueTime  time.Time
}

func (e QueueEntry) String() string {
	return fmt.Sprintf("(message-id:%s;from:%s;to:%s)", e.MessageId, e.Sender, strings.Join(e.Recipients, ";"))
}

func InitQueues(errorFilename string, mailFilename string) error {
	MailDirectChannel = make(chan QueueEntry, MAX_MAIL_BUFFER_SIZE)
	ErrorQueueChannel = make(chan QueueEntry, MAX_ERROR_BUFFER_SIZE)
	var err error
	errorDb, err = bolt.Open(errorFilename, 0600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return err
	}

	mailDb, err = bolt.Open(mailFilename, 0600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return err
	}

	var errorCounter int64
	var mailCounter int64

	err = errorDb.Update(func(tx *bolt.Tx) error {
		eBucket, err := tx.CreateBucketIfNotExists([]byte(ERROR_BUCKET_NAME))
		if err != nil {
			return err
		}
		errorCounter = int64(eBucket.Stats().KeyN)
		return nil
	})

	err = mailDb.Update(func(tx *bolt.Tx) error {
		mBucket, err := tx.CreateBucketIfNotExists([]byte(MAIL_BUCKET_NAME))
		if err != nil {
			return err
		}
		mailCounter = int64(mBucket.Stats().KeyN)
		return nil
	})

	if err != nil {
		return err
	}
	SetCounterInitialValues(errorCounter, mailCounter)
	go QueueHandler(ErrorQueueChannel, ERROR_BUCKET_NAME, &ErrorQueueCounter, 2000)
	return nil
}

func CloseQueues() {
	errorDb.Close()
}

func QueueHandler(ch chan QueueEntry, queueName string, queueCounter *int64, bufferSize int) {
	for {
		var entries []QueueEntry
		var entry QueueEntry
		for i := 0; i < bufferSize; i++ {
			select {
			case entry = <-ch:
				entries = append(entries, entry)
			default:
				if len(entries) == 0 {
					entry = <-ch
				} else {
					break
				}
			}
		}

		err := errorDb.Update(func(tx *bolt.Tx) error {
			for _, entry := range entries {
				b := tx.Bucket([]byte(queueName))
				json, err := json.Marshal(entry)
				if err != nil {
					return err
				}
				err = b.Put([]byte(entry.MessageId), json)
				if err != nil {
					return err
				}
				QueueIncreaseCounter(queueCounter, 1)
			}
			return nil
		})
		if err != nil {
			log.Error("Error writing queue on disk: %s", err.Error())
		}

	}
}

func PutMail(entry QueueEntry) error {
	select {
	case MailDirectChannel <- entry:
	default:
		{
			json, err := json.Marshal(entry)
			if err != nil {
				return err
			}
			err = mailDb.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte(MAIL_BUCKET_NAME))
				err = b.Put([]byte(entry.MessageId), json)
				if err != nil {
					return err
				}
				MailQueueIncreaseCounter(1)
				return nil
			})
		}
	}
	return nil
}

func ExtractMail() (entry QueueEntry, success bool, err error) {
	success = false
	var data []byte
	var key []byte
	err = mailDb.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(MAIL_BUCKET_NAME))
		c := b.Cursor()
		key, data = c.First()
		if err != nil {
			return err
		}
		if key == nil {
			return nil
		}
		err := b.Delete(key)
		if err != nil {
			return err
		}
		success = true
		MailQueueDecreaseCounter(1)
		return nil
	})
	err = json.Unmarshal(data, &entry)
	return
}

func PutError(entry QueueEntry) error {
	ErrorQueueChannel <- entry
	return nil
}

func ExtractError(ch chan QueueEntry) error {
	err, count := Extract(ch, ERROR_BUCKET_NAME, true)
	if err != nil {
		return err
	}
	ErrorQueueDecreaseCounter(count)
	return nil
}

func Extract(ch chan QueueEntry, queueName string, checkDate bool) (error, int) {
	var err error
	var count int
	var outdatedList []QueueEntry

	now := time.Now()

	err = errorDb.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(queueName))
		cursor := bucket.Cursor()
		for key, data := cursor.First(); key != nil && count < MAX_TRANSACTION_LENGTH; key, data = cursor.Next() {
			var entry QueueEntry
			err = json.Unmarshal(data, &entry)
			if err != nil {
				return err
			}
			if checkDate && entry.UnqueueTime.After(now) {
				continue
			}
			outdatedList = append(outdatedList, entry)
		}
		return nil
	})
	if err != nil {
		return err, 0
	}

	if len(outdatedList) == 0 {
		return nil, 0
	}

	return errorDb.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(queueName))
		count = 0
		for _, entry := range outdatedList {
			select {
			case ch <- entry:
				{
					err = bucket.Delete([]byte(entry.MessageId))
					if err != nil {
						return err
					}
					count++
					log.Info("msg %s UNQUEUED from %s", entry.String(), queueName)
				}
			default:
				return nil
			}
		}

		return nil
	}), count
}
