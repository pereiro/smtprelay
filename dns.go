package main

import (
	"errors"
	"fmt"
	"net"
)

func lookupMailServer(domain string, errorCount int) (string, error) {
	mxList, err := net.LookupMX(domain)
	if err != nil {
		return "", err
	}
	mx, err := getRoundElement(mxList, errorCount)
	if err != nil {
		return "", err
	}

	if len(mx.Host) == 0 {
		return "", errors.New(fmt.Sprintf("incorrect MX record for domain %s - %v;", domain, mx))
	}

	return mx.Host[:len(mx.Host)-1] + ":25", nil
}

func getRoundElement(mxList []*net.MX, errorCount int) (mx net.MX, err error) {
	l := len(mxList)
	if l == 0 {
		return mx, errors.New("MX record not found")
	}
	for errorCount >= l {
		errorCount = errorCount - l
	}
	mx = *mxList[errorCount]
	return
}
