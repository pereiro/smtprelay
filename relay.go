package main

import (
	"relay/smtpd"
    "runtime"
)
var(
    conf *Conf
)

func main() {
    InitLogger()
    log.Info("Starting..")
    conf=new(Conf)
    if err:=conf.Load();err!=nil{
        log.Critical("can't load config,shut down:",err.Error())
        panic(err.Error())
    }
    runtime.GOMAXPROCS(conf.NumCPU)

    if err := InitQueues();err!=nil{
        log.Critical("can't init redis MQ",err.Error())
        panic(err.Error())
    }
    log.Info("MQ initialized")
    go StartStatisticServer()
    go StartSender()

    if conf.DKIMEnabled {
        log.Info("DKIM enabled, loading keys..")
        err := DKIMLoadKeyRepository()
        if err!=nil {
            log.Critical("Can't load DKIM repo:%s",err.Error())
            panic(err.Error())
        }
    }else{
        log.Info("DKIM disabled")
    }

    if conf.TestModeEnabled {
        log.Info("TEST MODE ENABLED!! All messages will be redirected to %s",conf.TestModeServer)
    }
    log.Info("Incoming connections limit - %d",conf.MaxIncomingConnections)
    log.Info("Outcoming connections limit - %d",conf.MaxOutcomingConnections)

    server := &smtpd.Server{
        Hostname: conf.ServerHostName,
		WelcomeMessage: conf.WelcomeMessage,
		MaxConnections:conf.MaxIncomingConnections,
        Handler:        handler,
	}


	log.Info("SMTP Relay started at %s",conf.ListenPort)

    work := func (){
        server.ListenAndServe(conf.ListenPort)
/*        defer func() {
            if r:=recover();r!=nil{
                log.Critical("recovered from PANIC:%s",r)
            }
        }()*/
    }

    for  {
        work()
    }

}

func handler(peer smtpd.Peer, env smtpd.Envelope) error {
    //Processing only marketing emails with 1 recipient
    if(len(env.Recipients)>1 || len(env.Recipients)==0){
        log.Error("message dropped, rcpt count limited to 1: %s",ErrTooManyRecipients.Error())
        return ErrTooManyRecipients
    }
    msg,err:= ParseMessage(env.Recipients,env.Sender,env.Data)
    if err != nil {
        log.Error("message dropped - %s: %s",err,ErrMessageError.Error())
        return ErrMessageError
    }

    mailServer,err:=lookupMailServer(msg.Rcpt[0].Domain)
    if err!=nil{
        log.Error("message dropped, can't get MX record for %s: %s",msg.Rcpt[0].Domain,ErrDomainNotFound.Error())
        return ErrDomainNotFound
    }

    if conf.TestModeEnabled {
        mailServer = conf.TestModeServer
    }

    entry := QueueEntry{MailServer:mailServer,Sender:env.Sender,Recipients:env.Recipients,Data:env.Data,SenderDomain:msg.Sender.Domain}

    select {
        case MailMQChannel <- entry:
        default:
            err = PutMail(entry)
            if err != nil {
                log.Error("error writing msg in MQ - %s: %s",err.Error(),ErrServerError.Error())
                return ErrServerError
            }
    }

    return nil

}
