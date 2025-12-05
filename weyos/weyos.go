package weyos

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/zhujintao/kit-go/ssh"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"resty.dev/v3"
)

var protocols map[int]string = map[int]string{}

type client struct {
	http *resty.Client
	*nat
}

type sNatOpt struct {
	*SNat
	Id      string   `json:"old_name,omitempty"`
	Action  string   `json:"opt"`
	DelList []string `json:"del_list,omitempty"`
}
type dNatOpt struct {
	*DNat
	Id      string   `json:"old_name,omitempty"`
	Action  string   `json:"opt"`
	DelList []string `json:"del_list,omitempty"`
}

type respResult struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

// support FBM-568G 24.03.28V (86409) wayos4.0
func NewClient(url, password string, viaSsh ...string) (*client, error) {
	var sshaddr, sshuser, sshpasswd, privatekeyFile string
	if len(viaSsh) >= 3 {
		sshaddr = viaSsh[0]
		sshuser = viaSsh[1]
		sshpasswd = viaSsh[2]
	}
	if len(viaSsh) == 4 {
		privatekeyFile = viaSsh[3]
	}

	sshcli, err := ssh.NewConn(sshaddr, sshuser, sshpasswd, privatekeyFile)
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

	var token struct {
		Data struct {
			CsrfToken string `json:"csrfprotect"`
		} `json:"data"`
	}

	c.R().SetResult(&token).SetForceResponseContentType("application/json;charset=gb2312").Get("/csrfprotect.data")
	body := fmt.Sprintf(`{"csrfprotect":"%s","password":"%s"}`, token.Data.CsrfToken, encryption(password))
	r, err := c.R().SetContentType("application/json;charset=UTF-8").SetBody(body).Post("/log_in.cgi")

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
	// load /etc/protocols
	parserProtocol(protocols)
	return &client{http: c, nat: &nat{}}, nil
}

func (cli *client) FetchSnat() *nat {

	var result struct {
		respResult
		Data struct {
			All []SNat `json:"all"`
		} `json:"data"`
	}
	cli.http.R().SetResult(&result).SetForceResponseContentType("application/json;charset=gb2312").Get("/mrprot.data")
	if result.Error != "" {
		fmt.Println("fetch snat error:", result.Error)
		return nil
	}
	cli.sNats = result.Data.All
	//fmt.Println("fetch snat record:", len(result.Data.All))
	return cli.nat
}

func (cli *client) FetchDnat() *nat {

	var result struct {
		respResult
		Data struct {
			All []DNat `json:"portforward"`
		} `json:"data"`
	}
	cli.http.R().SetResult(&result).SetForceResponseContentType("application/json;charset=gb2312").Get("/nat_base.data")
	if result.Error != "" {
		fmt.Println("fetch snat error:", result.Error)
		return nil
	}
	cli.dNats = result.Data.All
	return cli.nat
}

func (cli *client) SetDnat(s *DNat) {

	if cli.FetchDnat().FindDNat(s.Name) == nil {
		fmt.Println("recod not exist, rename use Rename", s.Name)
		return
	}

	opt := dNatOpt{
		DNat:   s,
		Action: "mod",
	}
	str := cli.marshalBody(opt)
	fmt.Println(str)
	var r respResult
	cli.http.R().SetResult(&r).
		SetContentType("application/json;charset=UTF-8").
		SetForceResponseContentType("application/json;charset=gb2312").
		SetBody(str).
		Post("/nat_base2.asp")
	fmt.Println(r)
	cli.FetchDnat()

}

func (cli *client) SetSnat(s *SNat) {

	if cli.FetchSnat().FindSNat(s.Name) == nil {
		fmt.Println("recod not exist, rename use Rename", s.Name)
		return
	}

	opt := sNatOpt{
		SNat:   s,
		Action: "mod",
	}
	str := cli.marshalBody(opt)
	fmt.Println(str)
	var r respResult
	cli.http.R().SetResult(&r).
		SetContentType("application/json;charset=UTF-8").
		SetForceResponseContentType("application/json;charset=gb2312").
		SetBody(str).
		Post("/mrprot.asp")
	fmt.Println(r)
	cli.FetchSnat()

}

