package main

import (
	"encoding/json"
	"fmt"
	"relay/redismq"
	"relay/smtpd"
	"strings"
)

const MAIL_CONSUMER_NAME = "MAIL"
const ERROR_CONSUMER_NAME = "ERROR"

var (
	StatisticServer *redismq.Server
	MailQueue       *redismq.BufferedQueue
	MailConsumer    *redismq.Consumer
	ErrorQueue      *redismq.BufferedQueue
	ErrorConsumer   *redismq.Consumer
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
}

func (e QueueEntry) String() string {
	return fmt.Sprintf("(message-id:%s;from:%s;to:%s)", e.MessageId, e.Sender, strings.Join(e.Recipients, ";"))
}

func InitQueues() error {
	var err error
	MailQueue, err = redismq.SelectBufferedQueue(conf.RedisHost, conf.RedisPort, conf.RedisPassword, conf.RedisDB, conf.RedisMailQueueName, conf.MQQueueBuffer)
	if err != nil {
		MailQueue = redismq.CreateBufferedQueue(conf.RedisHost, conf.RedisPort, conf.RedisPassword, conf.RedisDB, conf.RedisMailQueueName, conf.MQQueueBuffer)
	}
	MailConsumer, err = MailQueue.AddConsumer(MAIL_CONSUMER_NAME)
	err = MailQueue.Start()
	if err != nil {
		return err
	}

	ErrorQueue, err = redismq.SelectBufferedQueue(conf.RedisHost, conf.RedisPort, conf.RedisPassword, conf.RedisDB, conf.RedisErrorQueueName, conf.MQQueueBuffer)
	if err != nil {
		ErrorQueue = redismq.CreateBufferedQueue(conf.RedisHost, conf.RedisPort, conf.RedisPassword, conf.RedisDB, conf.RedisErrorQueueName, conf.MQQueueBuffer)
	}
	ErrorConsumer, err = ErrorQueue.AddConsumer(ERROR_CONSUMER_NAME)
	err = ErrorQueue.Start()
	if err != nil {
		return err
	}

	return nil
}

func PutMail(entry QueueEntry) error {
	return Put(entry, MailQueue)
}

func GetMail() (QueueEntry, error) {
	return Get(MailQueue, MailConsumer)
}

func MultiGetMail(ch chan QueueEntry) error {
	return MultiGet(ch, MailQueue, MailConsumer)
}

func PutError(entry QueueEntry) error {
	return Put(entry, ErrorQueue)
}

func GetError() (QueueEntry, error) {
	return Get(ErrorQueue, ErrorConsumer)
}

func MultiGetError(ch chan QueueEntry) error {
	return MultiGet(ch, ErrorQueue, ErrorConsumer)
}

func Put(entry QueueEntry, q *redismq.BufferedQueue) error {
	json, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return q.Put(string(json))
}

func Get(q *redismq.BufferedQueue, c *redismq.Consumer) (entry QueueEntry, err error) {
	var pkg *redismq.Package
	if c.HasUnacked() {
		pkg, err = c.GetUnacked()
		if err != nil {
			return
		}
	} else {
		pkg, err = c.Get()
		if err != nil {
			return
		}
	}
	err = json.Unmarshal([]byte(pkg.Payload), &entry)
	if err != nil {
		return
	}
	err = pkg.Ack()
	if err != nil {
		return
	}
	return entry, nil
}

func MultiGet(ch chan QueueEntry, q *redismq.BufferedQueue, c *redismq.Consumer) (err error) {
	var pkgs []*redismq.Package
	pkgs = *new([]*redismq.Package)
	entry := *new(QueueEntry)
	if c.HasUnacked() {
		entry, err = Get(q, c)
		if err != nil {
			return
		}
		ch <- entry
		return nil
	} else {
		pkgs, err = c.MultiGet(conf.MQQueueBuffer)
		if err != nil {
			return
		}
		for _, pkg := range pkgs {
			err = json.Unmarshal([]byte(pkg.Payload), &entry)
			if err != nil {
				return
			}
			log.Info("msg %s UNQUEUED from %s", entry.String(), q.Name)
			err = pkg.Ack()
			if err != nil {
				log.Error("msg %s ACK error: %s", err.Error())
			} else {
				ch <- entry
			}
		}
		//        err = pkgs[len(pkgs)-1].MultiAck()
		//        if err != nil {
		//            return
		//        }
	}
	return nil
}
