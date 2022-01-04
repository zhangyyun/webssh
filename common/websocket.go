package common

import (
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	upgrader := websocket.Upgrader{
		// cross origin domain
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		Subprotocols:    []string{"webssh"},
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}
	return upgrader.Upgrade(w, r, nil)
}

func KeepAlive(conn *websocket.Conn, ch chan struct{}, logger *log.Logger) bool {
	cnt := 0
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()

	//disable tcp keepalive, use websocket ping/pong instead
	conn.UnderlyingConn().(*net.TCPConn).SetKeepAlive(false)
	conn.SetPongHandler(func(string) error { ch <- struct{}{}; return nil })

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
