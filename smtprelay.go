package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"smtprelay/smtpd"
	"strings"
)

var (
	conf *Conf
	//IncomingLimiter chan interface{}
)

const (
	MAINCONFIGFILENAME = "config.json"
	LOGCONFIGFILENAME  = "logconfig.xml"
)

type Flags struct {
	MainConfigFilePath string
	LogConfigFilePath  string
}

func main() {
	flags := GetFlags()

	InitLogger(flags.LogConfigFilePath)

	log.Info("Starting..")
	conf = new(Conf)
	if err := conf.Load(flags.MainConfigFilePath); err != nil {
		log.Critical("can't load config,shut down:", err.Error())
		panic(err.Error())
	}

	runtime.GOMAXPROCS(conf.NumCPU)

	if err := InitQueues(conf.QueueFile); err != nil {
		log.Critical("can't init redis MQ", err.Error())
		panic(err.Error())
	}
	defer CloseQueues()

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
	//IncomingLimiter = make(chan interface{},conf.MaxIncomingConnections)

	server := &smtpd.Server{
		Hostname:       conf.ServerHostName,
		WelcomeMessage: conf.WelcomeMessage,
		MaxConnections: conf.MaxIncomingConnections,
		Handler:        handlerPanicProcessor(handler),
	}

	log.Info("Incoming connections limit - %d", conf.MaxIncomingConnections)
	log.Info("Outcoming connections limit - %d", conf.MaxOutcomingConnections)

	log.Info("SMTP Relay started at %s", conf.ListenPort)

	work := func() {
		server.ListenAndServe(conf.ListenPort)
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
		case MailDirectChannel <- entry:
		default:
			go func() {
				MailHandlersIncreaseCounter(1)
				defer MailHandlersDecreaseCounter(1)
				err = PutMail(entry)
				if err != nil {
					log.Error("msg %s DROPPED, MQ error - %s: %s", msg.String(), err.Error(), ErrServerError.Error())
					return
				}
				log.Info("msg %s QUEUED", msg.String())
			}()
		}
	}
	return nil

}

func GetFlags() (flags Flags) {
	var workDir = flag.String("workdir", "/usr/local/etc/smtprelay/", "Enter path to workdir. Default:/usr/local/etc/smtprelay/")
	flag.Parse()
	flags.MainConfigFilePath = *workDir + string(os.PathSeparator) + MAINCONFIGFILENAME
	flags.LogConfigFilePath = *workDir + string(os.PathSeparator) + LOGCONFIGFILENAME
	if _, err := os.Stat(flags.MainConfigFilePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "File %s not found.Please specify correct workdir", flags.MainConfigFilePath)
		os.Exit(1)
	}
	if _, err := os.Stat(flags.LogConfigFilePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "File %s not found.Please specify correct workdir", flags.LogConfigFilePath)
		os.Exit(1)
	}
	return flags
}
