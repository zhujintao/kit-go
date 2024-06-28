package loki

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"

	"github.com/grafana/dskit/flagext"
	"github.com/grafana/loki/pkg/push"
	"github.com/grafana/loki/v3/clients/pkg/promtail/api"
	"github.com/grafana/loki/v3/pkg/logproto"
)

type LokiHandler struct {
	url  string
	opts slog.HandlerOptions
	mu   *sync.Mutex
}

func NewLokiHandler(lokiUrl string, opts *slog.HandlerOptions) *LokiHandler {

	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	var clientURL flagext.URLValue
	clientURL.Set(lokiUrl)

	return &LokiHandler{
		url:  lokiUrl,
		opts: *opts,
		mu:   &sync.Mutex{},
	}

}

func (l *LokiHandler) Handle(_ context.Context, r slog.Record) error {

	lbs := make(model.LabelSet, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		lbs[model.LabelName(a.Key)] = model.LabelValue(a.Value.String())
		return true
	})
	batch := newBatch(0, api.Entry{Labels: lbs, Entry: logproto.Entry{Timestamp: r.Time, Line: r.Message}})
	l.mu.Lock()
	defer l.mu.Unlock()
	err := newClient(l.url).sendBatch("", batch)
	return err
}

func (l *LokiHandler) Enabled(_ context.Context, level slog.Level) bool {

	minLevel := slog.LevelInfo
	if l.opts.Level != nil {
		minLevel = l.opts.Level.Level()
	}
	return level >= minLevel
}

func (l *LokiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	fmt.Println("WithAttrs", attrs)
	return l
}

func (l *LokiHandler) WithGroup(name string) slog.Handler {
	fmt.Println("WithGroup", name)
	return l
}

const (
	contentType           = "application/x-protobuf"
	ReservedLabelTenantID = "__tenant_id__"
)

var UserAgent = fmt.Sprintf("promtail/%s", "v0.1")

type client struct {
	url    flagext.URLValue
	client *http.Client
}

func newClient(lokiUrl string) *client {

	cfg := config.HTTPClientConfig{}
	c, err := config.NewClientFromConfig(cfg, "promtail", config.WithHTTP2Disabled())
	if err != nil {

		return nil
	}
	var clientURL flagext.URLValue
	clientURL.Set(lokiUrl)

	return &client{client: c, url: clientURL}
}

func (c *client) log(labels model.LabelSet, line string) {

	req := logproto.PushRequest{Streams: make([]logproto.Stream, 0)}
	req.Streams = append(req.Streams, push.Stream{})
	buf, err := proto.Marshal(&req)
	if err != nil {
		fmt.Println(err)
		return
	}

	buf = snappy.Encode(nil, buf)
	c.send(context.Background(), "", buf)

}

func (c *client) Msg(line interface{}) {

	labels := labels.NewBuilder(nil)
	labels.Set("stream", "stdout")

	lbs := make(model.LabelSet)
	for _, l := range labels.Labels() {
		lbs[model.LabelName(l.Name)] = model.LabelValue(l.Value)
	}

	batch := newBatch(0, api.Entry{Labels: lbs, Entry: logproto.Entry{Timestamp: time.Now(), Line: fmt.Sprintf("%v", line)}})
	c.sendBatch("", batch)
}

func (c *client) sendBatch(tenantID string, batch *batch) error {

	buf, _, err := batch.encode()
	if err != nil {
		return err
	}
	_, err = c.send(context.Background(), tenantID, buf)
	if err != nil {
		return err
	}
	return nil
}

func (c *client) send(ctx context.Context, tenantID string, buf []byte) (int, error) {

	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", c.url.String(), bytes.NewReader(buf))
	if err != nil {
		return -1, err
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", UserAgent)

	if tenantID != "" {
		req.Header.Set("X-Scope-OrgID", tenantID)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {

		scanner := bufio.NewScanner(io.LimitReader(resp.Body, 1024))
		line := ""
		if scanner.Scan() {
			line = scanner.Text()
		}
		err = fmt.Errorf("server returned HTTP status %s (%d): %s", resp.Status, resp.StatusCode, line)
	}
	return resp.StatusCode, err
}
