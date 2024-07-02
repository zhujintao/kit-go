package loki

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/grafana/loki/v3/pkg/util/build"

	"github.com/golang/snappy"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/zhujintao/kit-go/utils/buffer"
)

const (
	contentType  = "application/x-protobuf"
	maxErrMsgLen = 1024
	postPath     = "/loki/api/v1/push"
)

var UserAgent = fmt.Sprintf("promtail/%s", build.Version)

type loki struct {
	lokiURL  string
	client   *http.Client
	labels   model.LabelSet
	tenantID string
}

func (l *loki) SetTenantID(id string) *loki {

	l.tenantID = id
	return l
}

func NewLoki(URL string) *loki {
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println(err)
		return nil
	}
	l := &loki{
		lokiURL: URL,
		labels:  make(model.LabelSet),
	}
	l.labels[model.LabelName("hostname")] = model.LabelValue(hostname)
	var clientURL flagext.URLValue
	err = clientURL.Set(l.lokiURL)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	if !strings.Contains(clientURL.Path, postPath) {
		clientURL.Path = postPath
		l.lokiURL = clientURL.String()
	}

	cfg := config.DefaultHTTPClientConfig
	l.client, err = config.NewClientFromConfig(cfg, "loki-cli", config.WithHTTP2Disabled())
	if err != nil {
		fmt.Println(err)
		return nil
	}

	l.client.Timeout = 0

	return l
}

func appendAttr(line *buffer.Buffer, k, v string) {
	line.WriteString(" ")
	line.WriteString(k)
	line.WriteByte('=')
	line.WriteString(v)
}

func (l *loki) Log(t time.Time, level int, message string, args ...any)
func (l *loki) Send(message string, args ...any) {

	var line buffer.Buffer = *buffer.New()
	r := slog.NewRecord(time.Now(), 0, message, 0)
	appendAttr(&line, "msg", r.Message)
	r.Add(args...)
	r.Attrs(func(a slog.Attr) bool {
		l.labels[model.LabelName(a.Key)] = model.LabelValue(a.Value.String())
		appendAttr(&line, a.Key, a.Value.String())
		return true
	})

	stream := &logproto.Stream{Labels: l.labels.String(), Entries: []logproto.Entry{{Timestamp: r.Time, Line: line.String()}}}
	preq := logproto.PushRequest{
		Streams: make([]logproto.Stream, 0),
	}
	preq.Streams = append(preq.Streams, *stream)
	buf, err := proto.Marshal(&preq)
	if err != nil {
		fmt.Println(err)
		return
	}
	buf = snappy.Encode(nil, buf)
	req, err := http.NewRequest("POST", l.lokiURL, bytes.NewReader(buf))
	if err != nil {
		fmt.Println(err)
		return
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", UserAgent)
	if l.tenantID != "" {
		req.Header.Set("X-Scope-OrgID", l.tenantID)
	}
	resp, err := l.client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		scanner := bufio.NewScanner(io.LimitReader(resp.Body, maxErrMsgLen))
		line := ""
		if scanner.Scan() {
			line = scanner.Text()
		}
		fmt.Printf("server returned HTTP status %s (%d): %s", resp.Status, resp.StatusCode, line)
	}
}
