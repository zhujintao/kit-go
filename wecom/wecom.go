package wecom

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	restful "github.com/emicklei/go-restful/v3"

	"resty.dev/v3"
)

type result struct {
	Errcode int
	Errmsg  string
}
type client struct {
	http        *resty.Client
	corpid      string
	accessToken string
	ctx         context.Context
	web         *restful.WebService
}

type msgContent struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   CDATA    `xml:"ToUserName"`
	FromUserName CDATA    `xml:"FromUserName"`
	CreateTime   int64
	MsgType      CDATA `xml:"MsgType"`
	Content      string
	MsgId        uint64
	AgentID      uint64

	Event     string  `xml:"Event,omitempty"`
	Latitude  float64 `xml:"Latitude,omitempty"`
	Longitude float64 `xml:"Longitude,omitempty"`
	Precision uint64  `xml:"Precision,omitempty"`
	AppType   string  `xml:"AppType,omitempty"`
}

func (m *msgContent) toString(crypter *WXBizMsgCrypt) []byte {

	if m.CreateTime == 0 {
		m.CreateTime = time.Now().Unix()
	}

	timestamp := fmt.Sprintf("%d", m.CreateTime)
	msgbyte, _ := xml.Marshal(m)
	fmt.Println(string(msgbyte))
	b, _ := crypter.EncryptMsg(string(msgbyte), timestamp, timestamp)
	return b

}

func (c *client) Message(token, encoding_aeskey string) {

	crypter := NewWXBizMsgCrypt(token, encoding_aeskey, c.corpid, XmlType)

	c.web.Route(c.web.GET("/message").To(func(r *restful.Request, w *restful.Response) {
		msg_signature := r.QueryParameter("msg_signature")
		timestamp := r.QueryParameter("timestamp")
		nonce := r.QueryParameter("nonce")
		echostr := r.QueryParameter("echostr")

		b, err := crypter.VerifyURL(msg_signature, timestamp, nonce, echostr)
		if err != nil {
			fmt.Println(err)
			return
		}
		w.Write(b)

	}))

	c.web.Route(c.web.POST("/message").To(func(r *restful.Request, w *restful.Response) {
		msg_signature := r.QueryParameter("msg_signature")
		timestamp := r.QueryParameter("timestamp")
		nonce := r.QueryParameter("nonce")
		msgBody, _ := io.ReadAll(r.Request.Body)
		msgbyte, decryptErr := crypter.DecryptMsg(msg_signature, timestamp, nonce, msgBody)
		if decryptErr != nil {
			fmt.Println(decryptErr)
			return
		}

		var recvMsg msgContent
		xml.Unmarshal(msgbyte, &recvMsg)

		aaa := time.Now().Unix()

		var sendMsg msgContent
		sendMsg.ToUserName = recvMsg.FromUserName
		sendMsg.FromUserName = recvMsg.ToUserName
		sendMsg.CreateTime = aaa
		sendMsg.MsgId = recvMsg.MsgId
		sendMsg.Content = "Coming Soon!"
		sendMsg.MsgType = CDATA{Value: "text"}
		mm, _ := xml.Marshal(sendMsg)
		b, _ := crypter.EncryptMsg(string(mm), fmt.Sprintf("%d", aaa), fmt.Sprintf("%d", aaa))
		w.Write(b)

	}))

}

func (c *client) BuildAuth(appid int, redirect_uri string) string {
	u := &url.URL{
		Scheme:   "https",
		Host:     "open.weixin.qq.com",
		Path:     "/connect/oauth2/authorize",
		Fragment: "#wechat_redirect",
	}
	q := u.Query()
	q.Add("appid", c.corpid)
	q.Add("redirect_uri", redirect_uri)
	q.Add("response_type", "code")
	q.Add("scope", "snsapi_privateinfo") //snsapi_base,snsapi_privateinfo
	q.Add("state", "STATE")
	q.Add("agentid", fmt.Sprintf("%d", appid))
	u.RawQuery = q.Encode()

	return u.String()

}

func (c *client) Serve() {

	web := c.web

	//login
	web.Route(web.GET("/login").To(func(r *restful.Request, w *restful.Response) {
		code := r.QueryParameter("code")
		var user struct {
			User_ticket string
			Expires_in  int
			Userid      string
		}
		c.http.R().SetResult(&user).SetQueryParam("code", code).SetQueryParam("access_token", c.accessToken).Get("/cgi-bin/auth/getuserinfo")
		if user.User_ticket == "" {
			fmt.Println("user ticket not get.")
			return
		}

		var userdetail any
		c.http.R().SetResult(&userdetail).SetBody(`{ "user_ticket": "`+user.User_ticket+`"}`).SetQueryParam("access_token", c.accessToken).Post("/cgi-bin/auth/getuserdetail")
		var userb any
		c.http.R().SetResult(&userb).SetQueryParam("userid", user.Userid).SetQueryParam("access_token", c.accessToken).Get("/cgi-bin/user/get")
		fmt.Println(userb)
		//var depart any
		//c.http.R().SetResult(&depart).SetQueryParam("id", "5").SetQueryParam("access_token", c.accessToken).Get("/cgi-bin/department/get?access_token=ACCESS_TOKEN&id=ID")
		//fmt.Println(depart)

		res, _ := c.http.R().SetQueryParam("access_token", c.accessToken).Get("/cgi-bin/department/list")
		fmt.Println(res)
	}))

	restful.Add(web)
	http.ListenAndServeTLS(":443", "cert", "key", nil)

}

func New(corpid, corpsecret string) *client {

	client := &client{ctx: context.Background(),
		http:   resty.New().SetBaseURL("https://qyapi.weixin.qq.com"),
		web:    &restful.WebService{},
		corpid: corpid,
	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	go client.refreshToken(corpid, corpsecret, &wg)
	wg.Wait()
	return client
}
func (c *client) Close() {
	c.ctx.Done()
}

func (c *client) refreshToken(corpid, corpsecret string, wg *sync.WaitGroup) {

	ticker := time.NewTicker(time.Duration(1))
	defer ticker.Stop()

	for {

		select {

		case <-c.ctx.Done():
			return
		case <-ticker.C:
			var result struct {
				AccessToken string `json:"access_token"`
				ExpiresIn   int    `json:"expires_in"`
			}
			c.http.R().SetResult(&result).SetQueryParam("corpid", corpid).SetQueryParam("corpsecret", corpsecret).Get("/cgi-bin/gettoken")
			if result.AccessToken == "" {
				fmt.Println("token refesh fail")
				return

			}
			c.accessToken = result.AccessToken
			wg.Done()
			fmt.Println("token refesh", result)
			ticker.Reset(time.Second * time.Duration(result.ExpiresIn-60))
		}
	}
}
func (c *client) R() *resty.Request {
	return c.http.R().SetQueryParam("access_token", c.accessToken)
}
func (c *client) SetWorkShowTemplate(json string) {
	var resut result
	c.R().SetResult(&resut).SetBody(json).Post("/cgi-bin/agent/set_workbench_template")
	fmt.Println(resut)
}
func (c *client) SetMenu(appid int, json string) {
	var resut result
	c.R().SetResult(&resut).SetQueryParam("agentid", fmt.Sprintf("%d", appid)).SetBody(json).Post("/cgi-bin/menu/create")
	fmt.Println(resut)
}
