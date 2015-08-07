package main

import (
	"errors"
	"net"
	"smtprelay/smtpd"
	"sync"
	"time"
)

var (
	StoppedError = errors.New("Listener stopped")
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
				log.Info("SMTP listener stopped")
			}

		}
	}()
	return
}

func (server *StoppableSMTPServer) Stop() {
	log.Info("Stopping SMTP listener")
	server.Listener.Stop()
	log.Info("Waiting for processing existing incoming SMTP connections")
	server.WaitGroup.Wait()
	log.Info("SMTP server stopped")
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
		//Wait up to one second for a new connection
		sl.SetDeadline(time.Now().Add(time.Second))

		newConn, err := sl.TCPListener.Accept()

		//Check for the channel being closed
		select {
		case <-sl.stop:
			return nil, StoppedError
		default:
			//If the channel is still open, continue as normal
		}

		if err != nil {
			netErr, ok := err.(net.Error)

			//If this is a timeout, then continue to wait for
			//new connections
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
