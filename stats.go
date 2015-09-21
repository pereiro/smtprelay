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
	MailSendersCounter  int64
	MailHandlersCounter int64
	MaxQueueCounter     int64
	MailSentCounter     int64
	MailDroppedCounter  int64
)

type QueueStats struct {
	OverallCounter               int64
	ErrorBufferCounter           int64
	MailBufferCounter            int64
	OutboundSMTPConnects         int64
	InboundTCPHandlers           int64
	InboundTCPConnects           int64
	InboundSMTPConnects          int64
	MaxQueueSizeSinceLastRestart int64
	MailSentSinceLastRestart     int64
	MailDroppedSinceLastRestart  int64
	Configuration                *Conf
}

func GetStatistics() (data []byte, err error) {
	var stats QueueStats
	stats.OutboundSMTPConnects = MailSendersCounter
	stats.InboundSMTPConnects = MailHandlersCounter
	stats.ErrorBufferCounter = int64(len(ErrorChannel))
	stats.MailBufferCounter = int64(len(MailChannel))
	stats.InboundTCPHandlers = int64(len(TCPHandlersLimiter))
	stats.InboundTCPConnects = int64(len(TCPConnectionsLimiter))
	stats.OverallCounter = stats.ErrorBufferCounter + stats.MailBufferCounter
	stats.Configuration = conf
	data, err = json.Marshal(stats)
	if err != nil {
		return data, err
	}
	return data, err
}

func MailSentIncreaseCounter(count int) {
	atomic.AddInt64(&MailSentCounter, int64(count))
}

func MailDroppedIncreaseCounter(count int) {
	atomic.AddInt64(&MailDroppedCounter, int64(count))
}

func MailQueueCheckMax() {
	if currentValue := GetMailQueueLength() + GetErrorQueueLength(); currentValue > MaxQueueCounter {
		atomic.StoreInt64(&MaxQueueCounter, currentValue)
	}
}

func MailSendersIncreaseCounter(count int) {
	atomic.AddInt64(&MailSendersCounter, int64(count))
}

func MailSendersDecreaseCounter(count int) {
	atomic.AddInt64(&MailSendersCounter, -int64(count))
}

func MailHandlersIncreaseCounter(count int) {
	atomic.AddInt64(&MailHandlersCounter, int64(count))
}

func MailHandlersDecreaseCounter(count int) {
	atomic.AddInt64(&MailHandlersCounter, -int64(count))
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
