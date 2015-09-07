// Copyright (C) 2010, Kyle Lemons <kyle@kylelemons.net>.  All rights reserved.

package log4go

import (
	"fmt"
	"net"
	"os"
	"syscall"
)

// This log writer sends output to a socket
type SocketLogWriter chan *LogRecord

// This is the SocketLogWriter's output method
func (w SocketLogWriter) LogWrite(rec *LogRecord) {
	w <- rec
}

func (w SocketLogWriter) Close() {
	close(w)
}

func NewSocketLogWriter(proto, hostport string) SocketLogWriter {
	sock, err := net.Dial(proto, hostport)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewSocketLogWriter(%q): %s\n", hostport, err)
		return nil
	}

	w := SocketLogWriter(make(chan *LogRecord, LogBufferLength))

	go func() {
		defer func() {
			if sock != nil && proto == "tcp" {
				sock.Close()
			}
		}()

		for rec := range w {
			// Marshall into JSON
			js := FormatSyslogRecord(rec)
			_, err = sock.Write([]byte(js))
			if err != nil {
				if err == syscall.EPIPE {
					fmt.Fprintf(os.Stderr, "SMTPRELAY: Broken pipe detected - reconnect")
					sock, err = net.Dial(proto, hostport)
					if err != nil {
						fmt.Fprintf(os.Stderr, "SMTPRELAY: Reconnection error (%s): %s\n", hostport, err)
						continue
					}
				} else {
					fmt.Fprint(os.Stderr, "SocketLogWriter(%s): %s", hostport, err)
					continue
				}
			}
		}
	}()

	return w
}

func FormatSyslogRecord(rec *LogRecord) string {
	return FormatLogRecord("%L  %M", rec)
}
