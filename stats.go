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
	MailQueueCounter    int64
	MailHandlersCounter int64
	MailSendersCounter  int64
)

type QueueStats struct {
	OverallCounter        int64
	ErrorQueueCounter     int64
	MailQueueCounter      int64
	ErrorBufferCounter    int64
	MailBufferCounter     int64
	MailHandlersCounter   int64
	MailSendersCounter    int64
	MailExtractorsCounter int64
	MailDBStats           bolt.BucketStats
	ErrorDBStats          bolt.BucketStats
}

func SetCounterInitialValues(errors int64, mails int64) {
	ErrorQueueCounter = errors
	MailQueueCounter = mails
}

func GetStatistics() (data []byte, err error) {
	var stats QueueStats
	stats.ErrorQueueCounter = ErrorQueueCounter
	stats.MailQueueCounter = MailQueueCounter
	stats.MailHandlersCounter = MailHandlersCounter
	stats.MailSendersCounter = MailSendersCounter
	stats.ErrorBufferCounter = int64(len(ErrorQueueChannel))
	stats.MailBufferCounter = int64(len(MailDirectChannel))
	stats.MailExtractorsCounter = int64(len(ExtractorLimiter))
	stats.OverallCounter = stats.ErrorQueueCounter + stats.MailQueueCounter + stats.ErrorBufferCounter + stats.MailBufferCounter

	err = errorDb.View(func(tx *bolt.Tx) error {
		stats.ErrorDBStats = tx.Bucket([]byte(ERROR_BUCKET_NAME)).Stats()
		return nil
	})
	if err != nil {
		return data, err
	}
	err = mailDb.View(func(tx *bolt.Tx) error {
		stats.MailDBStats = tx.Bucket([]byte(MAIL_BUCKET_NAME)).Stats()
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

func MailQueueIncreaseCounter(count int) {
	atomic.AddInt64(&MailQueueCounter, int64(count))
}

func MailQueueDecreaseCounter(count int) {
	atomic.AddInt64(&MailQueueCounter, -int64(count))
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

func MailSendersDecreaseCounter(count int) {
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
