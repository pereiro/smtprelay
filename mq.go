package main

import (
    "github.com/adjust/redismq"
    "encoding/json"
    "smtpd"
)

const MAIL_CONSUMER_NAME = "MAIL"
const ERROR_CONSUMER_NAME = "ERROR"

type MQ struct{
    *redismq.BufferedQueue
}

type MQConsumer struct {
    *redismq.Consumer
}

var (
    StatisticServer *redismq.Server
    MailQueue MQ
    MailConsumer MQConsumer
    ErrorQueue MQ
    ErrorConsumer MQConsumer
)

type QueueEntry struct{
    MailServer string
    Sender string
    Recipients []string
    SenderDomain string
    Data []byte
    Error smtpd.Error
    ErrorCount int
}

func InitQueues() error {
    var err error
        MailQueue.BufferedQueue, err = redismq.SelectBufferedQueue(conf.RedisHost, conf.RedisPort, conf.RedisPassword, conf.RedisDB, conf.RedisMailQueueName,conf.MQQueueBuffer)
        if err != nil {
            MailQueue.BufferedQueue = redismq.CreateBufferedQueue(conf.RedisHost, conf.RedisPort, conf.RedisPassword, conf.RedisDB, conf.RedisMailQueueName,conf.MQQueueBuffer)
        }
        MailConsumer.Consumer, err = MailQueue.AddConsumer(MAIL_CONSUMER_NAME)
        err = MailQueue.Start()
        if err != nil {
            return err
        }

        ErrorQueue.BufferedQueue, err = redismq.SelectBufferedQueue(conf.RedisHost, conf.RedisPort, conf.RedisPassword, conf.RedisDB, conf.RedisErrorQueueName,conf.MQQueueBuffer)
        if err != nil {
            ErrorQueue.BufferedQueue = redismq.CreateBufferedQueue(conf.RedisHost, conf.RedisPort, conf.RedisPassword, conf.RedisDB, conf.RedisErrorQueueName,conf.MQQueueBuffer)
        }
        ErrorConsumer.Consumer, err = ErrorQueue.AddConsumer(ERROR_CONSUMER_NAME)
        err = ErrorQueue.Start()
        if err != nil {
            return err
        }    
    
    
    return nil
}

func PutMail(entry QueueEntry) error{
    return Put(entry,MailQueue)
}

func GetMail() (QueueEntry, error){
    return Get(MailQueue,MailConsumer)
}

func MultiGetMail(ch chan QueueEntry) error{
    return MultiGet(ch,MailQueue,MailConsumer)
}

func PutError(entry QueueEntry) error{
    return Put(entry,ErrorQueue)
}

func GetError() (QueueEntry, error){
    return Get(ErrorQueue,ErrorConsumer)
}

func MultiGetError(ch chan QueueEntry) error{
    return MultiGet(ch,ErrorQueue,ErrorConsumer)
}




func Put(entry QueueEntry,q MQ) error {
    json,err:=json.Marshal(entry)
    if err != nil {
        return err
    }
        return q.Put(string(json))
}

func Get(q MQ,c MQConsumer) (entry QueueEntry, err error){
    var pkg *redismq.Package
    if c.HasUnacked(){
        pkg, err = c.GetUnacked()
        if err != nil {
            return
        }
    }else {
        pkg, err = c.Get()
        if err != nil {
            return
        }
    }
    err = json.Unmarshal([]byte(pkg.Payload),&entry)
    if err != nil {
        return
    }
    err = pkg.Ack()
    if err != nil {
        return
    }
    return entry,nil
}

func MultiGet(ch chan QueueEntry,q MQ,c MQConsumer) (err error){
    var pkgs []*redismq.Package
    var entry QueueEntry
    if c.HasUnacked(){
        entry,err = Get(q,c)
        if err != nil {
            return
        }
        ch <- entry
        return nil
    }else {
        pkgs, err = c.MultiGet(conf.MQQueueBuffer)
        if err != nil {
            return
        }
        for _,pkg := range pkgs{
            err = json.Unmarshal([]byte(pkg.Payload),&entry)
            if err != nil {
                return
            }
            ch <- entry
        }
        err = pkgs[len(pkgs)-1].MultiAck()
        if err != nil {
            return
        }

    }

    return nil
}



