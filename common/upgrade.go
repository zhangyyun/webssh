package common

import (
	"net/http"

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
