package exporter

import "github.com/prometheus/client_golang/prometheus"

const namespace = "zjt"

type Action struct {
	desc map[string]*prometheus.Desc
	name string
	unit string // total, bytes,seconds,ratio (percent) https://prometheus.io/docs/practices/naming/

	labelName  []string
	labelValue []string
	idx        int
	help       string
	ch         chan<- prometheus.Metric
}
type sendch struct {
	metric prometheus.Metric
}

func newAction() *Action {
	return &Action{desc: make(map[string]*prometheus.Desc), labelName: make([]string, 100), labelValue: make([]string, 100)}
}

func (a *Action) CreateMetric(name, unit string) *Action {
	a.idx = 0
	a.name = name
	a.unit = unit
	a.help = ""
	a.labelName = make([]string, 100)
	a.labelValue = make([]string, 100)
	return a

}

func (a *Action) SetLabel(name, value string) *Action {

	a.labelName[a.idx] = name
	a.labelValue[a.idx] = value
	a.idx++

	return a

}
func (a *Action) SetHelp(help string) *Action {
	a.help = help
	return a
}

func (a *Action) SendGauge(v float64) {
	a.Send(prometheus.GaugeValue, v)
}
func (a *Action) SendCounter(v float64) {
	a.Send(prometheus.CounterValue, v)
}

func (a *Action) Send(valueType prometheus.ValueType, value float64) {

	var labelName []string
	var labelValue []string
	for _, v := range a.labelName {
		if v == "" {
			continue
		}
		labelName = append(labelName, v)
	}
	for _, v := range a.labelValue {
		if v == "" {
			continue
		}
		labelValue = append(labelValue, v)
	}

	desc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, a.name, a.unit),
		a.help,
		labelName,
		nil)
	if d, ok := a.desc[desc.String()]; ok {
		a.ch <- prometheus.MustNewConstMetric(d, valueType, value, labelValue...)
		return
	}
	a.desc[desc.String()] = desc

}
