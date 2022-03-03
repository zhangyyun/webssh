package vnc

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/gorilla/websocket"
	"github.com/myml/webssh/common"
)

func Proxy(logger *log.Logger, ws *websocket.Conn, conn net.Conn) {
	logger.Printf("vnc start working %s->%s", ws.RemoteAddr().String(), conn.RemoteAddr().String())

	ch := make(chan struct{}, 1)
	defer conn.Close()
	defer close(ch)
	go func() {
		if ok := common.KeepAlive(ws, ch, logger); !ok {
			ws.Close()
		}
	}()

	go func() {
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
	for {
		ctx, cancel := context.WithCancel(context.Background())
		go func(ctx context.Context) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Minute):
				logger.Printf("no user input, closing...")
				ws.Close()
			}
		}(ctx)
		msgType, msg, err := ws.ReadMessage()
		cancel()

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
