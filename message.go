package main
import (
    "strings"
    "errors"
    "net/mail"
    "bytes"
)

type EmailAddress struct {
    Address string
    Alias string
    Domain string
}

type Msg struct {
    Rcpt [] EmailAddress
    Sender  EmailAddress
    Message *mail.Message
}

func ParseDomain(addr string) (domain string,err error){
    var parts = strings.Split(addr,"@")
    if(len(parts)==0 || len(parts)>2){
        return "",errors.New("illegal addr "+addr)
    }
    return parts[1],nil
}

func ParseMessage(recipients []string,sender string,data []byte) (msg Msg,err error){

    msg.Sender,err = ParseAddress(sender)
    if err != nil {
        return msg,err
    }
    for _, rcpt := range recipients {
        rcptAddr,err:=ParseAddress(rcpt)
        if err != nil {
            return msg,err
        }
        msg.Rcpt = append(msg.Rcpt,rcptAddr)
    }
    msg.Message,err = mail.ReadMessage(bytes.NewReader(data))
    if err != nil {
        return msg,err
    }
    return msg,nil
}

func ParseAddress(rawAddress string) (address EmailAddress,err error){
    addr,err := mail.ParseAddress(rawAddress)
    if err!=nil{
        return address,err
    }
    address.Address = addr.Address
    address.Alias = addr.Name
    address.Domain,err = ParseDomain(address.Address)
    if err != nil {
        return address,err
    }
    return address,nil
}
