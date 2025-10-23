package sangfor

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"os"
	"regexp"

	"github.com/zhujintao/kit-go/ssh"
	"resty.dev/v3"
)

var publickey = "AF807BF54F973138A8DEB0434A5D8636F636CB259F97548FA4C8838FEAE733B2FE823135A893E5D640604BEDE6A1CCCA5AFD7F4417028A5CF5FE50FC6E40D801048945E1A0EA8B85E34DEC13D12451FBF7887B120A5AB67FB42AB75C31B9750F49088AA7481DA99E3F56B835E54FE801A4813177AC7B7577360FC96B11F9F953"

type client struct {
	user string
	cf   string
	http *resty.Client
}

type OprAction string

var (
	OprActionList   OprAction = "list"
	OprActionLogoff OprAction = "logoff"
)

type Bandwidth struct {
	Ip     string
	Up     float64
	Down   float64
	Total  float64
	App    string
	Line   int
	Detail struct {
		Data []*Bandwidth
	}
}

func encryptCF(s string) string {

	var v string
	v = s
	for range 3 {
		hasher := md5.New()
		hasher.Write([]byte(v))
		v = hex.EncodeToString(hasher.Sum(nil)[:])
	}
	return v

}

type requestBody map[string]any

func encryptPwd(s string) string {

	n, _ := (&big.Int{}).SetString(publickey, 16)
	e, _ := (&big.Int{}).SetString("10001", 16)

	key := &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}

	b, err := rsa.EncryptPKCS1v15(rand.Reader, key, []byte(s))
	if err != nil {
		fmt.Println(err)
		return ""
	}
	return hex.EncodeToString(b)
}
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
	os.Setenv("GODEBUG", "tlsrsakex=1")
	c := resty.New().SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS10}).SetContentLength(true).SetBaseURL(url)
	if sshcli != nil {
		t, _ := c.HTTPTransport()
		t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return sshcli.Dial(network, addr)
		}
	}
	pwd := encryptPwd(password)
	req := `{"opr":"login","data":{"user":"admin","pwd":"` + pwd + `"}}`
	var result struct {
		Ok  bool   `json:"success"`
		Msg string `json:"ErrorMsg"`
	}

	c.R().SetResult(&result).SetForceResponseContentType("application/json").SetBody(req).Post("/cgi-bin/login.cgi")
	if !result.Ok {
		return nil, fmt.Errorf("login failed, %s", result.Msg)
	}
	//get cf
	r, _ := c.R().Get("/framework.php")
	ck := regexp.MustCompile(`this.SF.ck = "(.*)";`).FindStringSubmatch(r.String())
	if ck == nil {
		return nil, fmt.Errorf("login failed, %s", "ck not found.")
	}

	cf := encryptCF(ck[1])
	return &client{http: c, user: user, cf: cf}, nil

}

func (c *client) Logout() {
	c.http.R().SetBody(c.buildBody(OprActionLogoff).build()).Post("/cgi-bin/login.cgi")
}

func (c *client) buildBody(op OprAction) *requestBody {

	return &requestBody{"opr": op, "cf": c.cf}

}
func (b *requestBody) buildBody(k string, v any) *requestBody {
	(*b)[k] = v
	return b
}
func (b *requestBody) build() string {

	ss, _ := json.Marshal(b)
	return string(ss)
}

func (c *client) GetStatusMsg() {

	req := c.buildBody(OprActionList).build()
	r, _ := c.http.R().SetBody(req).Post("/cgi-bin/statusmsg.cgi")
	fmt.Println(r)
}

func (c *client) jsonBody(s string) string {
	return fmt.Sprintf(s, c.cf)
}

func (c *client) FetchBandwidth() []*Bandwidth {
	var result struct {
		Success bool
		Data    []*Bandwidth
	}
	c.http.R().SetResult(&result).SetForceResponseContentType("application/json").SetBody(c.jsonBody(`{"opr":"list","filter":{"first":1,"time":0,"unit":"bytes"},"cf":"%s"}`)).Post("/cgi-bin/rtflux.cgi")
	return result.Data
}
