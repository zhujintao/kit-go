package ssh

import (
	"time"

	"golang.org/x/crypto/ssh"
)

func NewViaConn(addr string, user, password string) (*sshConn, error) {
	c, err := NewConn(addr, user, password)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *sshConn) NewClient(addr string, user, password string) (*ssh.Client, error) {
	conn, err := s.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	c, ch, req, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{User: user, Auth: []ssh.AuthMethod{ssh.Password(password)}, HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: time.Second * s.sec})

	if err != nil {
		return nil, err
	}
	cli := ssh.NewClient(c, ch, req)

	return cli, nil
}

func (s *sshConn) NewSession(addr string, user, password string) (*ssh.Session, error) {
	cli, err := s.NewClient(addr, user, password)
	if err != nil {
		return nil, err
	}

	session, err := cli.NewSession()
	if err != nil {
		return nil, err
	}

	return session, nil
}
