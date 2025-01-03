package exporter

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"os"
	"reflect"

	"github.com/prometheus/client_golang/prometheus"
	promcollectors "github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/cli/v3"
)

var (
	collectors map[string]*Collector = map[string]*Collector{}

	listen string
	app    *cli.Command = &cli.Command{
		Name:            "zjt_exporter",
		HideHelpCommand: true,
		Action: func(ctx context.Context, cli *cli.Command) error {

			return nil
		},
		OnUsageError: func(ctx context.Context, cmd *cli.Command, err error, isSubcommand bool) error {

			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "listen", Value: ":2121", Usage: "expose metrics and web interface` `", Destination: &listen},
		},
		CustomRootCommandHelpTemplate: help_v3,
		Usage:                         "a metrics all in one kit, base prometheus.",
	}
)

type exporter struct {
	name string
}

func NewApp(appName ...string) *exporter {

	cli.HelpFlag = &cli.BoolFlag{Name: "help", Hidden: true}

	if len(appName) == 1 {
		app.Name = appName[0]

	}

	err := app.Run(context.Background(), os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}
	if len(collectors) == 0 {
		fmt.Println("not available collectors, use --help")
		os.Exit(0)
	}
	fmt.Println("Working metrics:")
	for c := range maps.Keys(collectors) {
		fmt.Printf("\t%s\n", c)
	}
	return &exporter{name: app.Name}
}

type Collector struct {
	name   string
	metric *Metric
	fn     func(metric *Metric) error
}

// "github.com/urfave/cli/v3"
func (c *Collector) AddFlag(flag ...cli.Flag) {

	for _, f := range flag {
		_f := reflect.ValueOf(f).Elem().FieldByName("Name")
		_f.Set(reflect.ValueOf(c.name + "-" + _f.String()))
	}

	app.Flags = append(app.Flags, flag...)

}
func (Collector) Describe(ch chan<- *prometheus.Desc) {}
func (c *Collector) Collect(ch chan<- prometheus.Metric) {

	c.metric.ch = ch
	if c.fn == nil {
		return
	}
	c.fn(c.metric)
}
func (c *Collector) CallFunc(fn func(metric *Metric) error) {
	c.fn = fn
}

func NewCollector(name string) *Collector {

	c := &Collector{name: name, metric: newMetric()}

	return c
}

func RegCollector(fs ...func() *Collector) {
	for _, f := range fs {
		if f == nil {
			continue
		}
		c := f()
		c.RegCollector()
	}
}

func (c *Collector) RegCollector() {
	registry(c.name, c)
}

func (e *exporter) Run() {
	prometheus.Unregister(promcollectors.NewGoCollector())
	prometheus.Unregister(promcollectors.NewProcessCollector(promcollectors.ProcessCollectorOpts{PidFn: func() (int, error) {
		return os.Getpid(), nil
	}}))
	for _, c := range collectors {
		prometheus.MustRegister(c)
	}

	http.Handle("/metrics", promhttp.Handler())
	fmt.Println("Starting "+e.name, "listen", listen)
	http.ListenAndServe(listen, nil)
}

func registry(collector string, c *Collector, help ...string) {

	flagName := fmt.Sprintf(collector)
	flagHelp := ""
	if len(help) == 1 {
		flagHelp = help[0]
	}

	flag := &cli.BoolFlag{Name: flagName, Category: "collectors:", Usage: flagHelp, HideDefault: true}
	flag.Action = func(ctx context.Context, cli *cli.Command, b bool) error {
		if b {
			collectors[collector] = c
		}
		return nil
	}

	app.Flags = append(app.Flags, flag)

}
