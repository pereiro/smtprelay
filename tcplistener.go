package main

import (
	"net"
	"bytes"
	"encoding/binary"
	"github.com/golang/protobuf/proto"
	"fmt"
	"strings"
	"time"
)

var (
	TCPLimiter chan int
	TCPListenerStarted bool

)

func StartTCPServer() {

	l, err := net.Listen("tcp", conf.ListenTCPPort)
	if err != nil {
					log.Critical("can't start TCP listener:%s", err.Error())
					panic(err.Error())
	}
	defer l.Close()
	TCPLimiter = make(chan int,conf.TCPMaxConnections)
	TCPListenerStarted = true
	log.Info("SYSTEM: Started TCP listener at  " + conf.ListenTCPPort)
	for (TCPListenerStarted) {
		conn, err := l.Accept()
		log.Info("connection accepted from %s",conn.RemoteAddr().String())
		if err != nil {
			log.Error("Error accepting tcp connection: ", err.Error())
			continue
		}
		TCPLimiter <- 0
		go tcpHandler(conn)
	}
	return
}

func StopTCPListener(){
	log.Info("SYSTEM: Stopping TCP listener")
	TCPListenerStarted = false;
	if len(TCPLimiter)>0{
		log.Info("Waiting for processing existing %d tcp connections",len(TCPLimiter))
		for(len(TCPLimiter)>0){
			time.Sleep(time.Millisecond*100)
		}
	}
	log.Info("SYSTEM: TCP listener stopped")
}

func tcpHandler(conn net.Conn){
	log.Debug("Handler started for %s",conn.RemoteAddr().String())

	defer conn.Close()
	metadata := make ([]byte, 8)
	_,err := conn.Read(metadata)
	if err != nil {
		log.Error("error reading metadata frame from %s:%s",conn.RemoteAddr().String(),err.Error())
		conn.Write([]byte(err.Error()))
		return
	}
	reader := bytes.NewReader(metadata)
	log.Debug("metadata dump:%v",metadata)
	var payloadSize int64
	err = binary.Read(reader,binary.LittleEndian,&payloadSize)
	if err != nil {
		log.Error("error converting byte array: %s",err.Error())
	}
	log.Debug("payload size:%d",payloadSize)
	payload := make([]byte,0)
	log.Debug(len(payload))
	var n int
	var sum int64
	for(sum<payloadSize) {
		tempPayload := make ([]byte,payloadSize)
		n, err = conn.Read(tempPayload)
		if err != nil {
			log.Error("error reading data from %s:%s", conn.RemoteAddr().String(), err.Error())
			conn.Write([]byte(err.Error()))
			return
		}
		payload = append(payload,tempPayload[:n]...)
		sum = sum + int64(n)
	}

	log.Debug("read %d bytes from tcp conn",sum)

	packet := &EmailMessageWithByteArrayPacket{}
	err = proto.Unmarshal(payload,packet)
	if err != nil {
		log.Error("error deserilazing email packet from %s:%s",conn.RemoteAddr().String(),err.Error())
		conn.Write([]byte(fmt.Sprintf("error deserilazing email packet from %s:%s",conn.RemoteAddr().String(),err.Error())))
		return
	}

	for _,email:=range packet.Messages {

		var entry QueueEntry
		entry.Data = email.GetEmlData()
		entry.Recipients = email.GetRecipients()
		entry.Sender = email.GetSender()

		//log.Info("Data received from %s:%v", conn.RemoteAddr().String(), string(entry.Data))

		msg, err := ParseMessage(entry.Recipients, entry.Sender, entry.Data)
		if err != nil {
			var rcpt = strings.Join(entry.Recipients, ";")
			log.Error("incorrect msg from %s (sender:%s;rcpt:%s) - %s DROPPED: %s", conn.RemoteAddr().String(), entry.Sender, rcpt, err.Error(), ErrMessageError.Error())
			conn.Write([]byte(email.GetMessageId()+" "+ErrMessageError.Error()))
			return
		}

		log.Info("msg %s from %s TCP RECEIVED", msg.String(), conn.RemoteAddr().String())

		if len(entry.Recipients) > conf.MaxRecipients || len(entry.Recipients) == 0 {
			log.Error("message %s rcpt count limited to %d, DROPPED: %s", msg.String(), conf.MaxRecipients, ErrTooManyRecipients.Error())
			conn.Write([]byte(email.GetMessageId()+" "+ErrTooManyRecipients.Error()))
			return
		}

		var entries []QueueEntry

		for domain, _ := range msg.RcptDomains {

			mailServer, err := lookupMailServer(strings.ToLower(domain), 0)
			if err != nil {
				log.Error("message %s can't get MX record for %s - %s, DROPPED: %s", msg.String(), domain, err.Error(), ErrDomainNotFound.Error())
				conn.Write([]byte(email.GetMessageId()+" "+ErrDomainNotFound.Error()))
				return
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
		conn.Write([]byte(email.GetMessageId()+" "+ErrStatusSuccess.Error()))

	}
	<-TCPLimiter
	return
}