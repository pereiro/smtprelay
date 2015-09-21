package main

import (
	"encoding/json"
	"net/http"
)

func GetErrorQueueLength() int64 {
	return int64(len(ErrorChannel))
}

func GetMailQueueLength() int64 {
	return int64(len(MailChannel))
}

const (
	STATISTICS_CHANNELS_SIZE = 100000
)

var (
	MailSendersCounter  int64
	MailHandlersCounter int64
	MaxQueueCounter     int64
	MailSentCounter     int64
	MailDroppedCounter  int64
	MailSentChannel     chan int
	MailDroppedChannel  chan int
	MailMaxQueueChannel chan int
	MailHandlersChannel chan int
	MailSendersChannel  chan int
)

func InitStatistics() {
	MailSentChannel = make(chan int)
	MailDroppedChannel = make(chan int, STATISTICS_CHANNELS_SIZE)
	MailMaxQueueChannel = make(chan int, STATISTICS_CHANNELS_SIZE)
	MailHandlersChannel = make(chan int, STATISTICS_CHANNELS_SIZE)
	MailSendersChannel = make(chan int, STATISTICS_CHANNELS_SIZE)

	go func() {
		for val := range MailSentChannel {
			MailSentCounter += int64(val)
		}
	}()

	go func() {
		for val := range MailDroppedChannel {
			MailDroppedCounter += int64(val)
		}
	}()

	go func() {
		for val := range MailMaxQueueChannel {
			if int64(val) > MaxQueueCounter {
				MaxQueueCounter = int64(val)
			}

		}
	}()

	go func() {
		for val := range MailHandlersChannel {
			MailHandlersCounter += int64(val)
		}
	}()

	go func() {
		for val := range MailSendersChannel {
			MailSendersCounter += int64(val)
		}
	}()
}

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
	stats.MaxQueueSizeSinceLastRestart = MaxQueueCounter
	stats.MailSentSinceLastRestart = MailSentCounter
	stats.MailDroppedSinceLastRestart = MailDroppedCounter
	stats.Configuration = conf
	data, err = json.Marshal(stats)
	if err != nil {
		return data, err
	}
	return data, err
}

func MailSentIncreaseCounter(count int) {
	MailSentChannel <- count
}

func MailDroppedIncreaseCounter(count int) {
	MailDroppedChannel <- count
}

func MailQueueCheckMax() {
	MailMaxQueueChannel <- 1
}

func MailSendersIncreaseCounter(count int) {
	MailSendersChannel <- count
}

func MailSendersDecreaseCounter(count int) {
	MailSendersChannel <- -count
}

func MailHandlersIncreaseCounter(count int) {
	MailHandlersChannel <- count
}

func MailHandlersDecreaseCounter(count int) {
	MailHandlersChannel <- -count
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
	InitStatistics()
	http.HandleFunc("/", StatisticHandler)
	http.ListenAndServe(":"+conf.StatisticPort, nil)
}
