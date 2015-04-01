package main
import ("relay/smtp"
    "time"
    "github.com/adjust/redismq"
)

var(
    MailMQChannel chan QueueEntry
    Observer *redismq.Observer
)

func StartSender(){
    MailMQChannel = make(chan QueueEntry,conf.MaxOutcomingConnections)
    go CloneMailers()
    go StartErrorHandler()
    for{
            err := MultiGetMail(MailMQChannel)
            if err != nil {
                log.Error("error reading msg from Mail MQ:%s", err.Error())
                time.Sleep(1000 * time.Millisecond)
            }
    }
}

func StartErrorHandler(){
    Observer = redismq.NewObserver(conf.RedisHost,conf.RedisPort,conf.RedisPassword,conf.RedisDB)
    for{
        err := MultiGetError(MailMQChannel)
        if err != nil {
            log.Error("error reading msg from Error MQ:%s", err.Error())
            time.Sleep(1000 * time.Millisecond)
        }
        Observer.UpdateAllStats()
        if Observer.Stats[conf.RedisErrorQueueName].InputSizeSecond<int64(conf.MQQueueBuffer) {
            time.Sleep(time.Duration(conf.DeferredMailDelay) * time.Second)
        }
    }
}

func StartStatisticServer(){
    StatisticServer = redismq.NewServer(conf.RedisHost,conf.RedisPort,conf.RedisPassword,conf.RedisDB,conf.MQStatisticPort)
    StatisticServer.Start()
}

func CloneMailers(){
    for{
        entry := <-MailMQChannel
        go SendMail(entry)
    }
}

func SendMail(entry QueueEntry){
    var err error
    var data []byte
    if(conf.DKIMEnabled) {
        data,err = DKIMSign(entry.Data,entry.SenderDomain)
        if err != nil {
            log.Warn("can't sign msg:%s",err.Error())
            data = entry.Data
        }
    }else{
        data = entry.Data
    }

    if err:=smtp.SendMail(
    entry.MailServer,
    nil,
    entry.Sender,
    entry.Recipients,
    data,
    );err!=nil{
        smtpError:=ParseOutcomingError(err.Error())
        if(smtpError.Code/100==5){
            log.Error("msg DROPPED from %s to %s(%s): %s", entry.Sender,entry.Recipients[0],entry.MailServer,smtpError.Error())
            return
        }else {
            entry.ErrorCount += 1
            entry.Error = smtpError
            if (entry.ErrorCount>conf.DeferredMailMaxErrors){
                log.Error("msg DROPPED defer count=(%d/%d) from %s to %s(%s): %s",
                    entry.ErrorCount,conf.DeferredMailMaxErrors,entry.Sender, entry.Recipients[0], entry.MailServer, smtpError.Error())
                return
            }
            err:=PutError(entry)
            if err != nil {
                log.Error("can't DEFER msg from %s to %s(%s), msg DROPPED: %s", entry.Sender, entry.Recipients[0], entry.MailServer, smtpError.Error())
            }
            log.Error("msg DEFERRED (%d/%d) from %s to %s(%s): %s",entry.ErrorCount,conf.DeferredMailMaxErrors,entry.Sender, entry.Recipients[0], entry.MailServer, smtpError.Error())
        }
    }else{
        log.Info("msg from %s to %s(%s): %s",entry.Sender,entry.Recipients[0],entry.MailServer,ErrStatusSuccess.Error())
    }

}