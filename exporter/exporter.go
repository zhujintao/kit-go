package exporter

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"os"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	promcollectors "github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/cli/v3"
)

var (
	collectors map[string]*Collector = map[string]*Collector{}
	flagCheck  map[string][]string   = map[string][]string{}

	listen string
	app    *cli.Command = &cli.Command{
		Name:            "zjt_exporter",
		HideHelpCommand: true,
		Action: func(ctx context.Context, cli *cli.Command) error {

			return nil
		},

		Flags: []cli.Flag{
			&cli.StringFlag{Name: "listen", Value: ":2121", Usage: "expose metrics and web interface` `", Destination: &listen},
		},
		CustomRootCommandHelpTemplate: help_v3,
		Usage:                         "a metrics all in one kit, base prometheus.",
	}
	l sync.Locker = &sync.RWMutex{}
)

type exporter struct {
	name string
}

func (exporter) Describe(ch chan<- *prometheus.Desc) {}
func (e *exporter) Collect(ch chan<- prometheus.Metric) {

	metric := NewMetric(ch)
	metric.Create(e.name, "up").SendWithoutNs(prometheus.GaugeValue, 1)
	metric = nil
	var wg sync.WaitGroup

	for name, c := range collectors {

		wg.Add(1)
		go func(name string, c *Collector) {
			defer wg.Done()
			c.exec(ch)
		}(name, c)

	}
	wg.Wait()

}

type Collector struct {
	name string
	fn   []func(*Collector) error
	//metric   *Metric
	callFunc []func(metric *Metric)
}

func NewCollector(name string) *Collector {
	return &Collector{
		name: name,
		//metric: newMetric(),
	}
}
func (c *Collector) Do(fn func(*Collector) error) *Collector {
	c.fn = append(c.fn, fn)
	return c
}
func (c *Collector) CallFunc(fn func(metric *Metric)) *Collector {

	c.callFunc = append(c.callFunc, fn)
	return c

}

// use v3 "github.com/urfave/cli/v3"
func (c *Collector) AddFlag(flag cli.Flag, required ...bool) {
	f := reflect.ValueOf(flag).Elem().FieldByName("Name")
	f.Set(reflect.ValueOf(c.name + "-" + f.String()))
	var req bool
	if len(required) == 1 {
		req = required[0]
	}
	if req {
		flagCheck[c.name] = append(flagCheck[c.name], f.String())
	}

	app.Flags = append(app.Flags, flag)
}

func (c *Collector) GetValue(flagName string) interface{} {
	return app.Value(c.name + "-" + flagName)
}

func (c *Collector) exec(ch chan<- prometheus.Metric) {

	//c.metric.ch = ch
	if len(c.callFunc) == 0 {
		return
	}
	metric := &Metric{desc: make(map[string]*prometheus.Desc), labelName: make([]string, 100), labelValue: make([]string, 100), ch: ch}
	var wg sync.WaitGroup
	for _, f := range c.callFunc {
		wg.Add(1)
		go func(f func(metric *Metric), m *Metric) {
			defer wg.Done()
			ctx, done := context.WithCancel(context.Background())
			go func(f func(metric *Metric), m *Metric) {
				f(m)
				done()
			}(f, m)
			select {
			case <-time.NewTimer(time.Second * 10).C:
				m.cacel = true
				return
			case <-ctx.Done():
				return
			}

		}(f, metric)
	}
	wg.Wait()
	metric = nil
}

func (c *Collector) Register(help ...string) {

	flagName := fmt.Sprintf(c.name)
	flagHelp := ""
	if len(help) == 1 {
		flagHelp = help[0]
	}

	flag := &cli.BoolFlag{Name: flagName, Category: "collectors:", Usage: flagHelp, HideDefault: true, Action: func(ctx context.Context, cc *cli.Command, b bool) error {

		if b {
			for _, flag := range flagCheck[c.name] {
				if !slices.Contains(cc.FlagNames(), flag) {
					return fmt.Errorf("collector [%s] require flag: --%s ", c.name, flag)
				}
			}

			for _, f := range c.fn {
				if err := f(c); err != nil {
					return err
				}
			}
			collectors[c.name] = c
		}

		return nil
	}}
	app.Flags = append(app.Flags, flag)
}

func NewApp(appName ...string) *exporter {
	cli.HelpFlag = &cli.BoolFlag{Name: "help", Hidden: true}

	if len(appName) == 1 {
		app.Name = appName[0]

	}
	return &exporter{name: app.Name}
}

func (e *exporter) Run() {

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Println("Error: ", err)
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
	prometheus.MustRegister(e)
	prometheus.Unregister(promcollectors.NewGoCollector())
	prometheus.Unregister(promcollectors.NewProcessCollector(promcollectors.ProcessCollectorOpts{PidFn: func() (int, error) {
		return os.Getpid(), nil
	}}))

	http.Handle("/metrics", promhttp.Handler())

	fmt.Println("Starting "+e.name, "listen", listen)

	if err := http.ListenAndServe(listen, nil); err != nil {
		fmt.Println(err)
	}

}
