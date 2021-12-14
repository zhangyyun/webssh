package webssh

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

var (
	client  naming_client.INamingClient
	iprange *net.IPNet
)

type VmInfo struct {
	Ip string `json:"ip"`
}

func try_init() (naming_client.INamingClient, error) {
	ip := os.Getenv("NACOS_SERVER_IP")
	port := os.Getenv("NACOS_SERVER_PORT")
	name := os.Getenv("NACOS_SERVER_USERNAME")
	passwd := os.Getenv("NACOS_SERVER_PASSWORD")
	cidr := os.Getenv("AGENT_CIDR")

	if ip == "" || port == "" || cidr == "" {
		return nil, errors.New("environment variables missing")
	}
	if name == "" {
		name = "nacos"
	}
	if passwd == "" {
		passwd = "nacos"
	}
	portnum, err := strconv.ParseUint(port, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("port format error: %w", err)
	}
	_, iprange, err = net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("cidr format error: %w", err)
	}

	sc := []constant.ServerConfig{
		*constant.NewServerConfig(ip, portnum),
	}

	cc := constant.NewClientConfig(
		constant.WithUsername(name),
		constant.WithPassword(passwd),
		constant.WithCacheDir("/var/cache/nacos/"),
		constant.WithLogLevel("debug"),
	)

	client, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  cc,
			ServerConfigs: sc,
		},
	)
	return client, err
}

func Query(token string) (string, error) {
	var err error
	if token == "" {
		return "", nil
	}

	if client == nil {
		client, err = try_init()
		if err != nil {
			return "", err
		}
	}
	instance, err := client.SelectOneHealthyInstance(vo.SelectOneHealthInstanceParam{
		ServiceName: "demo.go",
	})
	if err != nil {
		return "", err
	}

	path := fmt.Sprintf("http://%s:%d/x?token=%s", instance.Ip, instance.Port, url.QueryEscape(token))
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

	ip := net.ParseIP(info.Ip)
	if ip == nil || !iprange.Contains(ip) {
		return "", errors.New("internal ip invalid")
	}

	return info.Ip, nil
}
