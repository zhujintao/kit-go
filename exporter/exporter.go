package exporter

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"os"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	promcollectors "github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/cli/v3"
)

type Collector interface {
	Exec(ch chan<- prometheus.Metric) error
}

var (
	collectors map[string]Collector = map[string]Collector{}
	listen     string
	app        *cli.Command = &cli.Command{
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

type collector struct {
	name    string
	running map[string]bool
	metric  *Metric
	fn      func(metric *Metric) error
}

func (c *collector) AddFlag(flag ...cli.Flag) {
	app.Flags = append(app.Flags, flag...)
}

func (c *collector) CallFunc(fn func(metric *Metric) error) {

	c.fn = fn
}

func (c *collector) Exec(ch chan<- prometheus.Metric) error {
	c.metric.ch = ch
	return c.fn(c.metric)
}

func (c *collector) RegistryCollector() {

	RegistryCollector(c.name, c)
}

func NewCollector(name string) *collector {

	return &collector{
		name:   name,
		metric: newMetric(),
	}
}

func RegistryCollector(collector string, c Collector, help ...string) {
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
func RegistryFactory(collector string, factory func() (Collector, error), help ...string) {

	if factory == nil {
		return
	}

	c, err := factory()
	if err != nil {
		return
	}

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

type exporter struct {
	name   string
	metric *Metric
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
	return &exporter{name: app.Name, metric: newMetric()}
}

func (exporter) Describe(ch chan<- *prometheus.Desc) {}
func (e *exporter) Collect(ch chan<- prometheus.Metric) {
	e.metric.ch = ch

	e.metric.Create(e.name, "up").SendWithoutNs(prometheus.GaugeValue, 1)
	var wg sync.WaitGroup
	for name, c := range collectors {

		wg.Add(1)
		go func(name string, c Collector) {
			defer wg.Done()
			err := c.Exec(ch)
			if err != nil {
				fmt.Println(c, err)
				return
			}
		}(name, c)

	}
	wg.Wait()

}
func (e *exporter) Namespace(n string) *exporter {
	namespace = n
	return e
}
func (e *exporter) Run() {
	prometheus.Unregister(promcollectors.NewGoCollector())
	prometheus.Unregister(promcollectors.NewProcessCollector(promcollectors.ProcessCollectorOpts{PidFn: func() (int, error) {
		return os.Getpid(), nil
	}}))
	prometheus.MustRegister(e)
	http.Handle("/metrics", promhttp.Handler())

	fmt.Println("Starting "+e.name, "listen", listen)
	http.ListenAndServe(listen, nil)
}
