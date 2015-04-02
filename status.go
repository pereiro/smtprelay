package main
import (
    "relay/smtpd"
    "strconv"
    "strings"
)



var statusString = map[int]string{
    StatusPasswordNeeded:     "4.7.12  A password transition is needed",
    StatusMessageError:       "Requested mail action not taken",
    StatusTempAuthFailure:    "4.7.0  Temporary authentication failure",
    StatusAuthInvalid:        "5.7.8  Authentication credentials invalid",
    StatusAuthRequired:       "5.7.0  Authentication required",
    StatusEncryptionRequired: "5.7.11  Encryption required for requested authentication mechanism",
    StatusServerError:        "Requested mail action not taken: server error",
    StatusExceedStorage:      "Requested mail action aborted: exceeded storage allocation",
    StatusTooManyRecipients:        "Too many recipients",
    StatusDomainNotFound:       "Domain not found",
    StatusSuccess:              "OK",
}

func StatusString(status int) string {
    s, found := statusString[status]
    if !found {
        return "unknown"
    }
    return s
}

const (
    StatusSuccess              = 250
    StatusPasswordNeeded       = 432
    StatusMessageError         = 450
    StatusMessageExceedStorage = 452
    StatusTempAuthFailure      = 454
    StatusAuthInvalid          = 535
    StatusAuthRequired         = 530
    StatusEncryptionRequired   = 538
    StatusServerError          = 550
    StatusExceedStorage        = 552
    StatusDomainNotFound       = 554
    StatusTooManyRecipients    = 452
)

var (
    ErrStatusSuccess        = smtpd.Error{Code: StatusSuccess, Message: StatusString(StatusSuccess)}
    ErrPasswordNeeded       = smtpd.Error{Code: StatusPasswordNeeded, Message: StatusString(StatusPasswordNeeded)}
    ErrMessageError         = smtpd.Error{Code: StatusMessageError, Message: StatusString(StatusMessageError)}
    ErrMessageErrorUnknown  = smtpd.Error{Code: StatusMessageError, Message: StatusString(StatusMessageError)}
    ErrMessageExceedStorage = smtpd.Error{Code: StatusMessageExceedStorage, Message: StatusString(StatusMessageExceedStorage)}
    ErrTempAuthFailure      = smtpd.Error{Code: StatusTempAuthFailure, Message: StatusString(StatusTempAuthFailure)}
    ErrAuthInvalid          = smtpd.Error{Code: StatusAuthInvalid, Message: StatusString(StatusAuthInvalid)}
    ErrAuthRequired         = smtpd.Error{Code: StatusAuthRequired, Message: StatusString(StatusAuthRequired)}
    ErrServerError          = smtpd.Error{Code: StatusServerError, Message: StatusString(StatusServerError)}
    ErrExceedStorage        = smtpd.Error{Code: StatusExceedStorage, Message: StatusString(StatusExceedStorage)}
    ErrTooManyRecipients    = smtpd.Error{Code: StatusTooManyRecipients,Message: StatusString(StatusTooManyRecipients)}
    ErrDomainNotFound       = smtpd.Error{Code: StatusDomainNotFound,Message: StatusString(StatusDomainNotFound)}
    ErrServerErrorUnknown   = smtpd.Error{Code: StatusServerError,Message: StatusString(StatusServerError)}
)

func ParseOutcomingError(str string) (se smtpd.Error){
    str = strings.TrimSpace(str)
    if len(str)<3 {
        return ErrMessageErrorUnknown
    }
    parts := strings.SplitN(str," ",2)
    var code string
    var msg string
    if l:=len(parts);l>0 {
        code=parts[0]
        if l>1 {
            msg=parts[1]
        }
    }else{
        return smtpd.Error{Code:450,Message:str}
    }
    intCode,err:=strconv.Atoi(code)
    if err != nil {
        return smtpd.Error{Code:450,Message:str}
    }
    return smtpd.Error{Code:intCode,Message:msg}
}
