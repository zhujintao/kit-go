package mysql

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/schema"
	"github.com/juju/errors"
	"github.com/zhujintao/kit-go/ssh"
)

type Conn struct {
	connLock sync.Mutex
	conn     *client.Conn
	ctx      context.Context
	cfg      *Config
	cancel   context.CancelFunc
}

func NewClient(cfg *Config) *Conn {
	c := new(Conn)
	if cfg.Dialer == nil {
		dialer := &net.Dialer{}
		cfg.Dialer = dialer.DialContext
	}
	c.cfg = cfg
	c.ctx, c.cancel = context.WithCancel(context.Background())

	return c

}

func NewClientViaSSH(sshAddr, sshUser, sshPassword string, cfg *Config) *Conn {
	sshcon, err := ssh.NewConn(sshAddr, sshUser, sshPassword)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	viasshDialer := func(ctx context.Context, network, address string) (net.Conn, error) {
		return sshcon.Dial(network, address)
	}
	c := new(Conn)
	cfg.Dialer = viasshDialer
	c.cfg = cfg
	c.ctx, c.cancel = context.WithCancel(context.Background())
	return c

}

func (c *Conn) GetTableInfo(db, table string) (*schema.Table, error) {
	return schema.NewTable(c, db, table)

}

func (c *Conn) Close() {
	c.conn.Close()
}

func (c *Conn) connect(options ...client.Option) (*client.Conn, error) {
	ctx, cancel := context.WithTimeout(c.ctx, time.Second*10)
	defer cancel()

	return client.ConnectWithDialer(ctx, "", c.cfg.Addr,
		c.cfg.User, c.cfg.Password, "", c.cfg.Dialer, options...)
}
func (c *Conn) UseDB(db string, args ...interface{}) (err error) {
	c.connLock.Lock()
	defer c.connLock.Unlock()
	argF := make([]client.Option, 0)
	if c.cfg.TLSConfig != nil {
		argF = append(argF, func(conn *client.Conn) error {
			conn.SetTLSConfig(c.cfg.TLSConfig)
			return nil
		})
	}

	retryNum := 3
	for i := 0; i < retryNum; i++ {
		if c.conn == nil {
			c.conn, err = c.connect(argF...)
			if err != nil {
				return errors.Trace(err)
			}
		}

		err = c.conn.UseDB(db)
		if err != nil {
			if mysql.ErrorEqual(err, mysql.ErrBadConn) {
				c.conn.Close()
				c.conn = nil
				continue
			}
			return err
		}
		break
	}
	return err
}

func (c *Conn) Execute(cmd string, args ...interface{}) (rr *mysql.Result, err error) {
	c.connLock.Lock()
	defer c.connLock.Unlock()
	argF := make([]client.Option, 0)
	if c.cfg.TLSConfig != nil {
		argF = append(argF, func(conn *client.Conn) error {
			conn.SetTLSConfig(c.cfg.TLSConfig)
			return nil
		})
	}

	retryNum := 3
	for i := 0; i < retryNum; i++ {
		if c.conn == nil {
			c.conn, err = c.connect(argF...)
			if err != nil {
				return nil, errors.Trace(err)
			}
		}

		rr, err = c.conn.Execute(cmd, args...)
		if err != nil {
			if mysql.ErrorEqual(err, mysql.ErrBadConn) {
				c.conn.Close()
				c.conn = nil
				continue
			}
			return nil, err
		}
		break
	}
	return rr, err
}

func (c *Conn) ExecuteSelectStreaming(cmd string, perRowCallback func(row []mysql.FieldValue) error, perResultCallback func(result *mysql.Result) error) (err error) {
	c.connLock.Lock()
	defer c.connLock.Unlock()
	argF := make([]client.Option, 0)
	if c.cfg.TLSConfig != nil {
		argF = append(argF, func(conn *client.Conn) error {
			conn.SetTLSConfig(c.cfg.TLSConfig)
			return nil
		})
	}

	retryNum := 3
	for i := 0; i < retryNum; i++ {
		if c.conn == nil {
			c.conn, err = c.connect(argF...)
			if err != nil {
				return errors.Trace(err)
			}
		}
		var result mysql.Result
		err = c.conn.ExecuteSelectStreaming(cmd, &result, perRowCallback, perResultCallback)
		if err != nil {
			if mysql.ErrorEqual(err, mysql.ErrBadConn) {
				c.conn.Close()
				c.conn = nil
				continue
			}
			return err
		}
		break
	}
	return err
}

func (c *Conn) GetNextPage(table, key, startId string, limit int) []string {
	var list []string
	var maxid string

	maxid = startId
	list = append(list, maxid)
	var count int

	for {
		ts := time.Now()
		count++
		sql := fmt.Sprintf("SELECT MAX(%s) FROM (SELECT %s FROM %s WHERE %s > ? ORDER BY ID LIMIT %d) a", key, key, table, key, limit)

		r, err := c.Execute(sql, maxid)
		if err != nil {
			return nil
		}
		maxid = string(r.Values[0][0].AsString())
		if maxid == "" {
			break
		}
		fmt.Printf("%d %v %v %v\n", count, maxid, r.Status, time.Since(ts).Truncate(time.Millisecond).String())
		list = append(list, maxid)
	}
	return list
}
