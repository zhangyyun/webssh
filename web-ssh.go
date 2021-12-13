package webssh

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"os"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func NewWebSSH(id string) *WebSSH {
	return &WebSSH{
		id:       id,
		buffSize: 256 * 1024,
		logger:   log.New(os.Stdout, "[webssh] ", log.Ltime|log.Ldate),
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
	id        string
	logger    *log.Logger
	buffSize  uint32
	websocket *websocket.Conn
	conn      *ssh.Client
	sshSess   *session
	sftpSess  *session
}

func (ws *WebSSH) Cleanup() {
	if ws.sshSess != nil {
		ws.sshSess.close()
	}
	if ws.sftpSess != nil {
		ws.sftpSess.close()
	}
	if ws.conn != nil {
		ws.conn.Close()
	}
	if ws.websocket != nil {
		ws.websocket.Close()
	}
}

func (ws *WebSSH) SetId(id string) {
	ws.id = id
}

// SetLogger set logger
func (ws *WebSSH) SetLogger(logger *log.Logger) *WebSSH {
	ws.logger = logger
	return ws
}

// SetBuffSize set buff size
func (ws *WebSSH) SetBuffSize(buffSize uint32) *WebSSH {
	ws.buffSize = buffSize
	return ws
}

// SetLogOut set logger output
func (ws *WebSSH) SetLogOut(out io.Writer) *WebSSH {
	ws.logger.SetOutput(out)
	return ws
}

// AddWebsocket add websocket connect
func (ws *WebSSH) AddWebsocket(conn *websocket.Conn) {
	ws.logger.Println("add websocket", ws.id)

	ws.websocket = conn

	go func() {
		ws.logger.Printf("%s server exit %v", ws.id, ws.server())
	}()
}

func (ws *WebSSH) server() error {
	defer ws.Cleanup()

	if err := ws.transformOutput(ws.sshSess, ws.sftpSess, ws.websocket); err != nil {
		return err
	}

	// Start remote shell
	if err := ws.sshSess.sess.Shell(); err != nil {
		return errors.Wrap(err, "shell")
	}
	for {
		var msg message
		msgType, data, err := ws.websocket.ReadMessage()
		if err != nil {
			return errors.Wrap(err, "websocket read")
		}
		if msgType == websocket.BinaryMessage {
			_, err = ws.sftpSess.stdin.Write(data)
			if err != nil {
				return errors.Wrap(err, "write sftp")
			}
			continue
		} else {
			err = json.Unmarshal(data, &msg)
			if err != nil {
				return errors.Wrap(err, "json unmarshal")
			}
			ws.logger.Println("new message", msg.Type)
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

func unmarshalUint32(b []byte) (uint32, []byte) {
	v := uint32(b[3]) | uint32(b[2])<<8 | uint32(b[1])<<16 | uint32(b[0])<<24
	return v, b[4:]
}

func (ws *WebSSH) transformOutput(ssh *session, sftp *session, conn *websocket.Conn) error {
	ws.logger.Println("transfer")
	copyShellOutput := func(t messageType, r io.Reader) {
		ws.logger.Println("copy to", t)
		buff := make([]byte, ws.buffSize)
		for {
			n, err := r.Read(buff)
			if err != nil {
				ws.logger.Printf("%s read fail", t)
				return
			}
			err = conn.WriteJSON(&message{Type: t, Data: buff[:n]})
			if err != nil {
				ws.logger.Printf("%s write fail", t)
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
