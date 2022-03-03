package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

func query(token string) (string, error) {
	s := os.Getenv("SERVER_IP")
	p := os.Getenv("SERVER_PORT")
	cidr := os.Getenv("AGENT_CIDR")

	//if set, use token as ip directly, used for test only
	if _, test := os.LookupEnv("WEBSSH_TEST"); test {
		return token, nil
	}

	if s == "" || p == "" || cidr == "" {
		return "", errors.New("environ missing")
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return "", fmt.Errorf("ip format error")
	}
	portnum, err := strconv.ParseUint(p, 10, 64)
	if err != nil {
		return "", fmt.Errorf("port format error: %w", err)
	}
	_, iprange, err = net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("cidr format error: %w", err)
	}

	path := fmt.Sprintf("http://%s:%d/cm/desktop/ip_info?token=%s", s, portnum, url.QueryEscape(token))
	res, err := http.Get(path)
	if err != nil {
		return "", err
	}
	if res.StatusCode == 404 {
		return "", nil
	}
	if res.StatusCode > 299 {
		return "", errors.New(fmt.Sprintf("GET response code %d", res.StatusCode))
	}
	defer res.Body.Close()

	var info VmInfo
	err = json.NewDecoder(res.Body).Decode(&info)
	if err != nil {
		return "", err
	}

	ipstr := net.ParseIP(info.Ip)
	if ipstr == nil || !iprange.Contains(ipstr) {
		return "", errors.New("internal ip invalid")
	}

	return info.Ip, nil
}

func GetTargetConn(token string, port uint16) (net.Conn, error, int) {
	ip, err := query(token)
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
