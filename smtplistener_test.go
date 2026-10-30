package main

import (
	"errors"
	"net"
	"runtime"
	"testing"
	"time"
)

func TestStoppableSMTPServerImmediateStop(t *testing.T) {
	previousConf := conf
	conf = &Conf{ListenPort: "127.0.0.1:0"}
	t.Cleanup(func() { conf = previousConf })

	// Exercise Stop before the serving goroutine has a chance to start.
	previousProcs := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousProcs) })

	server := new(StoppableSMTPServer)
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		server.OriginalListener.Close()
		server.WaitGroup.Wait()
	})

	server.Stop()
	if err := server.Listener.TCPListener.SetDeadline(time.Now()); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Stop returned before the listener closed: %v", err)
	}
}
