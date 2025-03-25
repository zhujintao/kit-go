package canal

import (
	"fmt"
	"strings"

	"github.com/zhujintao/kit-go/mysql"
)

func FullDataExport(conf *Config, dbs []string, gtidSet GTIDSet, fn func(tableInfo *mysql.TableInfo) func(row []mysql.FieldValue) error) error {
	cli := mysql.NewClient(&mysql.Config{Addr: conf.Addr, User: conf.User, Password: conf.Password})
	cli.Execute("SET @master_heartbeat_period=1")
	//lock table
	cli.Execute("FLUSH /*!40101 LOCAL */ TABLES")
	cli.Execute("FLUSH TABLES WITH READ LOCK")
	cli.Execute("SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ")
	cli.Execute("START TRANSACTION /*!40100 WITH CONSISTENT SNAPSHOT */")

	// check gitd mode
	r, _ := cli.Execute("SHOW VARIABLES LIKE 'gtid_mode'")
	fmt.Println(r.GetString(0, 1))

	// get gitd EXECUTED
	r, _ = cli.Execute("SELECT @@GLOBAL.GTID_EXECUTED")
	fmt.Println(r.GetString(0, 0))

	r, _ = cli.Execute("SHOW MASTER STATUS")
	fmt.Println(r.GetString(0, 4))
	fmt.Println()

	r, _ = cli.Execute("SELECT @@GLOBAL.GTID_EXECUTED")
	set, _ := r.GetString(0, 0)
	gtidSet.Update(set)

	// unlock table
	_, err := cli.Execute("UNLOCK TABLES")
	fmt.Println(err)

	defer func() {
		cli.Execute("UNLOCK TABLES")
		defer cli.Execute("COMMIT")
		defer cli.Close()
	}()

	for _, db := range dbs {
		st := strings.Split(db, ".")
		sql := "select * from " + db + " limit 1"
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
			return err
		}
	}

	return err
}
