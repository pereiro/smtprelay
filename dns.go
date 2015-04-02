package main
import (
    "net"
    "errors"
)


func lookupMailServer(domain string ) (string,error){
    mxList,err:=net.LookupMX(domain)
    if err!=nil{
        return "",err
    }
    if len(mxList)==0{
        return "",errors.New("MX record not found")
    }
    var mx *net.MX = mxList[0]
    return mx.Host[:len(mx.Host)-1]+":25",nil
}
