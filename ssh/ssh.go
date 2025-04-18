package ssh

import (
	"golang.org/x/crypto/ssh"
)

func NewConn(addr string, user, password string) (*ssh.Client, error) {
	return ssh.Dial("tcp", addr, &ssh.ClientConfig{User: user, Auth: []ssh.AuthMethod{ssh.Password(password)}, HostKeyCallback: ssh.InsecureIgnoreHostKey()})
}
