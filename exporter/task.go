package exporter

import "github.com/prometheus/client_golang/prometheus"

var namespace = "zjt"

type Task struct {
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

func newTask() *Task {
	return &Task{desc: make(map[string]*prometheus.Desc), labelName: make([]string, 100), labelValue: make([]string, 100)}
}

// unit: total, bytes,seconds,ratio (percent)
func (a *Task) CreateMetric(name, unit string) *Task {
	a.idx = 0
	a.name = name
	a.unit = unit
	a.help = ""
	a.labelName = make([]string, 100)
	a.labelValue = make([]string, 100)
	return a

}

func (a *Task) SetLabel(name, value string) *Task {

	a.labelName[a.idx] = name
	a.labelValue[a.idx] = value
	a.idx++

	return a

}
func (a *Task) SetHelp(help string) *Task {
	a.help = help
	return a
}

func (a *Task) SendGauge(v float64) {
	a.Send(prometheus.GaugeValue, v)
}
func (a *Task) SendCounter(v float64) {
	a.Send(prometheus.CounterValue, v)
}
func (a *Task) Send(valueType prometheus.ValueType, value float64) {
	a.send(namespace, valueType, value)
}
func (a *Task) SendWithoutNs(valueType prometheus.ValueType, value float64) {
	a.send("", valueType, value)
}
func (a *Task) send(namespace string, valueType prometheus.ValueType, value float64) {

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
