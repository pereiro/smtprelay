package main

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

func GetErrorQueueLength() int64 {
	return int64(len(ErrorChannel))
}

func GetMailQueueLength() int64 {
	return int64(len(MailChannel))
}

var (
	MailHandlersCounter int64
	MailSendersCounter  int64
)

type QueueStats struct {
	OverallCounter        int64
	ErrorBufferCounter    int64
	MailBufferCounter     int64
	MailHandlersCounter   int64
	MailSendersCounter    int64
	MailExtractorsCounter int64
}

func GetStatistics() (data []byte, err error) {
	var stats QueueStats
	stats.MailHandlersCounter = MailHandlersCounter
	stats.MailSendersCounter = MailSendersCounter
	stats.ErrorBufferCounter = int64(len(ErrorChannel))
	stats.MailBufferCounter = int64(len(MailChannel))
	stats.MailExtractorsCounter = int64(len(ExtractorLimiter))
	stats.OverallCounter = stats.ErrorBufferCounter + stats.MailBufferCounter
	data, err = json.Marshal(stats)
	if err != nil {
		return data, err
	}
	return data, err
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
