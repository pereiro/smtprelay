package main

import (
	"smtprelay/log4go"
)

var log log4go.Logger

//const LOG_CONFIG_FILE string = "logconfig.xml"

func InitLogger(logconfig string) {
	log = log4go.NewDefaultLogger(log4go.DEBUG)
	log.LoadConfiguration(logconfig)
}
