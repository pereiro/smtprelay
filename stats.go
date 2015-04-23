package main

import (
	"encoding/json"
	"github.com/boltdb/bolt"
	"net/http"
	"sync/atomic"
)

func GetErrorQueueLength() int64 {
	return ErrorQueueCounter
}

func GetMailQueueLength() int64 {
	return int64(len(MailDirectChannel))
}

var (
	ErrorQueueCounter   int64
	MailHandlersCounter int64
	MailSendersCounter  int64
)

type QueueStats struct {
	OverallCounter      int64
	ErrorQueueCounter   int64
	MailQueueCounter    int64
	ErrorBufferCounter  int
	MailHandlersCounter int64
	MailSendersCounter  int64
	DBStats             bolt.TxStats
	MailStats           bolt.BucketStats
	ErrorStats          bolt.BucketStats
}

func SetCounterInitialValues(errors int64, mails int64) {
	ErrorQueueCounter = errors
}

func GetStatistics() (data []byte, err error) {
	var stats QueueStats
	stats.ErrorQueueCounter = ErrorQueueCounter
	stats.MailQueueCounter = int64(len(MailDirectChannel))
	stats.MailHandlersCounter = MailHandlersCounter
	stats.MailSendersCounter = MailSendersCounter
	stats.ErrorBufferCounter = len(ErrorQueueChannel)
	stats.OverallCounter = stats.ErrorQueueCounter + stats.MailQueueCounter + int64(stats.ErrorBufferCounter)
	err = db.View(func(tx *bolt.Tx) error {
		stats.DBStats = tx.Stats()
		stats.MailStats = tx.Bucket([]byte(MAIL_BUCKET_NAME)).Stats()
		stats.ErrorStats = tx.Bucket([]byte(ERROR_BUCKET_NAME)).Stats()
		return nil
	})
	if err != nil {
		return data, err
	}
	data, err = json.Marshal(stats)
	if err != nil {
		return data, err
	}
	return data, err
}

func QueueIncreaseCounter(counter *int64, count int) {
	atomic.AddInt64(counter, int64(count))
}

func ErrorQueueDecreaseCounter(count int) {
	atomic.AddInt64(&ErrorQueueCounter, -int64(count))
}

func MailHandlersIncreaseCounter(count int) {
	atomic.AddInt64(&MailHandlersCounter, int64(count))
}

func MailHandlersDecreaseCounter(count int) {
	atomic.AddInt64(&MailHandlersCounter, -int64(count))
}

func MailSendersIncreaseCounter(count int) {
	atomic.AddInt64(&MailSendersCounter, int64(count))
}

func MailSenderssDecreaseCounter(count int) {
	atomic.AddInt64(&MailSendersCounter, -int64(count))
}

func StatisticHandler(w http.ResponseWriter, r *http.Request) {
	js, err := GetStatistics()
	if err != nil {
		log.Error("can't start statistics server:%s", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func StartStatisticServer() {
	http.HandleFunc("/", StatisticHandler)
	http.ListenAndServe(":"+conf.StatisticPort, nil)
}
