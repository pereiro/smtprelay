package main
import (
    "github.com/eaigner/dkim"
    "io/ioutil"
    "bytes"
    "errors"
    "os"
    "strings"
    "encoding/json"
)

const KEY_CONFIG_SUFFIX string = ".config"


type DKIM struct{
    Selector string
    Domain string
    Data []byte
    dkimConf dkim.Conf
    dkim dkim.DKIM
}

var DKIMRepo map[string] DKIM

func DKIMLoadKeyRepository() error{
    DKIMRepo = make(map[string]DKIM)
    dir,err := ioutil.ReadDir(conf.DKIMKeyDir)
    if err != nil {
        return err
    }
    var keyCount int
    var sep = os.PathSeparator
    for _, keyFile := range dir{
        if strings.Contains(keyFile.Name(),KEY_CONFIG_SUFFIX){
            break
        }

        key,err:=DKIMLoadConfig(conf.DKIMKeyDir+string(sep)+keyFile.Name()+KEY_CONFIG_SUFFIX)
        if err != nil {
            log.Warn("can't read key config from %s:%s",conf.DKIMKeyDir+string(sep)+keyFile.Name(),err.Error()+KEY_CONFIG_SUFFIX)
            break
        }

        key.Data,err =ioutil.ReadFile(conf.DKIMKeyDir+string(sep)+keyFile.Name())
        if err != nil {
            log.Warn("can't read key data from %s:%s",conf.DKIMKeyDir+string(sep)+keyFile.Name(),err.Error())
            break
        }
            DKIMRepo[key.Domain] = key;

        keyCount++;
    }
    log.Info("DKIM keys loaded - %d :", keyCount)
    for _,val:= range DKIMRepo{
        log.Info("%s - %s",val.Domain,val.Selector)
    }



    return nil
}

func DKIMLoadConfig(filename string) (keyConfig DKIM,err error){
    file, err := ioutil.ReadFile(filename)
    if err != nil {
        return keyConfig,err
    }

    err = json.Unmarshal(file, &keyConfig)
    if err != nil {
        return keyConfig,err
    }
    return keyConfig,nil
}

func DKIMSign(data []byte,domain string) ([]byte,error){
    var err error
    privKey := DKIMRepo[domain]
    if len(privKey.Domain)==0 {
        log.Error("no key for %s in keyrepo",domain)
        return data,errors.New("no key in keyrepo")
    }

    privKey.dkimConf,err = dkim.NewConf(privKey.Domain, privKey.Selector)
    if err != nil {
        log.Error("DKIM configuration error: %v", err)
        return data,errors.New("DKIM configuration error")
    }

    d, err := dkim.New(privKey.dkimConf, privKey.Data)
    if err != nil {
        log.Error("DKIM error: %v", err)
        return data,errors.New("DKIM error")
    }
    privKey.dkim = *d

   //log.Debug("Domain - %s\nSelector-%s\nData-%s",privKey.Domain,privKey.Selector,privKey.Data)
    //log.Debug("Mail data - %s",bytes.Replace(data, []byte("\n"), []byte("\r\n"), -1))

/*    _, err = dkim.New(dkimConf, privKey.Data)
    if err != nil {
        log.Error("DKIM error: %v", err)
        return data,err
    }*/

//    d, err := dkim.New(privKey.dkimConf, privKey.Data)
//    if err != nil {
//        log.Error("DKIM error: %v", err)
//        return data,err
//    }


    signeddata, err := privKey.dkim.Sign(bytes.Replace(data, []byte("\n"), []byte("\r\n"), -1))
    if err != nil {
        log.Error("DKIM signing error: %v", err)
        return data,err
    }
    return signeddata,nil
}
