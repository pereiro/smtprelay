package main

import (
	"encoding/json"
	"io/ioutil"
)

const CONFIG_FILE string = "config.json"

var Config Conf

type Conf struct {
    ServerHostName string
	ListenPort     string
	WelcomeMessage string
    MaxIncomingConnections int
    MaxOutcomingConnections int
    DKIMEnabled bool
    DKIMKeyDir string
    TestModeEnabled bool
    TestModeServer string
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

}

func (cf *Conf) Load() error {
	file, err := ioutil.ReadFile(CONFIG_FILE)
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

func (cf *Conf) Save() error {
	data, err := json.Marshal(cf)
	if err != nil {
		log.Error("json marshall  error:%s\n,err")
		return err
	}
	err = ioutil.WriteFile(CONFIG_FILE, data, 0644)
	if err != nil {
		log.Error("file error")
		return err
	}
    return nil
}

