package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	mathrand "math/rand"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	errBadPassword = errors.New("permission denied")
	serverVersions = []string{
		"SSH-2.0-OpenSSH_6.6.1p1 Ubuntu-2ubuntu2.3",
		"SSH-2.0-OpenSSH_6.7p1 Debian-5+deb8u3",
		"SSH-2.0-OpenSSH_7.2p2 Ubuntu-4ubuntu2.10",
		"SSH-2.0-OpenSSH_7.4",
		"SSH-2.0-OpenSSH_8.0",
		"SSH-2.0-OpenSSH_8.4p1 Debian-2~bpo10+1",
		"SSH-2.0-OpenSSH_8.4p1 Debian-5+deb11u1",
	}
)

const (
	defaultLogPath    = "/var/log/fakessh/fakessh.log"
	defaultListenAddr = ":22"
)

var (
	logPath    string
	listenAddr string

	logger = log.New(io.Discard, "", log.LstdFlags)
)

func openLogFile() error {
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Println("Failed to open log file:", logPath, err)
		os.Exit(1)
	}

	prevWriter := logger.Writer()
	logger.SetOutput(logFile)
	if closer, ok := prevWriter.(io.Closer); ok {
		closer.Close()
	}
	return nil
}

func main() {
	flag.StringVar(&logPath, "log", defaultLogPath, "log file path")
	flag.StringVar(&listenAddr, "listen", defaultListenAddr, "listen address")
	flag.Parse()

	openLogFile()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP)
	go func() {
		<-sigCh
		logger.Println("[system] received SIGHUP, reopening log file")
		openLogFile()
	}()

	serverConfig := &ssh.ServerConfig{
		MaxAuthTries:     3,
		PasswordCallback: passwordCallback,
		ServerVersion:    serverVersions[0],
	}

	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	signer, _ := ssh.NewSignerFromSigner(privateKey)
	serverConfig.AddHostKey(signer)

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Println("Failed to listen:", err)
		os.Exit(1)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Failed to accept:", err)
			break
		}
		go handleConn(conn, serverConfig)
	}
}

func passwordCallback(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	logger.Printf("[auth] ip=%s version=%q user=%q password=%q\n",
		conn.RemoteAddr(),
		string(conn.ClientVersion()),
		conn.User(),
		string(password))
	if string(conn.ClientVersion()) == "SSH-2.0-Go" {
		return nil, errBadPassword
	}
	return nil, nil
}

func handleConn(conn net.Conn, serverConfig *ssh.ServerConfig) {
	defer conn.Close()
	logger.Printf("[conn] ip=%s\n", conn.RemoteAddr())
	c, newChanCh, reqCh, err := ssh.NewServerConn(conn, serverConfig)
	if err != nil {
		if _, ok := err.(*ssh.ServerAuthError); ok {
			// don't log authentication failures
		} else if err == io.EOF {
			logger.Printf("[conn] ip=%s err=io.EOF\n", conn.RemoteAddr())
		} else {
			logger.Printf("[conn] ip=%s err=%q\n", conn.RemoteAddr(), err)
		}
		return
	}
	defer c.Close()
	go ssh.DiscardRequests(reqCh)
	handleServerConn(c, newChanCh)
}

func handleServerConn(c *ssh.ServerConn, newChanCh <-chan ssh.NewChannel) {
	for newChan := range newChanCh {
		if newChan.ChannelType() != "session" {
			newChan.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type %s", newChan.ChannelType()))
			continue
		}

		ch, reqs, err := newChan.Accept()
		if err != nil {
			continue
		}
		go handleSessionChannel(c, ch, reqs)
	}
}

func handleSessionChannel(c *ssh.ServerConn, ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	for req := range reqs {
		switch req.Type {
		case "exec":
			handleExecRequest(c, ch, req)
			return
		case "shell":
			handleShellRequest(c, ch, req)
			return
		default:
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
}

func handleExecRequest(c *ssh.ServerConn, ch ssh.Channel, req *ssh.Request) {
	if len(req.Payload) < 4 {
		logger.Printf("[exec] ip=%s cmd=<invalid>\n", c.RemoteAddr())
		req.Reply(false, nil)
		return
	}

	cmdlen := int(binary.BigEndian.Uint32(req.Payload[0:4]))
	if len(req.Payload) != 4+cmdlen {
		s := fmt.Sprintf("wrong command length, want %d, got %d", cmdlen, len(req.Payload)-4)
		logger.Printf("[warning] [exec] ip=%s err=%q\n", c.RemoteAddr(), s)
	}
	logger.Printf("[exec] ip=%s cmd=%q\n", c.RemoteAddr(), string(req.Payload[4:]))
	junkSize := cmdlen + mathrand.Intn(3*cmdlen)
	io.CopyN(ch, rand.Reader, int64(junkSize))
	ch.Write([]byte{'\n'})
	ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
}

func handleShellRequest(c *ssh.ServerConn, ch ssh.Channel, req *ssh.Request) {
	logger.Printf("[shell] ip=%s\n", c.RemoteAddr())
	head := make([]byte, 0, 1024)
	buf := make([]byte, 4096)
	total := 0
	start := time.Now()

	prompt := fmt.Sprintf("[%s@localhost] $ ", c.User())
	io.WriteString(ch, prompt)

outer:
	for {
		n, err := ch.Read(buf)
		if err != nil && err != io.EOF {
			total += n
			logger.Printf("[shell] ip=%s bytes=%d err=%q\n", c.RemoteAddr(), total, err)
			return
		}
		if total < 100 {
			copySize := 100 - total
			if copySize > n {
				copySize = n
			}
			copy(head[total:100], buf[:n])
			head = head[:len(head)+copySize]
		}
		total += n

		previousNewline := 0
		for i := 0; i < n; i++ {
			if buf[i] == '\n' {
				// echo back the line
				ch.Write(buf[previousNewline : i+1])
				if bytes.Equal(bytes.TrimSpace(buf[previousNewline:i+1]), []byte("exit")) {
					break outer
				}

				previousNewline = i + 1
				junkSize := len(head) + mathrand.Intn(3*len(head))
				io.CopyN(ch, rand.Reader, int64(junkSize))
				ch.Write([]byte{'\n'})
				io.WriteString(ch, prompt)
			}
		}
		// echo back the rest of the buffer
		ch.Write(buf[previousNewline:n])

		if err == io.EOF {
			break
		}
	}

	dur := time.Since(start)
	dur -= dur % time.Second
	logger.Printf("[shell] ip=%s duration=%s bytes=%d head=%q\n", c.RemoteAddr(), dur, total, head)
	ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
}
