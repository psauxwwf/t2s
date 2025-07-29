package net

import (
	"net"
	"net/url"
)

func RandomPort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func ResolveHost(link string) string {
	hostname := GetDomain(link)
	if net.ParseIP(hostname) != nil {
		return hostname
	}

	ip, err := net.LookupIP(hostname)
	if err != nil {
		return hostname
	}

	return ip[0].String()
}

func GetDomain(link string) string {
	_url, err := url.Parse(link)
	if err != nil {
		return link
	}
	return _url.Hostname()
}

func ToIP(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return address
	}
	return host
}
