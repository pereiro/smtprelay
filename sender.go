package main

import (
	"smtprelay/smtp"
	"time"
)

var (
	SenderLimiter    chan interface{}
	ExtractorLimiter chan interface{}
)

func StartSender() {
	SenderLimiter = make(chan interface{}, conf.MaxOutcomingConnections)
	go CloneMailers()
	go StartErrorHandler()
}

func StartErrorHandler() {

//	for {
//		if GetErrorQueueLength() == 0 || GetMailQueueLength() > 0 {
//			time.Sleep(1000 * time.Millisecond)
//		} else {
//			entry, success := ExtractError()
//			if success {
//				PushMail(entry)
//			} else {
//				time.Sleep(1000 * time.Millisecond)
//			}
//		}
//	}
	for {
				entry:= ExtractError()
				PushMail(entry)
		}
	}



func CloneMailers() {
	for {
			entry := PopMail()
			SenderLimiter <- 0
			go SendMail(entry)
	}
}

func SendMail(entry QueueEntry) {
	MailSendersIncreaseCounter(1)
	defer func() {
		MailSendersDecreaseCounter(1)
		<-SenderLimiter
	}()
	//log.Info("msg %s READY for processing", entry.String())
	var err error
	var data []byte
	var signed = ""
	if conf.DKIMEnabled {
		data, err = DKIMSign(entry.Data, entry.SenderDomain)
		if err != nil {
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
				log.Error("msg %s DEFER LIMIT=(%d/%d) DROPPED: %s", entry.String(), entry.ErrorCount, conf.DeferredMailMaxErrors, smtpError.Error())
				return
			}
			entry.QueueTime = time.Now()
			entry.UnqueueTime = entry.QueueTime.Add(time.Duration(conf.DeferredMailDelay) * time.Second)
			oldMX := entry.MailServer
			if conf.RelayModeEnabled {
				entry.MailServer = conf.RelayServer
			}else {
				entry.MailServer, err = lookupMailServer(entry.RecipientDomain, entry.ErrorCount)
				if err != nil {
					entry.MailServer = oldMX
					log.Warn("msg %s (%d/%d) (next attempt at %s ) can't find secondary MX record, old MX will be used: %s", entry.String(), entry.ErrorCount, conf.DeferredMailMaxErrors, entry.UnqueueTime, oldMX)
				}
			}

			PushError(entry)
			log.Error("msg %s (%d/%d) (next attempt at %s ) DEFERRED: %s", entry.String(), entry.ErrorCount, conf.DeferredMailMaxErrors, entry.UnqueueTime, smtpError.Error())
		}
	} else {
		log.Info("msg %s SENT%s: %s", entry.String(), signed, ErrStatusSuccess.Error())
	}

}
