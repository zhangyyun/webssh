package vnc

import (
	"log"
	"net"

	"github.com/gorilla/websocket"
)

func Proxy(logger *log.Logger, ws *websocket.Conn, conn net.Conn) {
	logger.Printf("vnc start working")

	go func() {
		defer logger.Printf("close websocket")
		defer ws.Close()
		for {
			buffer := make([]byte, 1024)
			n, err := conn.Read(buffer)
			if err != nil || n == 0 {
				logger.Printf("tcp conn read failed %s", err.Error())
				return
			}
			ws.WriteMessage(websocket.BinaryMessage, buffer[:n])
		}
	}()
	defer logger.Printf("close tcp conn")
	defer conn.Close()
	for {
		msgType, msg, err := ws.ReadMessage()
		if err != nil {
			logger.Printf("websocket read failed %s", err.Error())
			return
		}
		if msgType != websocket.BinaryMessage {
			logger.Printf("Non binary message recieved")
		}
		_, err = conn.Write(msg)
		if err != nil {
			logger.Printf("tcp conn write failed %s", err.Error())
			return
		}
	}
}
