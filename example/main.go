package main

import (
	"log"
	"net"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/myml/webssh"
	"golang.org/x/crypto/ssh"
)

func main() {
	http.Handle("/", http.FileServer(http.Dir(".")))
	http.HandleFunc("/api/ssh", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("Sec-WebSocket-Key")
		token := r.URL.Query().Get("token")
		user := r.URL.Query().Get("user")
		var wssh = webssh.NewWebSSH(id)
		conn, err := net.Dial("tcp", token)
		if err != nil {
			log.Panic(err)
		}

		config := ssh.ClientConfig{
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			User:            user,
			Auth: []ssh.AuthMethod{
				ssh.Password(""),
			},
		}
		err = wssh.NewSSHClient(conn, &config)
		if err != nil {
			log.Panic(err)
		}

		err = wssh.NewSSHXtermSession()
		if err != nil {
			log.Panic(err)
		}
		err = wssh.NewSftpSession()
		if err != nil {
			log.Panic(err)
		}

		upgrader := websocket.Upgrader{
			// cross origin domain
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
			Subprotocols:    []string{"webssh"},
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
		}
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Panic(err)
		}
		wssh.AddWebsocket(ws)
	})
	log.Println("start")
	http.ListenAndServe(":80", nil)
}
