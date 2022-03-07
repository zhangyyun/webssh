package common

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (*websocket.Conn, error) {
	upgrader := websocket.Upgrader{
		// cross origin domain
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		ReadBufferSize:  BufferSize,
		WriteBufferSize: BufferSize,
	}
	return upgrader.Upgrade(w, r, responseHeader)
}

func KeepAlive(conn *websocket.Conn, ch chan struct{}, logger *log.Logger) bool {
	cnt := 0
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()

	//disable tcp keepalive, use websocket ping/pong instead
	conn.UnderlyingConn().(*net.TCPConn).SetKeepAlive(false)
	conn.SetPongHandler(func(m string) error { ch <- struct{}{}; return nil })

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				logger.Printf("websocket keepalive channel closed")
				return true
			}
			cnt = 0
		case <-tick.C:
			err := conn.WriteControl(websocket.PingMessage, []byte("webssh"), time.Time{})
			if err != nil {
				logger.Printf("websocket ping failed %v", err)
			}

			cnt++
			if cnt > 3 {
				logger.Printf("websocket client not responding")
				return false
			}
		}
	}
}

func ReadMessageWithIdleTime(conn *websocket.Conn, logger *log.Logger) (messageType int, p []byte, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	go func(ctx context.Context) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(IdleTime) * time.Minute):
			logger.Printf("no user input, closing...")
			Shutdown(conn, "session expired")
		}
	}(ctx)

	msgType, p, err := conn.ReadMessage()
	cancel()
	return msgType, p, err
}

func Shutdown(conn *websocket.Conn, msg string) error {
	message := websocket.FormatCloseMessage(websocket.CloseGoingAway, msg)

	err := conn.WriteControl(websocket.CloseMessage, message, time.Time{})
	if err != nil && err != websocket.ErrCloseSent {
		return conn.Close()
	}
	return nil
}

func Client(token, path string, r *http.Request) (*websocket.Conn, *http.Response, error) {
	ip, err := query(token)
	if err != nil {
		return nil, nil, err
	}

	if r.Header.Get("Upgrade") == "" {
		req, _ := http.NewRequest(http.MethodGet, "*", nil)
		req.URL.Scheme = "https"
		req.URL.Host = ip + ":8443"
		req.URL.Path = "/" + path
		req.URL.ForceQuery = r.URL.ForceQuery
		req.URL.RawQuery = r.URL.RawQuery
		req.URL.Fragment = r.URL.Fragment
		req.URL.RawFragment = r.URL.RawFragment

		req.Header = r.Header.Clone()

		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}

		//rsp, err := http.DefaultClient.Do(req)
		rsp, err := client.Do(req)
		return nil, rsp, err
	}

	d := &websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		ReadBufferSize:  BufferSize,
		WriteBufferSize: BufferSize,
	}

	rm := []string{
		"Upgrade",
		"Connection",
		"Sec-Websocket-Key",
		"Sec-Websocket-Version",
	}

	reqHeader := r.Header.Clone()
	for _, h := range rm {
		reqHeader.Del(h)
	}
	//TODO: dcvserver requests https, we may pass through when certificates are ready
	reqHeader.Set("Origin", "https://"+ip+":8443")
	return d.Dial("wss://"+ip+":8443/"+path, reqHeader)
}
