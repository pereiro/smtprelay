package main

import (
	"flag"
	"smtprelay/smtpd"
	"runtime"
	"strings"
)

var (
	conf *Conf
)

func main() {

	var logConfigFile = flag.String("logconfig", "/usr/local/etc/smtprelay/logconfig.xml", "enter path to file with log settings. Default:/usr/local/etc/smtprelay/logconfig.xml")
	var mainConfigFile = flag.String("config", "/usr/local/etc/smtprelay/config.json", "enter path to config file. Default:/usr/local/etc/smtprelay/config.json")

	flag.Parse()

	InitLogger(*logConfigFile)
	log.Info("Starting..")
	conf = new(Conf)
	if err := conf.Load(*mainConfigFile); err != nil {
		log.Critical("can't load config,shut down:", err.Error())
		panic(err.Error())
	}
	runtime.GOMAXPROCS(conf.NumCPU)

	if err := InitQueues(conf.QueueFile); err != nil {
		log.Critical("can't init redis MQ", err.Error())
		panic(err.Error())
	}
	log.Info("MQ initialized")
	go StartStatisticServer()
	go StartSender()

	if conf.DKIMEnabled {
		log.Info("DKIM enabled, loading keys..")
		err := DKIMLoadKeyRepository()
		if err != nil {
			log.Critical("Can't load DKIM repo:%s", err.Error())
			panic(err.Error())
		}
	} else {
		log.Info("DKIM disabled")
	}

	if conf.RelayModeEnabled {
		log.Info("TEST MODE ENABLED!! All messages will be redirected to %s", conf.RelayServer)
	}
	log.Info("Incoming connections limit - %d", conf.MaxIncomingConnections)
	log.Info("Outcoming connections limit - %d", conf.MaxOutcomingConnections)

	server := &smtpd.Server{
		Hostname:       conf.ServerHostName,
		WelcomeMessage: conf.WelcomeMessage,
		MaxConnections: conf.MaxIncomingConnections,
		Handler:        handlerPanicProcessor(handler),
	}

	log.Info("SMTP Relay started at %s", conf.ListenPort)

	work := func() {
		server.ListenAndServe(conf.ListenPort)
		/*        defer func() {
		          if r:=recover();r!=nil{
		              log.Critical("recovered from PANIC:%s",r)
		          }
		      }()*/
	}

	for {
		work()
	}

}

func handlerPanicProcessor(handler func(peer smtpd.Peer, env smtpd.Envelope) error) func(peer smtpd.Peer, env smtpd.Envelope) error {
	return func(peer smtpd.Peer, env smtpd.Envelope) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Critical("PANIC, message DROPPED:%s", r)
				err = ErrMessageErrorUnknown
			}
		}()
		return handler(peer, env)
	}
}

func handler(peer smtpd.Peer, env smtpd.Envelope) error {
	msg, err := ParseMessage(env.Recipients, env.Sender, env.Data)
	if err != nil {
		var rcpt = strings.Join(env.Recipients, ";")
		log.Error("incorrect msg DROPPED from %s (sender:%s;rcpt:%s) - %s: %s", peer.Addr.String(), env.Sender, rcpt, err.Error(), ErrMessageError.Error())
		return ErrMessageError
	}

	log.Info("msg %s RECEIVED from %s", msg.String(), peer.Addr.String())

	if len(env.Recipients) > conf.MaxRecipients || len(env.Recipients) == 0 {
		log.Error("message %s DROPPED, rcpt count limited to %d: %s", msg.String(), conf.MaxRecipients, ErrTooManyRecipients.Error())
		return ErrTooManyRecipients
	}

	var entries []QueueEntry

	for domain, _ := range msg.RcptDomains {

		mailServer, err := lookupMailServer(strings.ToLower(domain))
		if err != nil {
			log.Error("message %s DROPPED, can't get MX record for %s - %s: %s", msg.String(), domain, err.Error(), ErrDomainNotFound.Error())
			return ErrDomainNotFound
		}

		if conf.RelayModeEnabled {
			mailServer = conf.RelayServer
		}

		entries = append(entries, QueueEntry{MailServer: mailServer,
			Sender:       env.Sender,
			Recipients:   msg.GetDomainRecipientList(domain),
			Data:         env.Data,
			SenderDomain: msg.Sender.Domain,
			MessageId:    msg.MessageId})
	}
	for _, entry := range entries {
		select {
		case MailMQChannel <- entry:
		default:
			err = PutMail(entry)
			if err != nil {
				log.Error("msg %s DROPPED, MQ error - %s: %s", msg.String(), err.Error(), ErrServerError.Error())
				return ErrServerError
			}
			log.Info("msg %s QUEUED", msg.String())
		}
	}
	return nil

}
