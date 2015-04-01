package main
import (
    "net"
)


func lookupMailServer(domain string ) (string,error){
    mxList,err:=net.LookupMX(domain)
    if err!=nil{
        return "",err
    }
    var mx *net.MX = mxList[0]
    return mx.Host[:len(mx.Host)-1]+":25",nil
}
