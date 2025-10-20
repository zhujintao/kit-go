package weyos

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/zhujintao/kit-go/ssh"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"resty.dev/v3"
)

type client struct {
	user string
	http *resty.Client
	*nat
}

type traffic struct {
	Name          string `json:"n"`
	Mac           string `json:"mac"`
	Ip            string
	IpUint        uint32  `json:"ip"`
	OnlineTime    int     `json:"ctm"`
	DataUpload    float64 `json:"zup"`
	DataDownload  float64 `json:"zdw"`
	SpeedUpload   float64 `json:"qup"`
	SpeedDownload float64 `json:"qdw"`
	ConnectCount  int32   `json:"ct"`
	Detail        []*detail
}

type detail struct {
	RemoteIp     string
	RemoteIpUint uint32 `json:"fip"`
	OutIp        string
	OutIpUint    uint32  `json:"nip"`
	RemotePort   int     `json:"fport"`
	LocalPort    int     `json:"iport"`
	OutPort      int     `json:"nport"`
	Port         int     `json:"port"`
	OnlineTime   float64 `json:"tm"`
	Upload       float64 `json:"z0"`
	Download     float64 `json:"z1"`
	Iface        int     `json:"mid"`
	Direction    int     `json:"dir"`
}

func (d *detail) IfaceStr() string {
	if d.Iface == 0 {
		return "LAN"
	}
	return fmt.Sprintf("WAN%d", d.Iface)
}

func (d *detail) DirectionStr() string {
	if d.Direction == 1 {
		return "<-"
	}
	if d.Direction == 0 {
		return "->"
	}
	return fmt.Sprintf("%d", d.Direction)
}

// Add,Modify,Delete
type SNatOptAction string
type LogCategory string

var (
	OptModify SNatOptAction = "mod"
	OptAdd    SNatOptAction = "add"
	OptDelete SNatOptAction = "del"

	LogNat  LogCategory = "mrzc"
	LogArp  LogCategory = "arp"
	LogSys  LogCategory = "message"
	LogWan  LogCategory = "wan"
	LogDdos LogCategory = "ddos"
)

func Login(url, user, password string, viaSsh ...string) (*client, error) {

	var sshaddr, sshuser, sshpasswd string
	if len(viaSsh) == 3 {
		sshaddr = viaSsh[0]
		sshuser = viaSsh[1]
		sshpasswd = viaSsh[2]
	}
	sshcli, err := ssh.NewConn(sshaddr, sshuser, sshpasswd)
	if err != nil {
		fmt.Println("skip viaSsh:", err.Error())
	}

	sshcli.SendHello(context.Background())
	c := resty.New().SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}).SetContentLength(true).SetBaseURL(url)
	if sshcli != nil {
		t, _ := c.HTTPTransport()
		t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return sshcli.Dial(network, addr)
		}
	}

	r, err := c.R().SetFormData(map[string]string{"user": user, "password": password}).Post("/login.cgi")
	if err != nil {
		return nil, err
	}

	if len(r.Cookies()) == 0 {
		return nil, fmt.Errorf("login failed")
	}
	// reset cookie
	c = resty.New().SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}).SetContentLength(true).SetBaseURL(c.BaseURL())
	if sshcli != nil {
		t, _ := c.HTTPTransport()
		t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return sshcli.Dial(network, addr)
		}
	}

	cookie := r.Cookies()[0]
	s := fmt.Sprintf("%s=%s; Path=%s;", cookie.Name, cookie.Value, cookie.Path)
	c.SetHeader("Cookie", s)

	c.AddContentTypeDecoder("application/json;charset=gb2312", func(r io.Reader, v any) error {
		b, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		read := transform.NewReader(strings.NewReader(string(b)), simplifiedchinese.GBK.NewDecoder())
		f := c.ContentTypeDecoders()["json"]
		f(read, v)
		return nil
	})
	return &client{user: user, http: c, nat: &nat{}}, nil

}

