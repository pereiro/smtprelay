package main

import (
	"fmt"
	"net"
	"strings"
)

func lookupDeliveryServers(domain string) ([]string, error) {
	if conf.RelayModeEnabled {
		return []string{conf.RelayServer}, nil
	}
	return lookupMailServers(domain)
}

func lookupMailServers(domain string) ([]string, error) {
	domain = strings.TrimSuffix(strings.ToLower(domain), ".")
	if invalidMailHost(domain) {
		return nil, fmt.Errorf("invalid recipient domain %q", domain)
	}
	mxList, err := net.LookupMX(domain)
	if err != nil {
		return nil, err
	}
	return mailServersFromMX(domain, mxList)
}

func mailServersFromMX(domain string, mxList []*net.MX) ([]string, error) {
	servers := make([]string, 0, len(mxList))
	seen := make(map[string]bool)
	for _, mx := range mxList {
		if mx == nil {
			continue
		}
		host := strings.TrimSuffix(strings.ToLower(mx.Host), ".")
		if invalidMailHost(host) || seen[host] {
			continue
		}
		servers = append(servers, net.JoinHostPort(host, "25"))
		seen[host] = true
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("no usable MX records for domain %s", domain)
	}
	return servers, nil
}

func invalidMailHost(host string) bool {
	ip := net.ParseIP(host)
	return host == "" || host == "localhost" || (ip != nil && ip.IsLoopback())
}
