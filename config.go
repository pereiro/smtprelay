package main

import (
	"encoding/json"
	"io/ioutil"
)

var Config Conf

type Conf struct {
    ServerHostName string
	ListenPort     string
	WelcomeMessage string
    MaxIncomingConnections int
    MaxOutcomingConnections int
    DKIMEnabled bool
    DKIMKeyDir string
    RelayModeEnabled bool
    RelayServer string
    NumCPU  int
    RedisMailQueueName string
    RedisErrorQueueName string
    RedisHost string
    RedisPort string
    RedisPassword string
    RedisDB int64
    MQStatisticPort string
    MQQueueBuffer int
    DeferredMailDelay int
    DeferredMailMaxErrors int
    MaxRecipients int

}

func (cf *Conf) Load(filename string) error {
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Error("file error:%s\n", err)
		return err
	}

	err = json.Unmarshal(file, &cf)
	if err != nil {
		log.Error("json parse error:%S\n", err)
		return err
	}
	return nil
}

func (cf *Conf) Save(filename string) error {
	data, err := json.Marshal(cf)
	if err != nil {
		log.Error("json marshall  error:%s\n,err")
		return err
	}
	err = ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		log.Error("file error")
		return err
	}
    return nil
}

