package loki

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
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

const (
	contentType           = "application/x-protobuf"
	ReservedLabelTenantID = "__tenant_id__"
)

var UserAgent = fmt.Sprintf("promtail/%s", "v0.1")

type client struct {
	url    flagext.URLValue
	client *http.Client
}

func New(lokiUrl string) *client {

	cfg := config.HTTPClientConfig{}
	c, err := config.NewClientFromConfig(cfg, "promtail", config.WithHTTP2Disabled())
	if err != nil {

		return nil
	}
	var clientURL flagext.URLValue
	err = clientURL.Set(lokiUrl)
	if err != nil {
		fmt.Println(err)
		return nil
	}

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

func (c *client) sendBatch(tenantID string, batch *batch) {

	buf, _, err := batch.encode()
	if err != nil {
		fmt.Println(err)
		return
	}
	int, err := c.send(context.Background(), tenantID, buf)
	fmt.Println(int, err)
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
