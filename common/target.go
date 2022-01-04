package common

import (
	"net"
	"net/http"
	"strconv"
)

func GetTargetConn(token string, port uint16) (net.Conn, error, int) {
	ip, err := Query(token)
	if err != nil {
		return nil, err, http.StatusInternalServerError
	}
	if ip == "" {
		return nil, nil, http.StatusNotFound
	}

	conn, err := net.Dial("tcp", ip+":"+strconv.Itoa(int(port)))
	if err != nil {
		return nil, err, http.StatusServiceUnavailable
	}
	return conn, nil, 0
}
