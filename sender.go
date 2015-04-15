package main

import (
	"smtprelay/smtp"
	"time"
    "net/http"
)

const ERRORQUEUETHRESHOLD = 100

var (
	MailMQChannel chan QueueEntry
)

func StartSender() {
	MailMQChannel = make(chan QueueEntry, conf.MaxOutcomingConnections)
	go CloneMailers()
	go StartErrorHandler()
	for {
        if GetMailQueueLength()==0 {
            time.Sleep(1000 * time.Millisecond)
            continue
        }
		err := ExtractMail(MailMQChannel)
		if err != nil {
			log.Error("error reading msg from Mail Queue DB:%s", err.Error())
		}

	}
}

func StartErrorHandler() {

	for {
        if GetErrorQueueLength()==0 || GetMailQueueLength()>0{
            time.Sleep(1000 * time.Millisecond)
            continue
        }
        err := ExtractError(MailMQChannel)
        if err != nil {
            log.Error("error reading msg from Error Queue DB:%s", err.Error())
        }
	}
}

func StatisticHandler(w http.ResponseWriter, r *http.Request) {
    js,err := GetQueueStatistics()
    if err != nil {
        log.Error("can't start statistics server:%s",err)
    }
    w.Header().Set("Content-Type", "application/json")
    w.Write(js)
}


func StartStatisticServer() {
    http.HandleFunc("/", StatisticHandler)
    http.ListenAndServe(":"+conf.StatisticPort, nil)
}

func CloneMailers() {
	for {
		entry := <-MailMQChannel
		go SendMail(entry)
	}
}

func SendMail(entry QueueEntry) {
	log.Info("msg %s READY for processing", entry.String())
	var err error
	var data []byte
	var signed = ""
	if conf.DKIMEnabled {
		data, err = DKIMSign(entry.Data, entry.SenderDomain)
		if err != nil {
			//log.Warn("can't sign msg:%s",err.Error())
			data = entry.Data
			signed = "(NOT SIGNED)"
		}
	} else {
		data = entry.Data
	}

	if err := smtp.SendMail(
		entry.MailServer,
		nil,
		entry.Sender,
		entry.Recipients,
		data,
		conf.ServerHostName); err != nil {
		smtpError := ParseOutcomingError(err.Error())
		if smtpError.Code/100 == 5 {
			log.Error("msg %s DROPPED: %s", entry.String(), smtpError.Error())
			return
		} else {
			entry.ErrorCount += 1
			entry.Error = smtpError
			if entry.ErrorCount >= conf.DeferredMailMaxErrors {
				log.Error("msg %s DROPPED defer limit =(%d/%d): %s", entry.String(), entry.ErrorCount, conf.DeferredMailMaxErrors, smtpError.Error())
				return
			}
            entry.QueueTime = time.Now()
            entry.UnqueueTime = entry.QueueTime.Add(time.Duration(conf.DeferredMailDelay)*time.Second)
			err := PutError(entry)
			if err != nil {
				log.Error("msg %s DROPPED, can't defer cause of %s: %s", entry.String(), err.Error(), smtpError.Error())
			}
			log.Error("msg %s DEFERRED (%d/%d): %s", entry.String(), entry.ErrorCount, conf.DeferredMailMaxErrors, smtpError.Error())
		}
	} else {
		log.Info("msg %s SENT%s: %s", entry.String(), signed, ErrStatusSuccess.Error())
	}

}

//func BufferLengthPercent() int {
//    return int((float32(len(MailMQChannel))/float32(cap(MailMQChannel)))*100)
//}