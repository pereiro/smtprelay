package main

import (
	"errors"
	"net"
	"smtprelay/smtpd"
	"strings"
	"sync"
	"time"
)

var (
	StoppedError = errors.New("Listener stopped")
	smtpServer   StoppableSMTPServer
)

type StoppableListener struct {
	*net.TCPListener
	stop chan int
}

type StoppableSMTPServer struct {
	smtpd.Server
	OriginalListener net.Listener
	Listener         *StoppableListener
	WaitGroup        sync.WaitGroup
}

func (server *StoppableSMTPServer) Start() (err error) {

	server.OriginalListener, err = net.Listen("tcp", conf.ListenPort)
	if err != nil {
		return
	}

	server.Listener, err = NewStoppableListener(server.OriginalListener)
	if err != nil {
		return
	}

	go func() {
		server.WaitGroup.Add(1)
		defer server.WaitGroup.Done()
		err = server.Serve(server.Listener)
		if err != nil {
			if err != StoppedError {
				log.Critical("Error while start SMTP server at port %s:%s", conf.ListenPort, err.Error())
				return
			} else {
				log.Info("SYSTEM: SMTP listener stopped")
			}

		}
	}()
	return
}

func (server *StoppableSMTPServer) Stop() {
	log.Info("SYSTEM: Stopping SMTP listener")
	server.Listener.Stop()
	log.Info("SYSTEM: Waiting for processing existing incoming SMTP connections")
	server.WaitGroup.Wait()
	log.Info("SYSTEM: SMTP server stopped")
}

func NewStoppableListener(l net.Listener) (*StoppableListener, error) {
	tcpL, ok := l.(*net.TCPListener)

	if !ok {
		return nil, errors.New("Cannot wrap listener")
	}

	retval := &StoppableListener{}
	retval.TCPListener = tcpL
	retval.stop = make(chan int)

	return retval, nil
}

func (sl *StoppableListener) Accept() (net.Conn, error) {

	for {
		sl.SetDeadline(time.Now().Add(time.Second))
		newConn, err := sl.TCPListener.Accept()
		select {
		case <-sl.stop:
			return nil, StoppedError
		default:
		}

		if err != nil {
			netErr, ok := err.(net.Error)
			if ok && netErr.Timeout() && netErr.Temporary() {
				continue
			}
		}

		return newConn, err
	}
}
func (sl *StoppableListener) Stop() {
	close(sl.stop)
}

func smtpHandler(peer smtpd.Peer, env smtpd.Envelope) error {
	MailHandlersIncreaseCounter(1)
	defer MailHandlersDecreaseCounter(1)
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

func StartSMTPServer() {

	smtpServer.Hostname = conf.ServerHostName
	smtpServer.WelcomeMessage = conf.WelcomeMessage
	smtpServer.MaxConnections = conf.MaxIncomingConnections
	smtpServer.Handler = handlerPanicProcessor(smtpHandler)

	log.Info("SYSTEM: SMTP Relay started at %s", conf.ListenPort)

	err := smtpServer.Start()
	if err != nil {
		panic(err.Error())
	}
}

func StopSMTPServer() {
	smtpServer.Stop()
}
