package main

import (
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

		if user == "" || token == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		ip, err := webssh.Query(token)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)

			w.Write([]byte(err.Error()))
			return
		}
		if ip == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		conn, err := net.Dial("tcp", ip+":22")
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		config := ssh.ClientConfig{
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			User:            user,
			Auth: []ssh.AuthMethod{
				ssh.Password(""),
			},
		}

		err = wssh.NewSSHClient(conn, &config)
		if err == nil {
			err = wssh.NewSSHXtermSession()
			if err == nil {
				err = wssh.NewSftpSession()
			}
		}

		if err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))

			wssh.Cleanup()
			return
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
			wssh.Cleanup()
			return
		}
		wssh.AddWebsocket(ws)
	})
	http.ListenAndServe(":80", nil)
}
