package main

import (
	"encoding/json"
	"fmt"
	"smtprelay/smtpd"
	"strings"
    "time"
    "github.com/boltdb/bolt"
    "sync/atomic"
)

const MAIL_BUCKET_NAME = "MAIL"
const ERROR_BUCKET_NAME = "ERROR"

var (
    db *bolt.DB
    ErrorQueueLength uint64
    MailQueueLength uint64
)

type QueueStats struct{
    ErrorQueueLength uint64
    MailQueueLength uint64
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
    err:= Put(entry, MAIL_BUCKET_NAME)
    if err != nil {
        return err
    }
    atomic.AddUint64(&MailQueueLength,1)
    return nil
}

func PutError(entry QueueEntry) error {
    err:= Put(entry, ERROR_BUCKET_NAME)
    if err != nil {
        return err
    }
    atomic.AddUint64(&ErrorQueueLength,1)
	return nil
}

func ExtractMail(ch chan QueueEntry) error{
    err,count := Extract(ch,MAIL_BUCKET_NAME,false)
    if err != nil {
        return err
    }
    atomic.AddUint64(&MailQueueLength,uint64(count))
    return nil
}

func ExtractError(ch chan QueueEntry) error{
    err,count := Extract(ch,ERROR_BUCKET_NAME,false)
    if err != nil {
        return err
    }
    atomic.AddUint64(&ErrorQueueLength,uint64(count))
    return nil
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

func Extract(ch chan QueueEntry, queueName string,checkDate bool) (error,int) {
    var err error
    var count int
    now := time.Now()
    return db.Update(func(tx *bolt.Tx) error {
        bucket := tx.Bucket([]byte(queueName))
        cursor := bucket.Cursor()
        count = 0
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
                    count++
                }
                default: return nil
            }
        }
        return nil
    }),count
}

//func GetQueueLength(queueName string) int {
//    var qStats bolt.BucketStats
//    err := db.View(func(tx *bolt.Tx) error {
//        qStats = tx.Bucket([]byte(queueName)).Stats()
//        return nil
//    })
//    if err != nil {
//        return 0
//    }
//    return qStats.KeyN
//}

func GetErrorQueueLength() uint64{
    return  ErrorQueueLength
}

func GetMailQueueLength() uint64{
    return  MailQueueLength
}

func GetQueueStatistics() (data []byte,err error) {
    var stats QueueStats
    stats.ErrorQueueLength = ErrorQueueLength
    stats.MailQueueLength = MailQueueLength
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





