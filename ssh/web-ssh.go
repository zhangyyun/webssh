package ssh

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/myml/webssh/common"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func NewWebSSH(logger *log.Logger) *WebSSH {
	return &WebSSH{
		buffSize: 256 * 1024,
		logger:   logger,
		ch:       make(chan struct{}, 1),
	}
}

type session struct {
	sess *ssh.Session

	stdin  io.WriteCloser
	stdout io.Reader
	stderr io.Reader
}

func (s *session) close() {
	if s.sess != nil {
		s.sess.Close()
	}
	if s.stdin != nil {
		s.stdin.Close()
	}
}

type WebSSH struct {
	logger    *log.Logger
	buffSize  uint32
	websocket *websocket.Conn
	conn      *ssh.Client
	sshSess   *session
	sftpSess  *session
	ch        chan struct{}
}

func (ws *WebSSH) Cleanup() {
	ws.logger.Printf("cleanup")
	if ws.sshSess != nil {
		ws.sshSess.close()
		ws.sshSess = nil
	}
	if ws.sftpSess != nil {
		ws.sftpSess.close()
		ws.sftpSess = nil
	}
	if ws.conn != nil {
		ws.conn.Close()
		ws.conn = nil
	}
	if ws.websocket != nil {
		ws.websocket.Close()
		ws.websocket = nil
	}
	if ws.ch != nil {
		close(ws.ch)
		ws.ch = nil
	}
}

// SetBuffSize set buff size
func (ws *WebSSH) SetBuffSize(buffSize uint32) *WebSSH {
	ws.buffSize = buffSize
	return ws
}

// AddWebsocket add websocket connect
func (ws *WebSSH) AddWebsocket(conn *websocket.Conn) {
	ws.websocket = conn

	go func() {
		ws.logger.Printf("server exit %v", ws.server())
	}()
}

func (ws *WebSSH) server() error {
	ws.logger.Printf("ssh start working")

	defer ws.Cleanup()

	go func() {
		if ok := common.KeepAlive(ws.websocket, ws.ch, ws.logger); !ok {
			ws.Cleanup()
		}
	}()

	if err := ws.transformOutput(ws.sshSess, ws.sftpSess, ws.websocket); err != nil {
		return err
	}

	// Start remote shell
	if err := ws.sshSess.sess.Shell(); err != nil {
		return errors.Wrap(err, "shell")
	}
	for {
		ctx, cancel := context.WithCancel(context.Background())
		go func(ctx context.Context) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Minute):
				ws.logger.Printf("no user input, closing...")
				ws.Cleanup()
			}
		}(ctx)

		var msg message
		msgType, data, err := ws.websocket.ReadMessage()
		cancel()

		if err != nil {
			return errors.Wrap(err, "websocket read")
		}
		if msgType == websocket.BinaryMessage {
			if data[4] == 5 { //blacklist SSH_FXP_READ
				errMsg := os.ErrPermission.Error()
				langTag := "en"

				l := 4 + 1 + 4 + // uint32(length)+byte(type)+uint32(id)
					4 +
					4 + len(errMsg) +
					4 + len(langTag)
				buf := make([]byte, 0, l)
				buf = marshalUint32(buf, uint32(l-4))
				buf = append(buf, byte(101))    //type SSH_FXP_STATUS
				buf = append(buf, data[5:9]...) //id
				buf = marshalUint32(buf, 3)     //code SSH_FX_PERMISSION_DENIED
				buf = append(marshalUint32(buf, uint32(len(errMsg))), errMsg...)
				buf = append(marshalUint32(buf, uint32(len(langTag))), langTag...)

				err = ws.websocket.WriteMessage(websocket.BinaryMessage, buf)
				if err != nil {
					return errors.Wrap(err, "deny read")
				}
				continue
			}
			_, err = ws.sftpSess.stdin.Write(data)
			if err != nil {
				return errors.Wrap(err, "write sftp")
			}
		} else {
			err = json.Unmarshal(data, &msg)
			if err != nil {
				return errors.Wrap(err, "json unmarshal")
			}
			switch msg.Type {
			case messageTypeStdin:
				_, err = ws.sshSess.stdin.Write(msg.Data)
				if err != nil {
					return errors.Wrap(err, "write ssh")
				}
			case messageTypeResize:
				err = ws.sshSess.sess.WindowChange(msg.Rows, msg.Cols)
				if err != nil {
					return errors.Wrap(err, "resize")
				}
			}
		}
	}
}

