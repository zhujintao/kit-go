package canal

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-mysql-org/go-mysql/canal"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/siddontang/go-log/loggers"
	"github.com/zhujintao/kit-go/utils"
)

type Canal = canal.Canal
type EventHandler = canal.EventHandler

type Container struct {
	Addr     string
	User     string
	Password string
	Handler  EventHandler
	Prepare  func(gtidSet GTIDSet, conf *Container, dbs []string) error
	Filter   *filterTable
	WorkDir  string
	Log      loggers.Advanced
}

func Run(id string, container Container, gtid_executed ...string) error {

	cfg := canal.NewDefaultConfig()
	if container.Handler == nil {
		cfg.Logger.Error("Handler is nil")
		return nil
	}
	cfg.Addr = container.Addr
	cfg.User = container.User
	cfg.Password = container.Password
	cfg.Dump.ExecutionPath = ""
	cfg.ReadTimeout = time.Hour * 24
	cfg.HeartbeatPeriod = time.Second * 1
	cfg.MaxReconnectAttempts = 3

	if container.Log != nil {
		cfg.Logger = container.Log
	}
	container.Log = cfg.Logger

	if container.Filter != nil {
		cfg.IncludeTableRegex = container.Filter.include
		cfg.ExcludeTableRegex = container.Filter.exclude

	}

	c, err := canal.NewCanal(cfg)
	if err != nil {
		cfg.Logger.Error(err)
		return err
	}

	wg := &sync.WaitGroup{}

	h, ok := container.Handler.(*defaultEventHandler)
	if ok {
		h.setCanal(c)
		wg.Add(1)
		go h.work(wg, cfg.Logger, 200)
	}
	c.SetEventHandler(h)

	var gtidSet mysql.GTIDSet

	if h.MasterInfo == nil {
		h.MasterInfo = &masterInfo{}
	}

	if err := h.MasterInfo.Init(&container.WorkDir, id); err != nil {
		cfg.Logger.Error("MasterInfo Init ", err)
		return err
	}

	g, err := h.MasterInfo.Load()
	if err != nil {
		cfg.Logger.Error(err)
		return err
	}
	gtidSet, _ = mysql.ParseGTIDSet("mysql", g)

	if len(gtid_executed) == 1 && gtid_executed[0] != "" {
		gtidSet, _ = mysql.ParseGTIDSet("mysql", gtid_executed[0])
	}

	if container.Prepare != nil {

		query := "select table_schema as database_name, table_name from information_schema.tables where table_type != 'view'  order by database_name, table_name"
		r, err := c.Execute(query)
		if err != nil {
			return fmt.Errorf("get dbs: %v", err)
		}
		var dbs []string
		for _, row := range r.Values {

			db := string(row[0].AsString())
			table := string(row[1].AsString())
			t := db + "." + table
			if container.Filter.Match(t) {
				dbs = append(dbs, t)
			}

		}

		err = container.Prepare(gtidSet, &container, dbs)
		if err != nil {
			cfg.Logger.Error(err)
			return err
		}

	}

	if gtidSet.String() == "" {
		cfg.Logger.Error("gtidSet not set.")
		return nil
	}

	utils.SignalNotify().Close(func() {
		c.Close()
		err := h.MasterInfo.Close()
		container.Log.Infoln("masterinfo save error:", err)
		container.Log.Infoln("sig close")

	})
	err = c.StartFromGTID(gtidSet)
	wg.Wait()
	container.Log.Infoln(id, "exit")
	return err
}
