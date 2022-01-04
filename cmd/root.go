/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/myml/webssh/common"
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
	Short: "A ssh&sftp proxy",
	Long: `WebSSH proxy websocket to ssh.

It enables a web client to ssh to the destination host.`,
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
		var wssh = webssh.NewWebSSH(id)

		if user == "" || token == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		conn, err, respCode := common.GetTargetConn(token, 22)
		if conn == nil {
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
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))

			wssh.Cleanup()
			return
		}

		ws, err := common.Upgrade(w, r)
		if err != nil {
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
		conn, err, respCode := common.GetTargetConn(token, 5901)
		if conn == nil {
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

		ws, err := common.Upgrade(w, r)
		if err != nil {
			conn.Close()
			return
		}

		vnc.Proxy(id, ws, conn)
	})
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