func (ws *WebSSH) NewSSHClient(conn net.Conn, config *ssh.ClientConfig) error {
	var err error
	c, chans, reqs, err := ssh.NewClientConn(conn, conn.RemoteAddr().String(), config)
	if err != nil {
		return errors.Wrap(err, "tcp client")
	}
	ws.conn = ssh.NewClient(c, chans, reqs)
	return nil
}

// NewSSHXtermSession start ssh xterm session
func (ws *WebSSH) NewSSHXtermSession() error {
	var err error
	s, err := ws.conn.NewSession()
	if err != nil {
		return errors.Wrap(err, "ssh session")
	}
	// Set up terminal modes
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: ws.buffSize,
		ssh.TTY_OP_OSPEED: ws.buffSize,
	}
	// Request pseudo terminal
	err = s.RequestPty("xterm", 40, 80, modes)
	if err != nil {
		s.Close()
		return errors.Wrap(err, "pty")
	}

	stdin, err := s.StdinPipe()
	if err != nil {
		s.Close()
		return errors.Wrap(err, "ssh stdin")
	}
	stdout, err := s.StdoutPipe()
	if err != nil {
		s.Close()
		return errors.Wrap(err, "ssh stdout")
	}
	stderr, err := s.StderrPipe()
	if err != nil {
		s.Close()
		return errors.Wrap(err, "ssh stderr")
	}

	ws.sshSess = &session{
		sess:   s,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}
	return nil
}

func (ws *WebSSH) NewSftpSession() error {
	s, err := ws.conn.NewSession()
	if err != nil {
		return errors.Wrap(err, "sftp session")
	}
	if err := s.RequestSubsystem("sftp"); err != nil {
		return errors.Wrap(err, "sftp subsystem")
	}

	stdin, err := s.StdinPipe()
	if err != nil {
		s.Close()
		return errors.Wrap(err, "sftp stdin")
	}
	stdout, err := s.StdoutPipe()
	if err != nil {
		s.Close()
		return errors.Wrap(err, "sftp stdout")
	}
	ws.sftpSess = &session{
		sess:   s,
		stdin:  stdin,
		stdout: stdout,
	}
	return nil
}

func marshalUint32(b []byte, v uint32) []byte {
	return append(b, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func unmarshalUint32(b []byte) (uint32, []byte) {
	v := uint32(b[3]) | uint32(b[2])<<8 | uint32(b[1])<<16 | uint32(b[0])<<24
	return v, b[4:]
}

func (ws *WebSSH) transformOutput(ssh *session, sftp *session, conn *websocket.Conn) error {
	copyShellOutput := func(t messageType, r io.Reader) {
		buff := make([]byte, ws.buffSize)
		for {
			n, err := r.Read(buff)
			if err != nil {
				ws.logger.Printf("%s read failed %v", t, err)
				return
			}
			err = conn.WriteJSON(&message{Type: t, Data: buff[:n]})
			if err != nil {
				ws.logger.Printf("%s write failed %s", t, err)
				return
			}
		}
	}
	copySftpOutput := func(r io.Reader) {
		buf := make([]byte, ws.buffSize)
		for {
			if _, err := io.ReadFull(r, buf[:4]); err != nil {
				ws.logger.Printf("sftp read length failed %v", err)
				return
			}
			length, _ := unmarshalUint32(buf)
			if length > ws.buffSize-4 {
				ws.logger.Printf("recv packet %d bytes too long", length)
				return
			}
			if length == 0 {
				ws.logger.Printf("recv packet of 0 bytes too short")
				return
			}
			if _, err := io.ReadFull(r, buf[4:length+4]); err != nil {
				ws.logger.Printf("recv packet %d bytes: err %v", length, err)
				return
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:length+4]); err != nil {
				ws.logger.Printf("sftp write failed %v", err)
				return
			}
		}
	}
	go copyShellOutput(messageTypeStdout, ssh.stdout)
	go copyShellOutput(messageTypeStderr, ssh.stderr)
	go copySftpOutput(sftp.stdout)
	return nil
}
