package dcv

import (
	"log"

	"github.com/gorilla/websocket"
	"github.com/myml/webssh/common"
)

func Proxy(logger *log.Logger, src *websocket.Conn, dst *websocket.Conn) {
	logger.Printf("dcv start working %s->%s", src.RemoteAddr().String(), dst.RemoteAddr().String())

	ch := make(chan struct{}, 1)
	defer close(ch)
	go func() {
		if ok := common.KeepAlive(src, ch, logger); !ok {
			src.Close()
		}
	}()

	go func() {
		defer src.Close()
		for {
			msgType, msg, err := dst.ReadMessage()
			if err != nil {
				logger.Printf("dst websocket read failed %s", err.Error())
				return
			}
			src.WriteMessage(msgType, msg)
		}
	}()
	for {
		defer dst.Close()

		msgType, msg, err := src.ReadMessage()

		if err != nil {
			logger.Printf("src websocket read failed %s", err.Error())
			return
		}
		err = dst.WriteMessage(msgType, msg)
		if err != nil {
			logger.Printf("dst websocket write failed %s", err.Error())
			return
		}
	}
}