func (c *client) Logout() {
	c.http.R().SetQueryParam("user", c.user).Get("/logout.asp")
}

func (c *client) FetchRuleDNat() error {
	var result struct {
		Rule string `json:"portforward"`
	}
	c.http.R().SetResult(&result).SetForceResponseContentType("application/json;charset=gb2312").Get("/nat_base.data")
	c.DNatfrom(result.Rule)

	return nil

}

func (c *client) FetchRuleSNat() error {
	var result struct {
		Rule string `json:"all"`
	}
	c.http.R().SetResult(&result).SetForceResponseContentType("application/json;charset=gb2312").Get("/mrprot.data")
	c.SNatfrom(result.Rule)

	return nil
}
func (c *client) SetRuleDnat() error {
	c.http.R().SetQueryParam("portforward", c.DNatString(true)).SetQueryParam("exec_service", "firewall-restart").Get("/nat_base.asp")
	c.FetchRuleDNat()
	return nil
}

func (c *client) SetRuleSNat(entry *SNat, oldname string, action SNatOptAction) error {

	switch action {
	case OptAdd:
		c.http.R().SetQueryParams(entry.Map(true)).SetQueryParam("opt", string(action)).Get("/mrprot.asp")
	case OptModify:
		c.http.R().SetQueryParams(entry.Map(true)).SetQueryParam("old_name", oldname).SetQueryParam("opt", string(action)).Get("/mrprot.asp")
	case OptDelete:
		c.http.R().SetQueryParam("name", oldname).SetQueryParam("opt", string(action)).Get("/mrprot.asp")
	}
	c.FetchRuleSNat()

	return nil

}

func (c *client) GetTrafficstats() []*traffic {

	var result []*traffic

	c.http.R().SetResult(&result).SetForceResponseContentType("json").Get("/hilist.data")
	for _, v := range result {

		v.Ip = net.IPv4(byte(v.IpUint>>24), byte(v.IpUint>>16), byte(v.IpUint>>8), byte(v.IpUint)).String()
		var detail []*detail
		c.http.R().SetResult(&detail).SetForceResponseContentType("application/json").SetQueryParam("hi", fmt.Sprintf("%d", v.IpUint)).Get("/hictlistxx2.data")

		for _, d := range detail {
			d.RemoteIp = net.IPv4(byte(d.RemoteIpUint>>24), byte(d.RemoteIpUint>>16), byte(d.RemoteIpUint>>8), byte(d.RemoteIpUint)).String()
			d.OutIp = net.IPv4(byte(d.OutIpUint>>24), byte(d.OutIpUint>>16), byte(d.OutIpUint>>8), byte(d.OutIpUint)).String()
			v.Detail = append(v.Detail, d)
		}
	}

	return result
}

func (c *client) GetWanTraffic() {

	var result struct {
		Totup  float64 `json:"totup"`
		Totdw  float64 `json:"totdw"`
		Totupk float64 `json:"totupk"`
		Totdwk float64 `json:"totdwk"`
	}
	c.http.R().SetResult(&result).SetForceResponseContentType("application/json").SetQueryParam("iface", "all").Get("/wanll_tu.data")
	fmt.Println(result.Totdw, result.Totup, "\t", result.Totdwk, result.Totupk)

}

func (c *client) FetchLog(l LogCategory) {

	var resutl []string
	c.http.R().SetForceResponseContentType("application/json;charset=gb2312").SetResult(&resutl).SetQueryParam("id", string(l)).Get("/sys_log.data")
	for i, l := range resutl {
		fmt.Println(i, strings.Split(l, "<"))
	}
}
func (c *client) ClearLog(l LogCategory) {
	c.http.R().SetQueryParam("id", string(l)).Get("/sys_log.asp")
}

func (c *client) FetchRuleArp() {
	//var result struct {
	//}

	//http://192.168.1.1/arp_list.data?_=1760686526181

	//var static struct {
	//	Record string `json:"all"`
	//}
	//spr
	//http://192.168.1.1/arp_static.data
}
