package ssh

import (
	"context"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

type Client = ssh.Client

func NewConn(addr string, user, password string) (*ssh.Client, error) {
	return ssh.Dial("tcp", addr, &ssh.ClientConfig{User: user, Auth: []ssh.AuthMethod{ssh.Password(password)}, HostKeyCallback: ssh.InsecureIgnoreHostKey()})
}

func Ping(ctx context.Context, c *Client, s time.Duration) {
	go func() {

		ticker := time.NewTicker(time.Second * s)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_, _, err := c.SendRequest("ping", true, nil)
				if err != nil {
					return
				}
			case <-ctx.Done():
				return

			}
		}

	}()
}

type sshcli struct {
	cli *ssh.Client
	sec time.Duration
}

func NewClient() *sshcli {

	return &sshcli{
		sec: 2,
	}

}

func (s *sshcli) NewDial(addr string, user, password string) (*ssh.Client, error) {
	cli, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{User: user, Auth: []ssh.AuthMethod{ssh.Password(password)}, HostKeyCallback: ssh.InsecureIgnoreHostKey()})

	if err != nil {
		return nil, err
	}
	s.cli = cli

	return cli, nil
}
func (s *sshcli) Ping(ctx context.Context) {

	go func() {

		ticker := time.NewTicker(time.Second * s.sec)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_, _, err := s.cli.SendRequest("ping", true, nil)
				if err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *sshcli) Dial(n, addr string) (net.Conn, error) {
	return s.cli.Dial(n, addr)
}
