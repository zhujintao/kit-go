package promwrite

import (
	"sort"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"resty.dev/v3"
)

type WriteRequest = prompb.WriteRequest
type HttpRequest = resty.Request

type metric struct {
	name   string
	unit   string
	help   string
	mtype  prompb.MetricMetadata_MetricType
	labels []prompb.Label
	values []prompb.Sample
}

type byLabelName []prompb.Label

func (a byLabelName) Len() int           { return len(a) }
func (a byLabelName) Less(i, j int) bool { return a[i].Name < a[j].Name }
func (a byLabelName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func (m *metric) sort() {
	sort.Stable(byLabelName(m.labels))
}

func AddMetric(name, unit string, help ...string) *metric {
	_help := ""
	if len(help) == 1 {
		_help = help[0]
	}
	fullName := name
	if unit != "" {
		fullName = name + "_" + unit
	}

	return &metric{name: fullName, unit: unit, help: _help, labels: []prompb.Label{{Name: "__name__", Value: fullName}}}

}
func (m *metric) AddLabel(labelName, labelValue string) *metric {
	m.labels = append(m.labels, prompb.Label{Name: labelName, Value: labelValue})
	return m
}

func (m *metric) SetGauge(value float64, ts ...int64) *metric {
	now := time.Now().UnixMilli()
	if len(ts) != 1 {
		now = time.Now().UnixMilli()
	}
	m.mtype = prompb.MetricMetadata_GAUGE
	m.values = append(m.values, prompb.Sample{Value: value, Timestamp: now})
	return m
}

func (m *metric) ApplyWriteRequest(w *WriteRequest) {

	if m.values == nil {
		panic("value not set")
	}
	m.sort()
	w.Timeseries = append(w.Timeseries, prompb.TimeSeries{Labels: m.labels, Samples: m.values})

	w.Metadata = append(w.Metadata, prompb.MetricMetadata{Type: m.mtype, MetricFamilyName: m.name, Unit: m.unit, Help: m.help})

}

type promWrite struct {
	WriteRequest *WriteRequest
	cli          *resty.Client
}

func New() *promWrite {
	return &promWrite{WriteRequest: &prompb.WriteRequest{}, cli: resty.New()}
}
func (w *promWrite) AddMetric(name, unit string, help ...string) *metric {

	return AddMetric(name, unit, help...)

}

// use resty Post() write remote
func (w promWrite) Send() *HttpRequest {

	data, err := w.WriteRequest.Marshal()
	if err != nil {
		panic(err)
	}

	return w.cli.R().
		SetBody(snappy.Encode(nil, data)).
		SetHeader("Content-Type", "application/x-protobuf").
		SetHeader("Content-Encoding", "snappy").
		SetHeader("X-Prometheus-Remote-Write-Version", "0.1.0").
		SetHeader("User-Agent", "promremote-go/1.0.0")

}
