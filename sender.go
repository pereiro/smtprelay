package main

import (
	"smtprelay/smtp"
	"time"
)

const EXTRACTOR_MAX_COUNT = 5

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

	for {
		if GetErrorQueueLength() == 0 || GetMailQueueLength() > 0 {
			time.Sleep(1000 * time.Millisecond)
			continue
		}
		err := ExtractError(MailDirectChannel)
		if err != nil {
			log.Error("error reading msg from Error Queue DB: %s", err.Error())
		}
	}
}

func CloneMailers() {
	ExtractorLimiter = make(chan interface{}, EXTRACTOR_MAX_COUNT)
	for {
		select {
		case entry := <-MailDirectChannel:
			SenderLimiter <- 0
			go SendMail(entry)
		default:
			{
				if MailQueueCounter > 0 {
					ExtractorLimiter <- 0
					go func() {
						defer func() { <-ExtractorLimiter }()
						entry, success, err := PopMail()
						if err != nil {
							log.Error("error reading message from Mail Queue DB: %s", err.Error())
							return
						}
						log.Debug("msg %s POPPED, success = %t", entry.String(), success)
						if success {
							SenderLimiter <- 0
							go SendMail(entry)
						}
					}()
				}
			}
		}

	}
}

func SendMail(entry QueueEntry) {
	MailSendersIncreaseCounter(1)
	defer func() {
		MailSendersDecreaseCounter(1)
		<-SenderLimiter
	}()
	log.Info("msg %s READY for processing", entry.String())
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
			err := PutError(entry)
			if err != nil {
				log.Error("msg %s can't be deferred - %s DROPPED: %s", entry.String(), err.Error(), smtpError.Error())
			}
			log.Error("msg %s (%d/%d) (next attempt at %s ) DEFERRED: %s", entry.String(), entry.ErrorCount, conf.DeferredMailMaxErrors, entry.UnqueueTime, smtpError.Error())
		}
	} else {
		log.Info("msg %s SENT%s: %s", entry.String(), signed, ErrStatusSuccess.Error())
	}

}
