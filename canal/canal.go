package canal

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/go-mysql-org/go-mysql/canal"
	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/zhujintao/kit-go/utils"
	"golang.org/x/crypto/ssh"
)

type Canal = canal.Canal
type EventHandler = canal.EventHandler
type Prepare func(gtidSet GTIDSet, c *Container, table []string) error
type Dialer = client.Dialer
type viaSSh struct {
	Addr     string
	User     string
	Password string
}

type Container struct {
	Addr     string
	User     string
	Password string
	// use canal.DefaultEvent()
	Handler EventHandler
	// full data logic
	Prepare    Prepare
	Filter     *filterTable
	WorkDir    string
	log        *logger
	TableCehck tableCheck
	ViaSsh     *viaSSh
}

func ViaSsh(addr, user, password string) *viaSSh {
	return &viaSSh{addr, user, password}
}

func Run(id string, container Container, gtid_executed ...string) error {
	container.log = newlogger(fmt.Sprintf("%s=%v\t", "id", id))
	cfg := canal.NewDefaultConfig()
	if container.Handler == nil {
		cfg.Logger.Infoln("Handler not nil, exit")
		return nil
	}
	cfg.Addr = container.Addr
	cfg.User = container.User
	cfg.Password = container.Password
	cfg.Dump.ExecutionPath = ""

	cfg.ReadTimeout = time.Hour * 24
	cfg.HeartbeatPeriod = time.Second * 2
	cfg.MaxReconnectAttempts = 3
	cfg.Logger = container.log
	if container.ViaSsh != nil {
		cfg.ReadTimeout = -1
		sconn, err := ssh.Dial("tcp", container.ViaSsh.Addr, &ssh.ClientConfig{User: container.ViaSsh.User,
			Auth:            []ssh.AuthMethod{ssh.Password(container.ViaSsh.Password)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		})
		if err != nil {
			return err
		}
		defer sconn.Close()

		go func() {

			for range time.Tick(cfg.HeartbeatPeriod) {
				_, _, err := sconn.SendRequest("hello", true, nil)
				if err != nil {
					container.log.Error(err)
					return
				}
			}
		}()

		cfg.Dialer = func(ctx context.Context, network, address string) (net.Conn, error) {
			return sconn.Dial(network, address)
		}
	}

	if container.Filter == nil {
		container.Filter = FilterTable()
	}

	cfg.IncludeTableRegex = container.Filter.include
	cfg.ExcludeTableRegex = container.Filter.exclude

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
		go h.work(wg, container.log, 200)
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
			container.log.Error(err)
			return err
		}

	}

	if gtidSet == nil || gtidSet.String() == "" {
		cfg.Logger.Error("gtid_executed not set, or use Container.Prepare")
		return nil
	}

	utils.SignalNotify().Close(func() {
		c.Close()
		err := h.MasterInfo.Close()
		container.log.Infoln("masterinfo save error:", err)
		container.log.Infoln("sig close")

	})
	err = c.StartFromGTID(gtidSet)
	wg.Wait()
	container.log.Infoln(id, "exit")
	return err
}
