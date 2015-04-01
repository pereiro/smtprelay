package main

import(
    "code.google.com/p/log4go"
)

var log log4go.Logger
const LOG_CONFIG_FILE string = "logconfig.xml"

func InitLogger() {
    log = log4go.NewDefaultLogger(log4go.DEBUG)
    log.LoadConfiguration(LOG_CONFIG_FILE)
}