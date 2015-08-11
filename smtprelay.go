package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"smtprelay/smtpd"
	"strings"
	"syscall"
	"time"
)

var (
	conf       *Conf
	flags      Flags
	smtpServer StoppableSMTPServer
	EXIT       chan int
)

const (
	MAINCONFIGFILENAME = "config.json"
	LOGCONFIGFILENAME  = "logconfig.xml"
)

type Flags struct {
	MainConfigFilePath string
	LogConfigFilePath  string
}

func ReloadConfig(filename string) {
	log.Info("Reloading config file")
	newConf := new(Conf)
	if err := newConf.Load(flags.MainConfigFilePath); err != nil {
		log.Critical("can't reload config, old settings will be used:", err.Error())
	}
	conf = newConf
	runtime.GOMAXPROCS(conf.NumCPU)
}

func main() {
	EXIT = make(chan int)
	flags = GetFlags()

	InitLogger(flags.LogConfigFilePath)

	log.Info("SYSTEM: Starting..")
	conf = new(Conf)
	if err := conf.Load(flags.MainConfigFilePath); err != nil {
		log.Critical("can't load config,shut down:", err.Error())
		panic(err.Error())
	}

	runtime.GOMAXPROCS(conf.NumCPU)

	if err := InitQueues(); err != nil {
		log.Critical("can't init MQ", err.Error())
		panic(err.Error())
	}

	log.Info("SYSTEM: MQ initialized")
	go StartStatisticServer()
	go StartSender()

	if conf.DKIMEnabled {
		log.Info("SYSTEM: DKIM enabled, loading keys..")
		err := DKIMLoadKeyRepository()
		if err != nil {
			log.Critical("Can't load DKIM repo:%s", err.Error())
			panic(err.Error())
		}
	} else {
		log.Info("DKIM disabled")
	}

	if conf.RelayModeEnabled {
		log.Info("SYSTEM: Relay mode enabled! All messages will be redirected to %s", conf.RelayServer)
	}

	log.Info("SYSTEM: Incoming connections limit - %d", conf.MaxIncomingConnections)
	log.Info("SYSTEM: Outcoming connections limit - %d", conf.MaxOutcomingConnections)

	//smtpServer = new(StoppableSMTPServer)
	smtpServer.Hostname = conf.ServerHostName
	smtpServer.WelcomeMessage = conf.WelcomeMessage
	smtpServer.MaxConnections = conf.MaxIncomingConnections
	smtpServer.Handler = handlerPanicProcessor(handler)

	go StartSignalListener()

	log.Info("SYSTEM: SMTP Relay started at %s", conf.ListenPort)

	err := smtpServer.Start()
	if err != nil {
		panic(err.Error())
	}

	<-EXIT
}

func handlerPanicProcessor(handler func(peer smtpd.Peer, env smtpd.Envelope) error) func(peer smtpd.Peer, env smtpd.Envelope) error {
	return func(peer smtpd.Peer, env smtpd.Envelope) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Critical("PANIC, message DROPPED: %s", r)
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
		log.Error("incorrect msg from %s (sender:%s;rcpt:%s) - %s DROPPED: %s", peer.Addr.String(), env.Sender, rcpt, err.Error(), ErrMessageError.Error())
		return ErrMessageError
	}

	log.Info("msg %s from %s RECEIVED", msg.String(), peer.Addr.String())

	if len(env.Recipients) > conf.MaxRecipients || len(env.Recipients) == 0 {
		log.Error("message %s rcpt count limited to %d, DROPPED: %s", msg.String(), conf.MaxRecipients, ErrTooManyRecipients.Error())
		return ErrTooManyRecipients
	}

	var entries []QueueEntry

	for domain, _ := range msg.RcptDomains {

		mailServer, err := lookupMailServer(strings.ToLower(domain), 0)
		if err != nil {
			log.Error("message %s can't get MX record for %s - %s, DROPPED: %s", msg.String(), domain, err.Error(), ErrDomainNotFound.Error())
			return ErrDomainNotFound
		}

		if conf.RelayModeEnabled {
			mailServer = conf.RelayServer
		}

		entries = append(entries, QueueEntry{MailServer: mailServer,
			Sender:          env.Sender,
			Recipients:      msg.GetDomainRecipientList(domain),
			Data:            env.Data,
			SenderDomain:    msg.Sender.Domain,
			RecipientDomain: domain,
			MessageId:       msg.MessageId})
	}
	for _, entry := range entries {
		PushMail(entry)
	}
	return nil

}

func GetFlags() (flags Flags) {
	var workDir = flag.String("workdir", "/usr/local/etc/smtprelay", "Enter path to workdir. Default:/usr/local/etc/smtprelay")
	flag.Parse()
	flags.MainConfigFilePath = *workDir + string(os.PathSeparator) + MAINCONFIGFILENAME
	flags.LogConfigFilePath = *workDir + string(os.PathSeparator) + LOGCONFIGFILENAME
	if _, err := os.Stat(flags.MainConfigFilePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "File %s not found.Please specify correct workdir\n", flags.MainConfigFilePath)
		os.Exit(1)
	}
	if _, err := os.Stat(flags.LogConfigFilePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "File %s not found.Please specify correct workdir\n", flags.LogConfigFilePath)
		os.Exit(1)
	}
	return flags
}

func StartSignalListener() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)
	for {
		var signal = <-c
		log.Info("SYSTEM: SIGNAL %s RECEIVED", signal.String())
		switch signal {
		case syscall.SIGUSR1:
			ReloadConfig(flags.MainConfigFilePath)
		case syscall.SIGUSR2:
			FlushQueues()
		case syscall.SIGINT:
			GracefullyStop()
		case syscall.SIGKILL:
			GracefullyStop()
		case syscall.SIGTERM:
			GracefullyStop()
		case syscall.SIGHUP:
			GracefullyStop()
		}

	}
}

func GracefullyStop() {
	smtpServer.Stop()
	log.Info("SYSTEM: Waiting for processing existing outcoming SMTP connections and queued messages (%d in all queues)", GetMailQueueLength()+GetErrorQueueLength())
	for GetMailQueueLength()+GetErrorQueueLength() > 0 {
		FlushErrors()
		time.Sleep(1 * time.Second)
		log.Info("SYSTEM: Messages left in queues - %d (mails - %d;errors - %d)", GetMailQueueLength()+GetErrorQueueLength(), GetMailQueueLength(), GetErrorQueueLength())
	}
	time.Sleep(200 * time.Millisecond)
	log.Info("SYSTEM: Smtprelay stopped")
	time.Sleep(200 * time.Millisecond)
	EXIT <- 1
}

func FlushQueues() {
	log.Info("SYSTEM: Start flushing queues")
	FlushErrors()
}
