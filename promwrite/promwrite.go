package promwrite

import (
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/golang/snappy"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/prometheus/prometheus/storage/remote"
	"resty.dev/v3"
)

type WriteRequest = writev2.Request
type HttpRequest = resty.Request

type metric struct {
	labelName  []string
	labelValue []string
	labels     map[string]int
	timeSeries []writev2.TimeSeries
}

var cli *resty.Client = resty.New()

// use resty Post() write remote /api/v1/write
func (m *metric) Send() *HttpRequest {
	symbols := append(m.labelName, m.labelValue...)
	w := WriteRequest{}
	w.Symbols = append(w.Symbols, symbols...)

	w.Timeseries = append(w.Timeseries, m.timeSeries...)
	fmt.Println(w)
	data, _ := w.Marshal()
	return cli.R().
		//SetBasicAuth("xxx", "xxx").
		SetBody(snappy.Encode(nil, data)).
		SetHeader("Content-Type", "application/x-protobuf;proto=io.prometheus.write.v2.Request").
		SetHeader("Content-Encoding", "snappy").
		SetHeader(remote.RemoteWriteVersionHeader, remote.RemoteWriteVersion20HeaderValue).
		SetHeader("User-Agent", "promwrite/0.0.0")

}

type label struct {
	m   *metric
	ref []uint32
}

func (l *label) Label(k, v string) *label {

	if !slices.Contains(l.m.labelName, k) {
		l.m.labelName = append(l.m.labelName, k)
	}

	if !slices.Contains(l.m.labelValue, v) {

		l.m.labelValue = append(l.m.labelValue, v)

		l.m.labels[k] = len(l.m.labelValue) - 1
	}

	return l
}

func (l *label) SetValue(value float64, ts ...int64) {
	sort.Strings(l.m.labelName)

	for kref, s := range l.m.labelName {
		if kref == 0 {
			continue
		}

		vref := l.m.labels[s] + len(l.m.labelName)
		l.ref = append(l.ref, uint32(kref), uint32(vref))

	}
	_ts := time.Now().UnixMilli()
	if len(ts) == 1 {
		_ts = ts[0]
	}

	timeSeries := writev2.TimeSeries{LabelsRefs: l.ref}
	timeSeries.Samples = append(timeSeries.Samples, writev2.Sample{Value: value, Timestamp: _ts})
	l.m.timeSeries = append(l.m.timeSeries, timeSeries)

	l.ref = l.ref[:0]

}

func (m *metric) Name(name, unit string) *label {

	_name := name
	if unit != "" {
		_name = name + "_" + unit
	}

	if !slices.Contains(m.labelValue, _name) {
		m.labelValue = append(m.labelValue, _name)

	}

	return &label{
		m: m,
	}
}

// Remote Write 2.0
func NewMetric() *metric {
	return &metric{labelName: []string{"", "__name__"}, labels: map[string]int{"__name__": 0}}
}