func (cli *client) AddDnat(d *DNat) {

	if d.Name == "" {
		fmt.Println("dnat name is empty")
		return
	}

	if d.LanIp == "" {
		fmt.Println("dnat lanip is empty")
		return
	}
	if d.LanPorts == "" {
		fmt.Println("dnat lanports is empty")
		return
	}

	if d.SrcPorts == "" {
		fmt.Println("dnat srcports is empty")
		return
	}
	opt := dNatOpt{
		DNat:   d,
		Action: "add",
	}
	str := cli.marshalBody(opt)
	fmt.Println(str)
	var r respResult
	cli.http.R().SetResult(&r).
		SetContentType("application/json;charset=UTF-8").
		SetForceResponseContentType("application/json;charset=gb2312").
		SetBody(str).
		Post("/nat_base2.asp")
	fmt.Println(r)
	cli.FetchSnat()
}

func (cli *client) AddSnat(s *SNat) {

	if s.Name == "" {
		fmt.Println("snat name is empty")
		return
	}
	if s.Priority == 0 {
		s.Priority = 30000
	}
	if s.Schedule == "" {
		s.Schedule = "OFF"
	}
	if s.Failovered == "" {
		s.Failovered = "1"
	}
	if s.Thdtype == "" {
		s.Thdtype = "0"
	}

	opt := sNatOpt{
		SNat:   s,
		Action: "add",
	}
	str := cli.marshalBody(opt)
	fmt.Println(str)
	var r respResult
	cli.http.R().SetResult(&r).
		SetContentType("application/json;charset=UTF-8").
		SetForceResponseContentType("application/json;charset=gb2312").
		SetBody(str).
		Post("/mrprot.asp")
	fmt.Println(r)
	cli.FetchSnat()
}

func (cli *client) DelDnat(s ...string) {
	opt := dNatOpt{
		DelList: s,
		Action:  "del",
	}
	str := cli.marshalBody(opt)
	fmt.Println(str)
	var r respResult
	cli.http.R().SetResult(&r).
		SetContentType("application/json;charset=UTF-8").
		SetForceResponseContentType("application/json;charset=gb2312").
		SetBody(str).
		Post("/nat_base2.asp")

	fmt.Println(r)
	cli.FetchDnat()
}

func (cli *client) DelSnat(s ...string) {
	opt := sNatOpt{
		DelList: s,
		Action:  "del",
	}
	str := cli.marshalBody(opt)
	fmt.Println(str)
	var r respResult
	cli.http.R().SetResult(&r).
		SetContentType("application/json;charset=UTF-8").
		SetForceResponseContentType("application/json;charset=gb2312").
		SetBody(str).
		Post("/mrprot.asp")

	fmt.Println(r)
	cli.FetchSnat()
}

func (cli *client) RenameDnat(oldname, newname string) {

	s := cli.FetchDnat().FindDNat(oldname)
	if s == nil {
		fmt.Println("recod not exist", oldname)
		return
	}
	if cli.FindDNat(newname) != nil {
		fmt.Println("recod name exist", newname)
		return
	}
	s.Name = newname
	opt := dNatOpt{
		DNat:   s,
		Id:     oldname,
		Action: "mod",
	}
	str := cli.marshalBody(opt)
	var r respResult
	cli.http.R().SetResult(&r).
		SetContentType("application/json;charset=UTF-8").
		SetForceResponseContentType("application/json;charset=gb2312").
		SetBody(str).
		Post("/nat_base2.asp")

	fmt.Println(r)
	cli.FetchDnat()

}

func (cli *client) RenameSnat(oldname, newname string) {

	s := cli.FetchSnat().FindSNat(oldname)
	if s == nil {
		fmt.Println("recod not exist", oldname)
		return
	}
	if cli.FindSNat(newname) != nil {
		fmt.Println("recod name exist", newname)
		return
	}
	s.Name = newname
	opt := sNatOpt{
		SNat:   s,
		Id:     oldname,
		Action: "mod",
	}
	str := cli.marshalBody(opt)
	var r respResult
	cli.http.R().SetResult(&r).
		SetContentType("application/json;charset=UTF-8").
		SetForceResponseContentType("application/json;charset=gb2312").
		SetBody(str).
		Post("/mrprot.asp")

	fmt.Println(r)
	cli.FetchSnat()

}

