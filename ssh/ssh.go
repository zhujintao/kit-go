package ssh

import (
	"context"
	"time"

	"golang.org/x/crypto/ssh"
)

type sshConn struct {
	*ssh.Client
	sec time.Duration
}

func NewConn(addr string, user, password string) (*sshConn, error) {
	conn, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{User: user, Auth: []ssh.AuthMethod{ssh.Password(password)}, HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: time.Second * 5})
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
