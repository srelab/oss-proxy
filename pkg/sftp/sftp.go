package sftp

import (
	"fmt"
	"io/ioutil"

	"io"
	"net"

	"github.com/pkg/sftp"
	"github.com/srelab/common/color"
	"github.com/srelab/ossproxy/pkg/g"
	"github.com/srelab/ossproxy/pkg/logger"
	"golang.org/x/crypto/ssh"
)

func handleChannels(chans <-chan ssh.NewChannel) {
	for newChannel := range chans {
		// Channels have a type, depending on the application level
		// protocol intended. In the case of an SFTP session, this is "subsystem"
		// with a payload string of "<length=4>sftp"
		logger.Infof("Incoming channel: %s", newChannel.ChannelType())

		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			logger.Warnf("Unknown channel type: %s", newChannel.ChannelType())

			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			logger.Fatal("could not accept channel.", err)
		}

		// accept
		logger.Info("Channel accepted")

		// Sessions have out-of-band requests such as "shell",
		// "pty-req" and "env".  Here we handle only the
		// "subsystem" request.
		go func(in <-chan *ssh.Request) {
			for req := range in {
				logger.Info("Request: %v", req.Type)

				ok := false
				switch req.Type {
				case "subsystem":
					logger.Infof("Subsystem: %s", req.Payload[4:])

					if string(req.Payload[4:]) == "sftp" {
						ok = true
					}
				}

				logger.Infof("channel accepted: %v", ok)
				req.Reply(ok, nil)
			}
		}(requests)

		root := NewOssHandler()
		server := sftp.NewRequestServer(channel, root)
		if err := server.Serve(); err == io.EOF {
			server.Close()
			logger.Infof("sftp client exited session.")
		} else if err != nil {
			logger.Fatal("sftp server completed with error:", err)
		}
	}
}

func Start() {
	// An SSH server is represented by a ServerConfig, which holds
	// certificate details and handles authentication of ServerConns.
	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			// Should use constant-time compare (or better, salt+hash) in
			// a production setting.
			logger.Infof("User Login: %s", c.User())
			if c.User() == "testuser" && string(pass) == "tiger" {
				return nil, nil
			}

			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
	}

	privateBytes, err := ioutil.ReadFile(g.Config().Sftp.Keypath)
	if err != nil {
		logger.Fatal("Failed to load private key", err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		logger.Fatal("Failed to parse private key", err)
	}

	// AddHostKey adds a private key as a host key
	config.AddHostKey(private)

	// Once a ServerConfig has been configured, connections can be accepted.
	address := fmt.Sprintf("%s:%s", g.Config().Sftp.Host, g.Config().Sftp.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		logger.Fatal("failed to listen for connection", err)
	}

	color.Printf("⇨ sftp server started on %s\n", color.Green(listener.Addr()))

	for {
		nConn, err := listener.Accept()
		if err != nil {
			logger.Fatal("failed to accept incoming connection", err)
		}

		// Before use, a handshake must be performed on the incoming net.Conn.
		sconn, chans, reqs, err := ssh.NewServerConn(nConn, config)
		if err != nil {
			logger.Error("failed to handshake", err)
			nConn.Close()

			continue
		}

		logger.Info("user login detected:", sconn.User())
		logger.Info("SSH server established")

		// The incoming Request channel must be serviced.
		go ssh.DiscardRequests(reqs)

		// Service the incoming Channel channel.
		go handleChannels(chans)
	}
}
