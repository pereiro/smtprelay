package main

import (
	"errors"
	"smtprelay/smtp"
	"time"
)

var (
	SenderLimiter chan interface{}
)

func StartSender() {
	SenderLimiter = make(chan interface{}, conf.MaxOutcomingConnections)
	go CloneMailers()
	go StartErrorHandler()
}

func StartErrorHandler() {
	for {
		entry := ExtractError()
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
	data := entry.Data
	signed := ""
	if conf.DKIMEnabled {
		signedData, err := DKIMSign(data, entry.SenderDomain)
		if err != nil {
			signed = "(NOT SIGNED)"
		} else {
			data = signedData
		}
	}

	if conf.RelayModeEnabled {
		entry.MailServers = []string{conf.RelayServer}
	}
	err := sendMailToServers(entry, data, conf.ServerHostName)
	if err == nil {
		log.Info("msg %s SENT%s: %s", entry.String(), signed, ErrStatusSuccess.Error())
		MailSentIncreaseCounter(1)
		return
	}

	smtpError := ParseOutcomingError(err.Error())
	if smtpError.Code/100 == 5 {
		log.Error("msg %s DROPPED: %s", entry.String(), smtpError.Error())
		MailDroppedIncreaseCounter(1)
		return
	}
	entry.ErrorCount += 1
	entry.Error = smtpError
	if entry.ErrorCount >= conf.DeferredMailMaxErrors {
		log.Error("msg %s DEFER LIMIT=(%d/%d) DROPPED: %s", entry.String(), entry.ErrorCount, conf.DeferredMailMaxErrors, smtpError.Error())
		MailDroppedIncreaseCounter(1)
		return
	}
	entry.QueueTime = time.Now()
	entry.UnqueueTime = entry.QueueTime.Add(time.Duration(conf.DeferredMailDelay) * time.Second)
	mailServers, err := lookupDeliveryServers(entry.RecipientDomain)
	if err == nil {
		entry.MailServers = mailServers
	} else {
		log.Warn("msg %s can't refresh MX records, previous list will be used: %s", entry.String(), err)
	}
	PushError(entry)
	log.Error("msg %s (%d/%d) (next attempt at %s ) DEFERRED: %s", entry.String(), entry.ErrorCount, conf.DeferredMailMaxErrors, entry.UnqueueTime, smtpError.Error())
}

func sendMailToServers(entry QueueEntry, data []byte, host string) error {
	err := errors.New("no mail servers available")
	for _, server := range entry.MailServers {
		err = smtp.SendMail(server, nil, entry.Sender, entry.Recipients, data, host)
		if err == nil || ParseOutcomingError(err.Error()).Code/100 == 5 {
			return err
		}
		log.Warn("msg %s delivery via %s failed: %s", entry.String(), server, err)
	}
	return err
}
