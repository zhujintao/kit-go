package promwrite

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/model/labels"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/prometheus/prometheus/storage/remote"
	"resty.dev/v3"
)

type WriteRequest = writev2.Request
type HttpRequest = resty.Request

type metric struct {
	cli *HttpRequest
}
type label struct {
	m       *metric
	name    string
	uint    string
	help    string
	s       writev2.SymbolsTable
	lables  labels.ScratchBuilder
	samples []writev2.Sample
}

func NewMetric(url string, auth ...string) *metric {

	cli := resty.New().SetDisableWarn(true)
	if len(auth) == 2 {
		cli.SetBasicAuth(auth[0], auth[1])
	}

	s := cli.R().
		SetHeader("Content-Type", "application/x-protobuf;proto=io.prometheus.write.v2.Request").
		SetHeader("Content-Encoding", "snappy").
		SetHeader(remote.RemoteWriteVersionHeader, remote.RemoteWriteVersion20HeaderValue).
		SetHeader("User-Agent", "promwrite/0.0.0").
		SetURL(url)

	return &metric{cli: s}

}

func (l *label) Label(name, value string) *label {

	l.lables.Add(name, value)

	return l
}
func (l *label) SetValue(value float64, ts ...int64) *label {
	_ts := time.Now().UnixMilli()

	if len(ts) == 1 {
		_ts = ts[0]
	}
	l.samples = append(l.samples, writev2.Sample{Value: value, Timestamp: _ts})

	return l

}
func (l *label) Send() error {

	l.lables.Add("__name__", l.name)
	l.lables.Sort()
	ref := l.s.SymbolizeLabels(l.lables.Labels(), nil)
	uint := l.s.Symbolize(l.uint)
	help := l.s.Symbolize(l.help)

	w := &WriteRequest{}
	w.Symbols = l.s.Symbols()
	w.Timeseries = []writev2.TimeSeries{{
		LabelsRefs: ref,
		Samples:    l.samples,
		Metadata:   writev2.Metadata{HelpRef: help, UnitRef: uint},
	}}

	data, err := w.Marshal()

	l.s.Reset()
	l.lables.Reset()

	l.samples = l.samples[:0]

	resutl, err := l.m.cli.SetBody(snappy.Encode(nil, data)).Post(l.m.cli.URL)
	if err != nil {
		return err
	}
	if resutl.StatusCode() != 204 {
		fmt.Println(resutl.String())
		return errors.New(resutl.String())

	}
	return nil
}

func (m *metric) Name(name, unit string, help ...string) *label {

	_name := name
	if unit != "" {
		_name = name + "_" + unit
	}

	return &label{
		m:      m,
		s:      writev2.NewSymbolTable(),
		lables: labels.ScratchBuilder{},
		name:   _name,
		uint:   unit,
	}

}
