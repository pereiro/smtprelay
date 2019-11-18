package main

import (
	"bytes"
	"errors"
	"fmt"
	"smtprelay/uuid"
	"net/mail"
	"strings"
)

type EmailAddress struct {
	RawAddress string
	Address    string
	Alias      string
	Domain     string
}

type Msg struct {
	Rcpt        []EmailAddress
	Sender      EmailAddress
	RcptDomains map[string]int
	MessageId   string
	Message     mail.Message
}

func (msg *Msg) String() string {
	var rcpt string
	for _, s := range msg.Rcpt {
		rcpt += s.Address + ";"
	}
	return fmt.Sprintf("(message-id:%s;from:%s;to:%s)", msg.MessageId, msg.Sender.Address, rcpt)
}

func ParseDomain(addr string) (domain string, err error) {
	var parts = strings.Split(addr, "@")
	if len(parts) == 0 || len(parts) > 2 {
		return "", errors.New("illegal addr " + addr)
	}
	return parts[1], nil
}

func ParseMessage(recipients []string, sender string, data []byte) (msg Msg, err error) {

	msg.Sender, err = ParseAddress(sender)
	if err != nil {
		return msg, err
	}
	for _, rcpt := range recipients {
		rcptAddr, err := ParseAddress(rcpt)
		if err != nil {
			return msg, err
		}
		msg.Rcpt = append(msg.Rcpt, rcptAddr)
	}
	message, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return msg, err
	}
	msg.Message = *message

	msg.MessageId = msg.Message.Header.Get("message-id")
	if msg.MessageId == "" {
		id := uuid.NewV4()
		uuid.SwitchFormat(uuid.Clean)
		msg.MessageId = id.String()
	}

	msg.RcptDomains = make(map[string]int)
	for _, d := range msg.Rcpt {
		msg.RcptDomains[d.Domain] = msg.RcptDomains[d.Domain] + 1
	}
	return msg, nil
}

func ParseAddress(rawAddress string) (address EmailAddress, err error) {
	addr, err := mail.ParseAddress(rawAddress)
	if err != nil {
		return address, err
	}
	address.RawAddress = rawAddress
	address.Address = addr.Address
	address.Alias = addr.Name
	address.Domain, err = ParseDomain(address.Address)
	if err != nil {
		return address, err
	}
	return address, nil
}

func (msg *Msg) GetDomainRecipientList(domain string) (recipients []string) {
	for _, rcpt := range msg.Rcpt {
		if strings.EqualFold(domain, rcpt.Domain) {
			recipients = append(recipients, rcpt.RawAddress)
		}
	}
	return
}
