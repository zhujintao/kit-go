package ssh

import (
	"context"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

type sshConn struct {
	*ssh.Client
	sec time.Duration
}

func NewConn(addr string, user, password string, privatekeyFile ...string) (*sshConn, error) {
	auth := []ssh.AuthMethod{ssh.Password(password)}

	if len(privatekeyFile) == 1 {
		key, _ := os.ReadFile(privatekeyFile[0])
		if signer, err := ssh.ParsePrivateKey(key); err == nil {
			auth = append(auth, ssh.PublicKeys(signer))
		}

	}
	conn, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{User: user, Auth: auth, HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: time.Second * 5})
	if err != nil {
		return nil, err
	}
	return &sshConn{sec: 2, Client: conn}, nil
}

func (s *sshConn) SendHello(ctx context.Context) {

	if s == nil {
		return
	}

	go func() {

		ticker := time.NewTicker(time.Second * s.sec)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_, _, err := s.SendRequest("hello", true, nil)
				if err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}
