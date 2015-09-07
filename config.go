package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"
)

type Conf struct {
	ServerHostName          string
	ListenPort              string
	WelcomeMessage          string
	MaxIncomingConnections  int
	MaxOutcomingConnections int
	DKIMEnabled             bool
	DKIMKeyDir              string
	RelayModeEnabled        bool
	RelayServer             string
	NumCPU                  int
	StatisticPort           string
	DeferredMailDelay       int
	DeferredMailMaxErrors   int
	MaxRecipients           int
	ListenTCPPort           string
	TCPMaxConnections       int
	TCPTimeoutSeconds       int
}

func (cf *Conf) Load(filename string) error {
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Error("file error:%s", err)
		return err
	}

	err = json.Unmarshal(file, &cf)
	if err != nil {
		log.Error("json parse error:%s", err)
		return err
	}

	//	err = cf.CheckConfig()
	//	if err != nil {
	//		log.Error("configuration isn't valid:%s",err.Error())
	//	}

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

func (cf *Conf) CheckConfig() error {
	ref := reflect.ValueOf(*cf)
	for i := 0; i < ref.NumField(); i++ {
		field := ref.Field(i)
		if field.IsNil() {
			return errors.New(fmt.Sprintf("no value found for %s", ref.Type().Field(i).Name))
		}
	}
	return nil
}
