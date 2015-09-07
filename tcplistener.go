package main

import (
	"net"
	"bytes"
	"encoding/binary"
	"github.com/golang/protobuf/proto"
	"fmt"
	"time"
	"strings"
	"errors"
	"bufio"
	"io"
	"io/ioutil"
)

const (
	METADATA_LENGTH_BYTES = 8
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
		if err != nil {
			log.Error("Error accepting tcp connection: ", err.Error())
			continue
		}
		log.Info("connection accepted from %s",conn.RemoteAddr().String())
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

func writeErrorResponse(conn net.Conn,arg0 string,args ...interface{} ){
	log.Error(arg0,args)
	conn.Write([]byte(fmt.Sprintf(arg0,args)))
}


func readMetaData(conn net.Conn) (payloadSize int64,err error){
	metadata := make ([]byte, METADATA_LENGTH_BYTES)
	_,err = conn.Read(metadata)
	if err != nil {
		return
	}
	reader := bytes.NewReader(metadata)
	log.Debug("metadata dump:%v",metadata)
	err = binary.Read(reader,binary.LittleEndian,&payloadSize)
	if err != nil {
		return
	}
	if (payloadSize<0){
		return payloadSize,errors.New(fmt.Sprintf("payload length is negative: %d",conn.RemoteAddr().String(),payloadSize))
	}
	return payloadSize,nil
}

func readPayload(conn net.Conn,payloadSize int64 ) (payload []byte,err error) {
	log.Debug("expected payload size:%d",payloadSize)
	payload = make([]byte,0)
	var sum int64
	for(sum<payloadSize) {
		tempPayload := make ([]byte,payloadSize)
		n, err := conn.Read(tempPayload)
		log.Debug("read %d bytes from tcp conn",n)
		if err != nil {
			return payload,err
		}
		payload = append(payload,tempPayload[:n]...)
		sum = sum + int64(n)
	}
	log.Debug("summary: read %d bytes from tcp conn",sum)
	log.Debug("actual payload size:%d",len(payload))

	return payload,nil
}

func readPayload2 (conn net.Conn,payloadSize int64 ) (payload []byte,err error) {
	log.Debug("expected payload size:%d",payloadSize)
	payload = make([]byte,payloadSize)
	reader := bufio.NewReader(conn)
	n,err := io.ReadFull(reader,payload)
	if err != nil {
		return
	}
	err = ioutil.WriteFile("dump1.txt",payload,0777)
	if err != nil {
		log.Error("error writing file: %s",err.Error())
	}
	log.Debug("summary: read %d bytes from tcp conn",n)
	log.Debug("actual payload size:%d",len(payload))

	return payload,nil
}

func tcpHandler(conn net.Conn){
	log.Debug("Handler started for %s",conn.RemoteAddr().String())

	defer conn.Close()

	payloadSize,err := readMetaData(conn)
	if err != nil {
		writeErrorResponse(conn,"error reading metadata from %s: %s", conn.RemoteAddr().String(), err.Error())
		return
	}
	payload,err := readPayload2(conn,payloadSize)
	if err != nil {
		writeErrorResponse(conn,"error reading payload data from %s: %s", conn.RemoteAddr().String(), err.Error())
		return
	}

	packet := &EmailMessageWithByteArrayPacket{}
	err = proto.Unmarshal(payload,packet)
	if err != nil {
		writeErrorResponse(conn,"error deserializing email packet from %s: %s",conn.RemoteAddr().String(),err.Error())
		return
	}

	log.Debug("Messages deserialized: %d",len(packet.GetMessages()))

	for _,email:=range packet.Messages {

		var entry QueueEntry
		entry.Data = email.GetEmlData()
		entry.Recipients = email.GetRecipients()
		entry.Sender = email.GetSender()

		//log.Info("Data received from %s:%v", conn.RemoteAddr().String(), string(entry.Data))

		msg, err := ParseMessage(entry.Recipients, entry.Sender, entry.Data)
		if err != nil {
			var rcpt = strings.Join(entry.Recipients, ";")
			writeErrorResponse(conn,"incorrect msg %s from %s (sender:%s;rcpt:%s) - %s DROPPED: %s",email.GetMessageId(),conn.RemoteAddr().String(), entry.Sender, rcpt, err.Error(), ErrMessageError.Error())
			continue
		}

		//log.Info("msg %s from %s TCP RECEIVED", msg.String(), conn.RemoteAddr().String())

		if len(entry.Recipients) > conf.MaxRecipients || len(entry.Recipients) == 0 {
			writeErrorResponse(conn,"message %s rcpt count limited to %d, DROPPED: %s", msg.String(), conf.MaxRecipients, ErrTooManyRecipients.Error())
			continue
		}

		var entries []QueueEntry

		for domain, _ := range msg.RcptDomains {

			mailServer, err := lookupMailServer(strings.ToLower(domain), 0)
			if err != nil {
				writeErrorResponse(conn,"message %s can't get MX record for %s - %s, DROPPED: %s", msg.String(), domain, err.Error(), ErrDomainNotFound.Error())
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
		conn.Write([]byte(email.GetMessageId()+" "+ErrStatusSuccess.Error()))

	}

	<-TCPLimiter
	return
}