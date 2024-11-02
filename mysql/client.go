package mysql

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/schema"
	"github.com/juju/errors"
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

func (c *Conn) GetTableInfo(db, table string) (*schema.Table, error) {
	return schema.NewTable(c, db, table)

}

func (c *Conn) Close() {
	c.conn.Close()
}
func (c *Conn) Ping() error {
	return c.conn.Ping()
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
