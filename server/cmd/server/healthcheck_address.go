package main

import (
	"fmt"
	"net"
)

func healthcheckAddress(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("parse LH_HTTP_ADDR %q: %w", addr, err)
	}
	if port == "" {
		return "", fmt.Errorf("parse LH_HTTP_ADDR %q: port is empty", addr)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port), nil
}
