package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

func NewClient(addr string, user, password string, viaSsh *sshConn) (*sshConn, error) {

	netConn, err := viaSsh.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	c, ch, req, err := ssh.NewClientConn(netConn, addr, &ssh.ClientConfig{User: user, Auth: []ssh.AuthMethod{ssh.Password(password)}, HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: time.Second * 5})
	if err != nil {
		return nil, err
	}
	cli := ssh.NewClient(c, ch, req)
	return &sshConn{Client: cli, sec: 5}, err
}

func NewViaServer(addr string, user, password string) (*sshConn, error) {
	s, err := NewConn(addr, user, password)
	if err != nil {
		return nil, err
	}

	s.SendHello(context.Background())

	return s, nil

}

func NewViaConn(addr string, user, password string) (*sshConn, error) {
	c, err := NewConn(addr, user, password)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *sshConn) NewClient(addr string, user, password string) (*sshConn, error) {
	conn, err := s.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	c, ch, req, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{User: user, Auth: []ssh.AuthMethod{ssh.Password(password)}, HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: time.Second * s.sec})

	if err != nil {
		return nil, err
	}
	cli := ssh.NewClient(c, ch, req)

	return &sshConn{Client: cli, sec: 5}, err
}
func (s *sshConn) ScpFrom(remotefile string, localfile string) error {

	session, err := s.NewSession()
	if err != nil {
		return err
	}

	defer session.Close()

	lf, err := os.Create(localfile)
	if err != nil {
		return err
	}
	defer lf.Close()

	rf, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	if err := session.Start("/usr/bin/scp -f " + remotefile); err != nil {
		return err
	}

	io.Copy(lf, rf)

	return session.Wait()

}

func (s *sshConn) ScpString(remotefile string, txt string) error {

	f := strings.NewReader(txt)
	filename := filepath.Base(remotefile)
	filedir := filepath.Dir(remotefile)

	session, err := s.NewSession()
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {

		w, err := session.StdinPipe()
		if err != nil {
			fmt.Println(err)
		}
		defer w.Close()
		fmt.Fprintf(w, "C0664 %d %s\n", f.Size(), filename)
		io.Copy(w, f)
		fmt.Fprintf(w, "\x00")
		wg.Done()

	}()

	err = session.Run("mkdir -p " + filedir + ";/usr/bin/scp -t " + filedir)
	return err
}

func (s *sshConn) ScpTo(remotefile string, localfile string) error {

	f, err := os.Open(localfile)
	if err != nil {
		return err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return err
	}

	filename := filepath.Base(remotefile)
	filedir := filepath.Dir(remotefile)

	session, err := s.NewSession()
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {

		w, err := session.StdinPipe()
		if err != nil {
			fmt.Println(err)
		}
		defer w.Close()
		fmt.Fprintf(w, "C0664 %d %s\n", stat.Size(), filename)
		io.Copy(w, f)
		fmt.Fprintf(w, "\x00")
		wg.Done()

	}()

	err = session.Run("mkdir -p " + filedir + ";/usr/bin/scp -t " + filedir)
	return err

}

func (s *sshConn) Pty() {

	session, err := s.NewSession()
	if err != nil {
		return
	}
	defer session.Close()
	fd := int(os.Stdin.Fd())
	oldState, _ := term.MakeRaw(fd)
	defer term.Restore(fd, oldState)
	w, h, _ := term.GetSize(fd)
	err = session.RequestPty("xterm-256color", h, w, ssh.TerminalModes{
		//ssh.ECHO:    0, // 禁用回显（用于密码输入）
		ssh.ECHOCTL: 0, // 禁用控制字符回显
		//ssh.ICRNL:         1,     // 将 CR 转换为 NL
		//ssh.ONLCR:         1,     // 将 NL 转换为 CR-NL
		//	ssh.ISIG:          1,     // 启用信号
		//ssh.ICANON:        1,     // 启用规范模式
		//ssh.OPOST:         1,     // 启用输出处理
		ssh.TTY_OP_ISPEED: 14400, // 输入速度
		ssh.TTY_OP_OSPEED: 14400,
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	err = session.Shell()
	if err != nil {

		fmt.Println(err)
		return
	}
	err = session.Wait()
	if err != nil {
		fmt.Println(err)
		return
	}

}
