package exporter

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var namespace = "zjt"

var MetricGlobalLable map[string]string = map[string]string{}

type Metric struct {
	desc map[string]*prometheus.Desc
	name string
	unit string // total, bytes, seconds, info, ratio (percent) https://prometheus.io/docs/practices/naming/

	labelName  []string
	labelValue []string
	idx        int
	help       string
	ch         chan<- prometheus.Metric
	l          sync.Mutex
	cacel      bool
}
type sendch struct {
	metric prometheus.Metric
}

func newMetric() *Metric {
	return &Metric{desc: make(map[string]*prometheus.Desc), labelName: make([]string, 100), labelValue: make([]string, 100), l: sync.Mutex{}}
}

// unit: total, bytes, seconds, info, ratio (percent)
func (a *Metric) Create(name, unit string) *Metric {
	a.idx = 0
	a.name = name
	a.unit = unit
	a.help = ""
	a.labelName = make([]string, 100)
	a.labelValue = make([]string, 100)

	return a

}

func (a *Metric) SetLabel(name, value string) *Metric {

	a.labelName[a.idx] = name
	a.labelValue[a.idx] = value
	a.idx++
	return a

}
func (a *Metric) SetHelp(help string) *Metric {
	a.help = help
	return a
}

func (a *Metric) SendGauge(v float64) {
	a.Send(prometheus.GaugeValue, v)
}
func (a *Metric) SendCounter(v float64) {
	a.Send(prometheus.CounterValue, v)
}
func (a *Metric) Send(valueType prometheus.ValueType, value float64) {
	a.send(namespace, valueType, value)
}
func (a *Metric) SendWithoutNs(valueType prometheus.ValueType, value float64) {
	a.send("", valueType, value)
}
func (a *Metric) send(namespace string, valueType prometheus.ValueType, value float64) {

	var labelName []string
	var labelValue []string

	for k, v := range MetricGlobalLable {

		if slices.Contains(a.labelName, k) {
			continue
		}

		a.labelName[a.idx] = k
		a.labelValue[a.idx] = v
		a.idx++

	}

	for i := range a.labelName {
		if a.labelName[i] == "" {
			continue
		}

		labelName = append(labelName, a.labelName[i])
		labelValue = append(labelValue, a.labelValue[i])
	}
	if len(labelName) != len(labelValue) {
		fmt.Printf("inconsistent label cardinality: expected %d label values but got %d in %v\n", len(labelName), len(labelValue), labelValue)
		return
	}
	desc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, a.name, a.unit),
		a.help,
		labelName,
		nil)
	if d, ok := a.desc[desc.String()+strings.Join(labelValue, "")]; ok {
		if a.cacel {
			return
		}

		a.ch <- prometheus.MustNewConstMetric(d, valueType, value, labelValue...)
		return
	}

	a.desc[desc.String()+strings.Join(a.labelValue, "")] = desc

}
