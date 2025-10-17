package weyos

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/zhujintao/kit-go/ssh"
	"resty.dev/v3"
)

type client struct {
	*resty.Client
}
type Opt string

var (
	Modify Opt = "mod"
	Add    Opt = "add"
	Delete Opt = "del"
)

func Login(url, user, password string, viaSsh ...string) *client {

	var sshConn *ssh.Client
	var err error
	if len(viaSsh) == 3 {
		sshConn, err = ssh.NewConn(viaSsh[0], viaSsh[1], viaSsh[2])
		if err != nil {
			fmt.Println(err)
			return nil
		}

		ssh.Ping(context.Background(), sshConn, 2)
	}

	c := resty.New().SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}).SetContentLength(true).SetBaseURL(url)
	if len(viaSsh) == 3 {
		t, _ := c.HTTPTransport()
		t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return sshConn.Dial(network, addr)
		}
	}

	r, err := c.R().SetFormData(map[string]string{"user": user, "password": password}).Post("/login.cgi")
	if err != nil {
		return nil
	}

	if len(r.Cookies()) == 0 {
		fmt.Println("login failed")
		return nil

	}
	// reset cookie
	c = resty.New().SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}).SetContentLength(true).SetBaseURL(c.BaseURL())
	if len(viaSsh) == 3 {
		t, _ := c.HTTPTransport()
		t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return sshConn.Dial(network, addr)
		}
	}

	cookie := r.Cookies()[0]
	s := fmt.Sprintf("%s=%s; Path=%s;", cookie.Name, cookie.Value, cookie.Path)
	c.SetHeader("Cookie", s)

	return &client{c}

}

func (c *client) GetRuleDNat() {
	var result struct {
		Rule string `json:"portforward"`
	}

	r, err := c.R().SetResult(&result).SetForceResponseContentType("application/json;charset=gb2312").Get("/nat_base.data")
	fmt.Println(r, err)
	fmt.Println(result)

}
func (c *client) SetRuleDnat() {
	//c.R().SetQueryParam("portforward", nat.DNatString(true)).SetQueryParam("exec_service", "firewall-restart").Get("/nat_base.asp")

}

func (c *client) GetRuleSNat() {
	var result struct {
		Rule string `json:"all"`
	}
	c.R().SetResult(&result).SetForceResponseContentType("application/json;charset=gb2312").Get("/mrprot.data")

}
func (c *client) SetRuleSNat(o Opt) {

	//update
	//rename need add old_name param
	//r, err = client.R().SetQueryParams(a.Map(true)).SetQueryParam("old_name", old_name).SetQueryParam("opt", "mod").Get("/mrprot.asp")
	//fmt.Println(r, err)
	//del
	//r, err = client.R().SetQueryParam("name", a.Name).SetQueryParam("opt", "del").Get("/mrprot.asp")

	//add
	//r, err = client.R().SetQueryParams(a.Map(true)).SetQueryParam("opt", "add").Get("/mrprot.asp")

}
