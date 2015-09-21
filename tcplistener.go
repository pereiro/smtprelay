package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"io"
	"io/ioutil"
	"net"
	"strings"
	"time"
)

const (
	METADATA_LENGTH_BYTES = 4
)

var (
	TCPHandlersLimiter    chan int
	TCPConnectionsLimiter chan int
	TCPListenerStarted    bool
)

func StartTCPServer() {

	l, err := net.Listen("tcp", conf.ListenTCPPort)
	if err != nil {
		log.Critical("can't start TCP listener:%s", err.Error())
		panic(err.Error())
	}
	defer l.Close()
	TCPHandlersLimiter = make(chan int, conf.TCPMaxHandlers)
	TCPConnectionsLimiter = make(chan int, conf.TCPMaxConnections)
	TCPListenerStarted = true
	log.Info("SYSTEM: Started TCP listener at  " + conf.ListenTCPPort)
	for TCPListenerStarted {
		conn, err := l.Accept()
		if err != nil {
			log.Error("Error accepting tcp connection: ", err.Error())
			continue
		}
		log.Debug("connection accepted from %s", conn.RemoteAddr().String())
		TCPConnectionsLimiter <- 0
		go tcpHandler(conn)
	}
	return
}

func StopTCPListener() {
	log.Info("SYSTEM: Stopping TCP listener")
	TCPListenerStarted = false
	if len(TCPHandlersLimiter) > 0 {
		log.Info("Waiting for processing existing %d tcp connections", len(TCPHandlersLimiter))
		for len(TCPHandlersLimiter) > 0 {
			time.Sleep(time.Millisecond * 100)
		}
	}
	log.Info("SYSTEM: TCP listener stopped")
}

func writeErrorResponse(conn net.Conn, arg0 string, args ...interface{}) {
	log.Error(arg0, args...)
	conn.Write([]byte(fmt.Sprintf(arg0, args...)))
	if err := conn.Close(); err != nil {
		log.Error("error close connection (error response): %s", err.Error())
	}
	<-TCPConnectionsLimiter
}

func writeSuccessResponse(conn net.Conn) {
	log.Debug("success response to %s", conn.RemoteAddr().String())
	conn.Write([]byte("OK"))
	if err := conn.Close(); err != nil {
		log.Error("error close connection (success response): %s", err.Error())
	}
	<-TCPConnectionsLimiter
}

func closeNoResponse(conn net.Conn) {
	log.Debug("close no response from %s", conn.RemoteAddr().String())
	conn.Close()
	<-TCPConnectionsLimiter
}

func readMetaData(conn net.Conn) (payloadSize int64, err error) {
	metadata := make([]byte, METADATA_LENGTH_BYTES)
	_, err = conn.Read(metadata)
	if err != nil {
		return
	}
	log.Debug("metadata dump from %s :%v", conn.RemoteAddr().String(), metadata)
	buf := proto.NewBuffer(metadata)
	uintSize, err := buf.DecodeFixed32()
	if err != nil {
		return 0, err
	}
	payloadSize = int64(uintSize)
	if payloadSize < 0 {
		return payloadSize, errors.New(fmt.Sprintf("payload length is negative %s: %d", conn.RemoteAddr().String(), payloadSize))
	}
	return payloadSize, nil
}

func readPayload(conn net.Conn, payloadSize int64) (payload []byte, err error) {
	log.Debug("expected payload size from %s :%d", conn.RemoteAddr().String(), payloadSize)
	payload = make([]byte, payloadSize)
	reader := bufio.NewReader(conn)
	n, err := io.ReadFull(reader, payload)
	if err != nil {
		return
	}
	log.Debug("summary: read %d bytes from tcp conn %s", n, conn.RemoteAddr().String())
	log.Debug("actual payload size from %s:%d", conn.RemoteAddr().String(), len(payload))

	return payload, nil
}