func (cli *client) marshalBody(opt any) string {
	b := bytes.NewBuffer(nil)
	enc := json.NewEncoder(b)
	enc.SetEscapeHTML(false)
	enc.Encode(opt)
	str := StringGB2312(b.String())
	return str
}

func encryption(input string) string {

	const chars = "ABCDEFGHJKMNPQRSTWXYZabcdefhijkmnprstwxyz2345678"
	b := make([]byte, 10)
	for i := range b {
		b[i] = chars[rand.IntN(len(chars))]
	}

	passwd := string(b)
	for i := 0; i < len(input); i++ {
		charCode := int(input[i])
		if i%2 == 0 {
			charCode += 2
		} else {
			charCode -= 2
		}
		passwd += string(rune(charCode))
	}
	return passwd
}

type trafficDetail struct {
	Port       int     `json:"iport"`
	RemoteIp   string  `json:"fip6"`
	RemotePort int     `json:"fport"`
	Protocol   int     `json:"prot"`
	OnlineTime float64 `json:"tm"`
	Upload     float64 `json:"z0"`
	Download   float64 `json:"z1"`
	Direction  int     `json:"dir"`
	NatIp      string  `json:"nip6"`
	NatIface   int     `json:"mid"`
	NatPort    int     `json:"nport"`
	N          string  `json:"n"`
	Rule       string  `json:"mr_rule"`
}

func (t *trafficDetail) PortStr() string {
	return fmt.Sprintf("%d", t.Port)
}
func (t *trafficDetail) RemotePortStr() string {
	return fmt.Sprintf("%d", t.RemotePort)
}
func (t *trafficDetail) NatIfaceStr() string {
	if t.NatIface == 0 {
		return "LAN"
	}
	return fmt.Sprintf("WAN%d", t.NatIface)
}
func (t *trafficDetail) DirectionStr() string {

	if t.Direction == 1 {
		return "<-"
	}
	return "->"
}

func (t *trafficDetail) ProtocolStr() string {
	return protocols[t.Protocol]
}

func (t *traffic) OutIfaceStr() string {
	if t.Outiface == 65535 {
		return ""
	}
	return fmt.Sprintf("WAN%d", t.Outiface)
}

type traffic struct {
	Name              string  `json:"n_user"`
	OnlineTime        int     `json:"ctm"`
	Mac               string  `json:"mac"`
	Ip                string  `json:"ip6"`
	ConnectCount      int32   `json:"ct"`
	SpeedUpload       float64 `json:"qup"`
	SpeedDownload     float64 `json:"qdw"`
	DataUploadTotal   float64 `json:"zup"`
	DataDownloadTotal float64 `json:"zdw"`
	Outiface          int     `json:"host_wan_id"`
	Detail            []trafficDetail
	//DataUpload        float64 `json:"up"`
	//DataDownload      float64 `json:"dw"`
}

func (cli *client) GetTrafficstats() []traffic {

	var result struct {
		respResult
		Data struct {
			All []traffic `json:"list"`
		} `json:"data"`
	}

	cli.http.R().SetResult(&result).SetForceResponseContentType("application/json;charset=gb2312").Get("/hilist.data")
	for i, a := range result.Data.All {

		var resultDetail struct {
			respResult
			Data []trafficDetail `json:"data"`
		}
		_, err := cli.http.R().SetResult(&resultDetail).SetQueryParam("hi", a.Ip).SetForceResponseContentType("application/json;charset=gb2312").Get("hictlistxx2.data")
		result.Data.All[i].Detail = resultDetail.Data
		fmt.Println(err)

	}

	return result.Data.All
}

func StringGB2312(s string) string {

	gb2312 := simplifiedchinese.GB18030.NewEncoder()
	sgb, _, err := transform.String(gb2312, s)
	if err != nil {
		fmt.Println("GK2312 error", err)
		return ""
	}
	return sgb
}
func parserProtocol(t map[int]string) {

	f, err := os.Open("/etc/protocols")
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {

		line := scanner.Text()
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		field := strings.Fields(line)
		if len(field) < 3 {
			continue
		}
		i, err := strconv.Atoi(field[1])
		if err != nil {
			continue
		}
		t[i] = field[0]

	}

}
