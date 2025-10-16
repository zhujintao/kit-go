package ssh

import (
	"context"
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