func uncompressPayload(payload []byte) (unzippedPayload []byte, err error) {

	reader := bytes.NewReader(payload)
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return
	}
	defer gzipReader.Close()
	unzippedPayload, err = ioutil.ReadAll(gzipReader)
	if err != nil {
		return
	}
	log.Debug("unzipped data size: %d", len(unzippedPayload))
	return
}

func tcpHandler(conn net.Conn) {
	TCPHandlersLimiter <- 0
	log.Debug("Handler started for %s", conn.RemoteAddr().String())

	conn.SetDeadline(time.Now().Add(time.Second * time.Duration(conf.TCPTimeoutSeconds)))
	defer func() {
		log.Debug("Handler released for %s", conn.RemoteAddr().String())
		<-TCPHandlersLimiter
	}()

	payloadSize, err := readMetaData(conn)
	if err != nil {
		if err == io.EOF {
			closeNoResponse(conn)
			return
		}
		writeErrorResponse(conn, "error reading metadata from %s: %s", conn.RemoteAddr().String(), err.Error())
		return
	}

	compressedPayload, err := readPayload(conn, payloadSize)
	if err != nil {
		writeErrorResponse(conn, "error reading payload data from %s: %s", conn.RemoteAddr().String(), err.Error())
		return
	}
	log.Debug("uncompressing payload from %s", conn.RemoteAddr().String())
	payload, err := uncompressPayload(compressedPayload)
	if err != nil {
		writeErrorResponse(conn, "error uncompressing payload from %s: %s", conn.RemoteAddr().String(), err.Error())
		return
	}

	//	err =  ioutil.WriteFile("test.txt",payload,0777)
	//	if err != nil {
	//		log.Error("error writing to file",err.Error())
	//		return
	//	}

	writeSuccessResponse(conn)

	log.Debug("unmarshalling payload from %s", conn.RemoteAddr().String())

	packet := &EmailMessageWithByteArrayPacket{}
	err = proto.Unmarshal(payload, packet)
	if err != nil {
		log.Error("error deserializing email packet from %s: %s", conn.RemoteAddr().String(), err.Error())
		return
	}

	log.Debug("Messages deserialized from %s: %d", conn.RemoteAddr().String(), len(packet.GetMessages()))

	for _, email := range packet.Messages {

		var entry QueueEntry
		entry.Data = email.GetEmlData()
		entry.Recipients = email.GetRecipients()
		entry.Sender = email.GetSender()

		msg, err := ParseMessage(entry.Recipients, entry.Sender, entry.Data)
		if err != nil {
			var rcpt = strings.Join(entry.Recipients, ";")
			log.Error("msg %s from %s (sender:%s;rcpt:%s) - %s DROPPED: %s", email.GetMessageId(), conn.RemoteAddr().String(), entry.Sender, rcpt, err.Error(), ErrMessageError.Error())
			continue
		}

		log.Info("msg %s from %s RECEIVED", msg.String(), conn.RemoteAddr().String())

		if len(entry.Recipients) > conf.MaxRecipients || len(entry.Recipients) == 0 {
			log.Error("message %s rcpt count limited to %d, DROPPED: %s", msg.String(), conf.MaxRecipients, ErrTooManyRecipients.Error())
			continue
		}

		var entries []QueueEntry

		for domain, _ := range msg.RcptDomains {

			mailServer, err := lookupMailServer(strings.ToLower(domain), 0)
			if err != nil {
				log.Error("message %s can't get MX record for %s - %s, DROPPED: %s", msg.String(), domain, err.Error(), ErrDomainNotFound.Error())
				continue
			}

			if conf.RelayModeEnabled {
				mailServer = conf.RelayServer
			}

			entries = append(entries, QueueEntry{MailServer: mailServer,
				Sender:          entry.Sender,
				Recipients:      msg.GetDomainRecipientList(domain),
				Data:            entry.Data,
				SenderDomain:    msg.Sender.Domain,
				RecipientDomain: domain,
				MessageId:       msg.MessageId})
		}
		for _, entry := range entries {
			PushMail(entry)
		}

	}

	return
}
