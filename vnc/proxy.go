package vnc

import (
	"log"
	"net"

	"github.com/gorilla/websocket"
)

func Proxy(id string, ws *websocket.Conn, conn net.Conn) {
	go func() {
		defer log.Printf("[%s]close websocket", id)
		defer ws.Close()
		for {
			buffer := make([]byte, 1024)
			n, err := conn.Read(buffer)
			if err != nil || n == 0 {
				log.Printf("[%s]tcp conn read failed %s", id, err.Error())
				return
			}
			ws.WriteMessage(websocket.BinaryMessage, buffer[:n])
		}
	}()
	defer log.Printf("[%s]close tcp conn", id)
	defer conn.Close()
	for {
		msgType, msg, err := ws.ReadMessage()
		if err != nil {
			log.Printf("[%s]websocket read failed %s", id, err.Error())
			return
		}
		if msgType != websocket.BinaryMessage {
			log.Printf("[%s]Non binary message recieved", id)
		}
		_, err = conn.Write(msg)
		if err != nil {
			log.Printf("[%s]tcp conn write failed %s", id, err.Error())
			return
		}
	}
}
