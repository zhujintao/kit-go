package loki

import (
	"bufio"
	"bytes"
	"context"
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
	dateFormat   = "2006-01-02T15:04:05.000"
)

var userAgent = fmt.Sprintf("promtail/%s", build.Version)

type loki struct {
	lokiURL  string
	client   *http.Client
	labels   model.LabelSet
	args     []any
	tenantID string
	timeout  time.Duration
}

// Set Loki Tenant ID
func (l *loki) SetTenantID(id string) *loki {

	l.tenantID = id
	return l
}

// Set a Label Job
func (l *loki) SetJob(value string) *loki {
	l.labels[model.LabelName("job")] = model.LabelValue(value)
	return l
}

// Set a Label SetNamespace
func (l *loki) SetNamespaceEnv(name string) *loki {

	value := os.ExpandEnv(fmt.Sprintf("${%s}", name))
	if value != "" {
		l.labels[model.LabelName("namespace")] = model.LabelValue(value)
	}

	return l
}

// Add Labels AddLabels(k,v,k,v)
func (l *loki) AddLabels(labels ...any) *loki {

	r := slog.NewRecord(time.Now(), 0, "", 0)
	r.Add(labels...)
	r.Attrs(func(a slog.Attr) bool {

		l.labels[model.LabelName(a.Key)] = model.LabelValue(a.Value.String())

		return true
	})

	return l
}

// URL loki push address
//
// env LOKI_PUSH_URL
func NewLoki(URL ...string) *loki {
	url := os.ExpandEnv("${LOKI_PUSH_URL}")
	if len(URL) == 1 {
		url = URL[0]
	}

	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println(err)

	}
	l := &loki{
		lokiURL: url,
		labels:  make(model.LabelSet),
		timeout: time.Second * 1,
	}
	l.labels[model.LabelName("hostname")] = model.LabelValue(hostname)
	var clientURL flagext.URLValue
	clientURL.Set(l.lokiURL)

	if !strings.Contains(clientURL.Path, postPath) {
		clientURL.Path = postPath
		l.lokiURL = clientURL.String()
	}

	cfg := config.DefaultHTTPClientConfig
	l.client, err = config.NewClientFromConfig(cfg, "promtail", config.WithHTTP2Disabled())
	if err != nil {
		fmt.Println(err)
	}
	l.client.Timeout = l.timeout

	exLbs := os.ExpandEnv("${LOKI_EXTERNAL_LABELS}")
	if exLbs != "" {

		lbs := strings.Split(exLbs, ",")
		for _, lb := range lbs {

			label := strings.Split(lb, "=")
			if len(label) == 2 {
				l.labels[model.LabelName(label[0])] = model.LabelValue(fmt.Sprintf("%v", label[1]))
			}
		}
	}

	return l
}

func appendAttr(line *buffer.Buffer, k, v string) {
	line.WriteString(" ")
	line.WriteString(k)
	line.WriteByte('=')
	line.WriteString(v)
}

func (l *loki) Log(t time.Time, level string, message string, args ...any) {
	if l.lokiURL == postPath {
		fmt.Println("LOKI_PUSH_URL must be defined")
		return
	}
	var line buffer.Buffer = *buffer.New()

	r := slog.NewRecord(time.Now(), 0, message, 0)
	line.WriteString(r.Time.Format(dateFormat))
	line.WriteString(" ")
	line.WriteString(level)
	line.WriteString(" ")
	line.WriteString(r.Message)
	l.labels[model.LabelName("level")] = model.LabelValue(level)

	r.Add(args...)
	r.Add(l.args...)

	r.Attrs(func(a slog.Attr) bool {
		l.labels[model.LabelName(a.Key)] = model.LabelValue(a.Value.String())
		if strings.ToLower(a.Key) == "job" {
			return true
		}
		appendAttr(&line, a.Key, a.Value.String())
		return true
	})

	l.send(r.Time, line.String())
}

func (l *loki) Send(message string, args ...any) {
	if l.lokiURL == postPath {
		fmt.Println("LOKI_PUSH_URL must be defined")
		return
	}
	var line buffer.Buffer = *buffer.New()

	r := slog.NewRecord(time.Now().Local(), 0, message, 0)
	line.WriteString(message)
	r.Add(args...)
	r.Add(l.args...)
	r.Attrs(func(a slog.Attr) bool {
		l.labels[model.LabelName(a.Key)] = model.LabelValue(a.Value.String())
		if strings.ToLower(a.Key) == "job" {
			return true
		}
		appendAttr(&line, a.Key, a.Value.String())
		return true
	})

	l.send(r.Time, line.String())

}
func (l *loki) send(t time.Time, msg string) {
	stream := &logproto.Stream{Labels: l.labels.String(), Entries: []logproto.Entry{{Timestamp: t, Line: msg}}}
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
	ctx, cancel := context.WithTimeout(context.Background(), l.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", l.lokiURL, bytes.NewReader(buf))
	if err != nil {
		fmt.Println(err)
		return
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", userAgent)
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
