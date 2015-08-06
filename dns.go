package main

import (
	"errors"
	"net"
)

func lookupMailServer(domain string, errorCount int) (string, error) {
	mxList, err := net.LookupMX(domain)
	if err != nil {
		return "", err
	}
	l := len(mxList)
	if l == 0 {
		return "", errors.New("MX record not found")
	}
	var mx *net.MX
	for errorCount > l {
		errorCount = errorCount - l
	}
	mx = mxList[errorCount]

	return mx.Host[:len(mx.Host)-1] + ":25", nil
}
