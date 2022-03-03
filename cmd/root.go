/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/myml/webssh/common"
	"github.com/myml/webssh/dcv"
	webssh "github.com/myml/webssh/ssh"
	"github.com/myml/webssh/vnc"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var (
	web  string
	port uint16
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "webssh",
	Short: "A ssh&vnc proxy",
	Long: `WebSSH proxy websocket to ssh or vnc.

It enables a web client to connect to the destination host.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: serve,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.webssh.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().Uint16VarP(&port, "port", "p", 80, "port to listen on")
	rootCmd.Flags().StringVar(&web, "web", "", "web dir to serve")
}

func serve(cmd *cobra.Command, args []string) {
	if web != "" {
		web, err := filepath.Abs(web)
		if err == nil {
			http.Handle("/", http.FileServer(http.Dir(web)))
		}
	}
	http.HandleFunc("/ssh", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("Sec-WebSocket-Key")
		token := r.URL.Query().Get("token")
		user := r.URL.Query().Get("user")

		if user == "" || token == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		logger := log.New(os.Stdout, "["+id+"] ", log.Ltime|log.Ldate)
		wssh := webssh.NewWebSSH(logger)

		conn, err, respCode := common.GetTargetConn(token, 22)
		if conn == nil {
			logger.Printf("ssh get target connection failed with %d(%s)", respCode, err)
			if respCode == 0 {
				respCode = http.StatusInternalServerError
			}
			if err != nil {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(respCode)

				w.Write([]byte(err.Error()))
			} else {
				w.WriteHeader(respCode)
			}
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
			logger.Printf("ssh create sessions failed %s", err)

			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))

			wssh.Cleanup()
			return
		}

		upgradeHeader := http.Header{"Sec-Websocket-Protocol": []string{"webssh"}}
		ws, err := common.Upgrade(w, r, upgradeHeader)
		if err != nil {
			logger.Printf("ssh upgrade websocket failed %s", err)
			wssh.Cleanup()
			return
		}
		wssh.AddWebsocket(ws)
	})
	http.HandleFunc("/vnc", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("Sec-WebSocket-Key")
		token := r.URL.Query().Get("token")
		if token == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		logger := log.New(os.Stdout, "["+id+"] ", log.Ltime|log.Ldate)

		conn, err, respCode := common.GetTargetConn(token, 5901)
		if conn == nil {
			logger.Printf("vnc get target connection failed with %d(%s)", respCode, err)
			if respCode == 0 {
				respCode = http.StatusInternalServerError
			}
			if err != nil {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(respCode)

				w.Write([]byte(err.Error()))
			} else {
				w.WriteHeader(respCode)
			}
			return
		}

		upgradeHeader := http.Header{"Sec-Websocket-Protocol": []string{"webssh"}}
		ws, err := common.Upgrade(w, r, upgradeHeader)
		if err != nil {
			logger.Printf("vnc upgrade websocket failed %s", err)
			conn.Close()
			return
		}

		go vnc.Proxy(logger, ws, conn)
	})
	http.HandleFunc("/dcv/", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("Sec-WebSocket-Key")
		ss := strings.SplitN(r.URL.Path, "/", 4)
		if len(ss) < 3 || ss[2] == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		token := ss[2]
		path := ""
		if len(ss) == 4 {
			path = ss[3]
		}

		logger := log.New(os.Stdout, "["+id+"/"+path+"] ", log.Ltime|log.Ldate)

		connBackend, rsp, err := common.Client(token, path, r)
		if connBackend == nil || err != nil {
			if err != nil {
				logger.Printf("dcv get target connection failed with (%s)", err)
			}
			if rsp != nil {
				for k, vv := range rsp.Header {
					for _, v := range vv {
						w.Header().Add(k, v)
					}
				}
				w.WriteHeader(rsp.StatusCode)

				io.Copy(w, rsp.Body)

			} else {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusBadRequest)

				w.Write([]byte(err.Error()))
			}
			return
		}

		upgradeHeader := http.Header{}
		if hdr := rsp.Header.Get("Sec-Websocket-Protocol"); hdr != "" {
			upgradeHeader.Set("Sec-Websocket-Protocol", hdr)
		}
		if hdr := rsp.Header.Get("Server"); hdr != "" {
			upgradeHeader.Set("Server", hdr)
		}
		connFrontend, err := common.Upgrade(w, r, upgradeHeader)
		if err != nil {
			logger.Printf("dcv upgrade websocket failed %s", err)
			connBackend.Close()
			return
		}

		go dcv.Proxy(logger, connFrontend, connBackend)
	})
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
