// Copyright Jetstack Ltd. See LICENSE for details.
package utils

import (
	"net"
	"strconv"
)

func FreePort() (string, error) {
	l, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	})
	if err != nil {
		return "", err
	}
	defer l.Close()

	port := l.Addr().(*net.TCPAddr).Port
	return strconv.Itoa(port), nil
}
