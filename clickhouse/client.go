package clickhouse

import (
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type Conn = driver.Conn

type Config struct {
	Addr     []string
	User     string
	Password string
	Options  *clickhouse.Options
	Settings clickhouse.Settings
}

type Batch = driver.Batch

func NewClient(cfg *Config) driver.Conn {

	c := &clickhouse.Options{}
	c.Addr = cfg.Addr
	c.Auth.Username = cfg.User
	c.Auth.Password = cfg.Password
	c.Compression = &clickhouse.Compression{
		Method: clickhouse.CompressionLZ4,
	}
	c.Settings = clickhouse.Settings{"insert_allow_materialized_columns": true}

	conn, err := clickhouse.Open(c)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return conn

}
