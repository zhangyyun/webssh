package dcv

import (
	"context"
	"log"
	"time"

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
		ctx, cancel := context.WithCancel(context.Background())
		go func(ctx context.Context) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Minute):
				logger.Printf("no user input, closing...")
				src.Close()
			}
		}(ctx)
		msgType, msg, err := src.ReadMessage()
		cancel()

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


