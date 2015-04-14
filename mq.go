package main

import (
	"encoding/json"
	"fmt"
	"smtprelay/smtpd"
	"strings"
    "time"
    "github.com/boltdb/bolt"
)

const MAIL_BUCKET_NAME = "MAIL"
const ERROR_BUCKET_NAME = "ERROR"

var (
    db *bolt.DB
)

type QueueStats struct{
    DBStats bolt.TxStats
    MailStats bolt.BucketStats
    ErrorStats bolt.BucketStats
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

func InitQueues(filename string) error {
	var err error
    db, err = bolt.Open(filename, 0600, &bolt.Options{Timeout: 2 * time.Second})
    if err != nil {
        return err
    }

    err = db.Update(func(tx *bolt.Tx) error {
        _, err = tx.CreateBucketIfNotExists([]byte(MAIL_BUCKET_NAME))
        if err != nil {
            return err
        }
        _, err = tx.CreateBucketIfNotExists([]byte(ERROR_BUCKET_NAME))
        if err != nil {
            return err
        }
        return nil
    })
    if err != nil {
        return err
    }

    return nil
}

func CloseQueues(){
    db.Close()
}


func PutMail(entry QueueEntry) error {
	return Put(entry, MAIL_BUCKET_NAME)
}

func PutError(entry QueueEntry) error {
	return Put(entry, ERROR_BUCKET_NAME)
}

func ExtractMail(ch chan QueueEntry) error{
    return Extract(ch,MAIL_BUCKET_NAME,false)
}

func ExtractError(ch chan QueueEntry) error{
    return Extract(ch,ERROR_BUCKET_NAME,true)
}

func Put(entry QueueEntry, queueName string) error {
	json, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return  db.Batch(func(tx *bolt.Tx) error {
    bucket := tx.Bucket([]byte(queueName))
    return bucket.Put([]byte(entry.MessageId),json)
    })
}

func Extract(ch chan QueueEntry, queueName string,checkDate bool) error {
    var err error
    now := time.Now()
    return db.Update(func(tx *bolt.Tx) error {
        bucket := tx.Bucket([]byte(queueName))
        cursor := bucket.Cursor()
        for key,data := cursor.First();key!=nil;key,data = cursor.Next() {
            var entry QueueEntry
            err = json.Unmarshal(data, &entry)
            if err != nil {
                return err
            }
            if checkDate && entry.UnqueueTime.After(now){
                continue
            }
            select {
                case ch <- entry: {
                    err = bucket.Delete(key)
                    if err != nil {
                        return err
                    }
                }
                default: return nil
            }
        }
        return nil
    })
}

func GetQueueLength(queueName string) int {
    var qStats bolt.BucketStats
    err := db.View(func(tx *bolt.Tx) error {
        qStats = tx.Bucket([]byte(queueName)).Stats()
        return nil
    })
    if err != nil {
        return 0
    }
    return qStats.KeyN
}

func GetErrorQueueLength()int{
    return  GetQueueLength(ERROR_BUCKET_NAME)
}

func GetQueueStatistics() (data []byte,err error) {
    var stats QueueStats
    err = db.View(func(tx *bolt.Tx) error {
        stats.DBStats = tx.Stats()
        stats.MailStats = tx.Bucket([]byte(MAIL_BUCKET_NAME)).Stats()
        stats.ErrorStats = tx.Bucket([]byte(ERROR_BUCKET_NAME)).Stats()
        return nil
    })
    if err != nil {
        return data,err
    }
    data,err = json.Marshal(stats)
    if err != nil {
        return data,err
    }
    return data,err
}





