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

    if conf.RelayModeEnabled {
        log.Info("TEST MODE ENABLED!! All messages will be redirected to %s",conf.RelayServer)
    }
    log.Info("Incoming connections limit - %d",conf.MaxIncomingConnections)
    log.Info("Outcoming connections limit - %d",conf.MaxOutcomingConnections)

    server := &smtpd.Server{
        Hostname: conf.ServerHostName,
		WelcomeMessage: conf.WelcomeMessage,
		MaxConnections:conf.MaxIncomingConnections,
        Handler:        handlerPanicProcessor(handler),
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

func handlerPanicProcessor(handler func(peer smtpd.Peer, env smtpd.Envelope) error) func(peer smtpd.Peer, env smtpd.Envelope) error{
   return func(peer smtpd.Peer, env smtpd.Envelope) (err error){
       defer func(){
           if r:=recover();r!=nil{
               log.Critical("PANIC, message DROPPED:%s",r)
               err = ErrMessageErrorUnknown
           }
       }()
       return handler(peer,env)
   }
}


func handler(peer smtpd.Peer, env smtpd.Envelope) error {
    //#TODO Few recipients

    msg,err:= ParseMessage(env.Recipients,env.Sender,env.Data)
    if err != nil {
        log.Error("incorrect msg DROPPED - %s: %s",err,ErrMessageError.Error())
        return ErrMessageError
    }

    if(len(env.Recipients)>1 || len(env.Recipients)==0){
        log.Error("message %s DROPPED, rcpt count limited to 1: %s",msg.String(),ErrTooManyRecipients.Error())
        return ErrTooManyRecipients
    }

    mailServer,err:=lookupMailServer(msg.Rcpt[0].Domain)
    if err!=nil{
        log.Error("message %s DROPPED, can't get MX record for %s - %s: %s",msg.String(),msg.Rcpt[0].Domain,err.Error(),ErrDomainNotFound.Error())
        return ErrDomainNotFound
    }

    if conf.RelayModeEnabled {
        mailServer = conf.RelayServer
    }

    entry := QueueEntry{MailServer:mailServer,Sender:env.Sender,Recipients:env.Recipients,Data:env.Data,SenderDomain:msg.Sender.Domain,MessageId:msg.MessageId}

    select {
        case MailMQChannel <- entry:
        default:
            err = PutMail(entry)
            if err != nil {
                log.Error("msg %s DROPPED, MQ error - %s: %s",msg.String(),err.Error(),ErrServerError.Error())
                return ErrServerError
            }
            log.Info("msg %s QUEUED",msg.String())
    }

    return nil

}
