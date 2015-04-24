package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"smtprelay/smtpd"
	"strings"
	"time"
)

var DATA_BUCKET = []byte("Q")
var METADATA_BUCKET = []byte("M")
var META_KEY = []byte("m")

const MAX_ERROR_BUFFER_SIZE = 1000000
const MAX_MAIL_BUFFER_SIZE = 1000
const MAX_TRANSACTION_LENGTH = 1000

var (
	errorDb           *bolt.DB
	mailDb            *bolt.DB
	MailDirectChannel chan QueueEntry
	ErrorQueueChannel chan QueueEntry
)

type QueueMetaData struct {
	FirstKey   uint64
	CurrentKey uint64
	LastKey    uint64
}

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
		eBucket, err := tx.CreateBucketIfNotExists(DATA_BUCKET)
		if err != nil {
			return err
		}
		errorCounter = int64(eBucket.Stats().KeyN)
		emBucket, err := tx.CreateBucketIfNotExists(METADATA_BUCKET)
		if err != nil {
			return err
		}
		return InitMetaData(emBucket)

	})

	err = mailDb.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(DATA_BUCKET)
		if err != nil {
			return err
		}
		//mailCounter = int64(mBucket.Stats().KeyN)
		mmBucket, err := tx.CreateBucketIfNotExists(METADATA_BUCKET)
		if err != nil {
			return err
		}
		err = InitMetaData(mmBucket)
		if err != nil {
			return err
		}
		metadata, err := GetMetaData(mmBucket)
		if err != nil {
			return err
		}
		mailCounter = int64(metadata.LastKey - metadata.CurrentKey)
		return nil
	})

	if err != nil {
		return err
	}
	SetCounterInitialValues(errorCounter, mailCounter)
	go QueueHandler(ErrorQueueChannel, DATA_BUCKET, &ErrorQueueCounter, 2000)
	return nil
}

func (m QueueMetaData) Empty() bool {
	return m.CurrentKey == m.LastKey
}

func InitMetaData(b *bolt.Bucket) error {
	var metadata QueueMetaData
	value := b.Get(META_KEY)
	if value == nil {
		metadata = QueueMetaData{}
		bytes, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		err = b.Put(META_KEY, bytes)
		if err != nil {
			return err
		}
	}
	return nil
}

func CloseQueues() {
	errorDb.Close()
}

func QueueHandler(ch chan QueueEntry, queueName []byte, queueCounter *int64, bufferSize int) {
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

func GetMetaData(b *bolt.Bucket) (*QueueMetaData, error) {
	var metadata QueueMetaData
	value := b.Get(META_KEY)
	if err := json.Unmarshal(value, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

func GetMailMetaData() (metadata *QueueMetaData, err error) {
	err = mailDb.View(func(tx *bolt.Tx) error {
		mBucket := tx.Bucket(METADATA_BUCKET)
		metadata, err = GetMetaData(mBucket)
		if err != nil {
			return err
		}
		return nil
	})
	return
}

func PutMetadata(b *bolt.Bucket, m *QueueMetaData) error {
	metadataBytes, err := json.Marshal(m)
	if err != nil {
		return err
	}

	err = b.Put(META_KEY, metadataBytes)
	if err != nil {
		return err
	}
	return nil
}

func GetKey(key uint64) []byte {
	keyBytes := make([]byte, 8)
	binary.PutUvarint(keyBytes, key)
	return keyBytes
}

func Push(entry QueueEntry, db *bolt.DB) error {
	bytes, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		dBucket := tx.Bucket(DATA_BUCKET)
		mBucket := tx.Bucket(METADATA_BUCKET)
		metadata, err := GetMetaData(mBucket)
		if err != nil {
			return err
		}

		err = dBucket.Put(GetKey(metadata.LastKey), bytes)
		if err != nil {
			return err
		}
		metadata.LastKey += 1

		err = PutMetadata(mBucket, metadata)
		if err != nil {
			return err
		}

		return nil
	})
	MailQueueIncreaseCounter(1)
	return err
}

func Pop(db *bolt.DB) (QueueEntry, bool, error) {
	var bytes []byte
	var entry QueueEntry

	err := db.Update(func(tx *bolt.Tx) error {
		dBucket := tx.Bucket(DATA_BUCKET)
		mBucket := tx.Bucket(METADATA_BUCKET)
		metadata, err := GetMetaData(mBucket)
		if err != nil {
			return err
		}
		log.Debug("POPCHECK EMPTY metadata.Current=%d,Last=%d. Empty=%t",metadata.CurrentKey,metadata.LastKey,metadata.Empty())
		if metadata.Empty() {
			return nil
		}
		log.Debug("POPCHECK BYTES metadata.Current=%d,Last=%d. Empty=%t",metadata.CurrentKey,metadata.LastKey,metadata.Empty())
		bytes := dBucket.Get(GetKey(metadata.CurrentKey))
		if bytes == nil {
			return nil
		}
		log.Debug("POPCHECK END metadata.Current=%d,Last=%d. Empty=%t",metadata.CurrentKey,metadata.LastKey,metadata.Empty())
		metadata.CurrentKey += 1

		err = PutMetadata(mBucket, metadata)
		if err != nil {
			return err
		}
		MailQueueDecreaseCounter(1)
		return nil
	})
	if err != nil {
		return entry, false, err
	}
	if bytes == nil {
		return entry, false, err
	}
	err = json.Unmarshal(bytes, &entry)
	if err != nil {
		return entry, false, err
	}
	return entry, true, nil

}

func PushMail(entry QueueEntry) error {
	select {
	case MailDirectChannel <- entry:
	default:
		{
			err := Push(entry, mailDb)
			if err != nil {
				log.Error("error pushing mail:%s", err.Error())
			} else {
			}
		}
	}
	return nil
}

func PopMail() (entry QueueEntry, success bool, err error) {
	success = false
	entry, success, err = Pop(mailDb)
	if err != nil {
		return entry, false, err
	}
	if !success {
		return entry, false, nil
	}
	return entry, true, err
}

func PutError(entry QueueEntry) error {
	ErrorQueueChannel <- entry
	return nil
}

func ExtractError(ch chan QueueEntry) error {
	err, count := Extract(ch, DATA_BUCKET, true)
	if err != nil {
		return err
	}
	ErrorQueueDecreaseCounter(count)
	return nil
}

func Extract(ch chan QueueEntry, queueName []byte, checkDate bool) (error, int) {
	var err error
	var count int
	var outdatedList []QueueEntry

	now := time.Now()

	err = errorDb.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(queueName)
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
		bucket := tx.Bucket(queueName)
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
