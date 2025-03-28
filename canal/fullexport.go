package canal

import (
	"strings"

	"github.com/zhujintao/kit-go/mysql"
)

type tableCheck interface {
	Begin() bool
	End() bool
}

// fn space scope, return func is cdc logic
func FullDataExport(c *Container, tables []string, gtidSet GTIDSet, fn func(tableInfo *mysql.TableInfo) func(row []mysql.FieldValue) error) error {

	cli := mysql.NewClient(&mysql.Config{Addr: c.Addr, User: c.User, Password: c.Password})
	if c.ViaSsh != nil {
		cli = mysql.NewClientViaSSH(c.ViaSsh.Addr, c.ViaSsh.User, c.ViaSsh.Password, &mysql.Config{Addr: c.Addr, User: c.User, Password: c.Password})
	}

	cli.Execute("SET @master_heartbeat_period=1")
	//lock table
	cli.Execute("FLUSH /*!40101 LOCAL */ TABLES")
	cli.Execute("FLUSH TABLES WITH READ LOCK")
	cli.Execute("SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ")
	cli.Execute("START TRANSACTION /*!40100 WITH CONSISTENT SNAPSHOT */")
	/*
		// check gitd mode
		r, _ := cli.Execute("SHOW VARIABLES LIKE 'gtid_mode'")
		fmt.Println(r.GetString(0, 1))

		// get gitd EXECUTED
		r, _ = cli.Execute("SELECT @@GLOBAL.GTID_EXECUTED")
		fmt.Println(r.GetString(0, 0))

		r, _ = cli.Execute("SHOW MASTER STATUS")
		fmt.Println(r.GetString(0, 4))
		fmt.Println()
	*/
	r, _ := cli.Execute("SELECT @@GLOBAL.GTID_EXECUTED")
	set, _ := r.GetString(0, 0)
	gtidSet.Update(set)

	// unlock table
	cli.Execute("UNLOCK TABLES")

	for _, table := range tables {

		sql := "select * from " + table
		st := strings.Split(table, ".")
		tableIfo, err := cli.GetTableInfo(st[0], st[1])
		if err != nil {
			return err
		}
		exec := fn(tableIfo)
		err = cli.ExecuteSelectStreaming(sql, func(row []mysql.FieldValue) error {
			if fn == nil {
				return nil
			}
			return exec(row)
		}, nil)

		if err != nil {
			c.log.Errorln("FullDataExport", table, err)
			return err
		}

		cli.Execute("UNLOCK TABLES")
		cli.Execute("COMMIT")
		cli.Close()
	}
	return nil
}
