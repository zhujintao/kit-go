package canal

import (
	"fmt"

	"github.com/go-mysql-org/go-mysql/canal"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/pingcap/tidb/pkg/parser"
)

type defaultHandler struct {
	syncer *syncer
}

type gsetSaver struct {
	gset  string
	force bool
}

func (h *defaultHandler) OnRotate(header *replication.EventHeader, rotateEvent *replication.RotateEvent) error {

	return h.syncer.ctx.Err()
}

func (h *defaultHandler) OnTableChanged(header *replication.EventHeader, schema string, table string) error {

	return h.syncer.ctx.Err()
}

func (h *defaultHandler) OnDDL(header *replication.EventHeader, nextPos mysql.Position, queryEvent *replication.QueryEvent) error {

	if queryEvent.GSet != nil {
		h.syncer.syncCh <- gsetSaver{queryEvent.GSet.String(), true}
	}

	if h.syncer.ddlFn == nil {
		return h.syncer.ctx.Err()
	}

	sql := string(queryEvent.Query)
	schema := string(queryEvent.Schema)

	pr := parser.New()
	stmt, err := pr.ParseOneStmt(sql, "", "")
	if err != nil {
		h.syncer.Close()
		return err
	}
	t := parseSql(schema, stmt)

	if !t.ddlFilter(h.syncer.ddlacl) {
		fmt.Println("skip ddl:", t.ddlaction)
		return nil
	}

	for _, table := range t.Tables {

		key := table.Schema + "." + table.Name
		// add CheckTableMatch to source code 284
		// func (c *Canal) CheckTableMatch(key string) bool {
		//	return c.checkTableMatch(key)
		//	}
		if !h.syncer.canal.CheckTableMatch(key) {
			continue
		}

		err := h.syncer.ddlFn(t.ddlaction, table.Schema, sql)

		if err != nil {
			h.syncer.Close()
			return err
		}

	}

	return nil

}

func (h *defaultHandler) OnXID(header *replication.EventHeader, nextPos mysql.Position) error {

	return h.syncer.ctx.Err()
}

func (h *defaultHandler) OnPosSynced(header *replication.EventHeader, pos mysql.Position, set mysql.GTIDSet, force bool) error {

	if set == nil {
		return fmt.Errorf("OnPosSynced GTIDSet error")
	}

	h.syncer.syncCh <- gsetSaver{set.String(), true}
	return h.syncer.ctx.Err()
}

func (h *defaultHandler) OnRowsQueryEvent(e *replication.RowsQueryEvent) error {

	return nil
}
func (h *defaultHandler) OnGTID(header *replication.EventHeader, gtidEvent mysql.BinlogGTIDEvent) error {

	return h.syncer.ctx.Err()
}
func (h *defaultHandler) String() string {
	return ""
}

func (h *defaultHandler) OnRow(e *canal.RowsEvent) error {

	if h.syncer.rowFn != nil {

		return h.syncer.rowFn(e)
	}
	return h.syncer.ctx.Err()
}
