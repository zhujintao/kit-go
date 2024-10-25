package canal

import (
	"fmt"

	"github.com/go-mysql-org/go-mysql/canal"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/pingcap/tidb/pkg/parser"
	mysql_ "github.com/zhujintao/kit-go/mysql"
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

	//h.syncer.syncCh <- gsetSaver{queryEvent.GSet.String(), true}
	if h.syncer.SetHandlerOnDDL == nil {
		return h.syncer.ctx.Err()
	}

	sql := string(queryEvent.Query)
	schema := string(queryEvent.Schema)
	pr := parser.New()
	stmt, err := pr.ParseOneStmt(sql, "", "")
	if err != nil {
		fmt.Println(err)
		return err
	}
	t := mysql_.ParseSql(schema, stmt)

	if !t.IsDDlAction() {
		return nil
	}
	for _, table := range t.Tables {

		key := table.Schema + "." + table.Name
		if h.syncer.canal.CheckTableMatch(key) {

			err := h.syncer.SetHandlerOnDDL(schema, sql)
			if err != nil {
				fmt.Println(err)
				return err
			}

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

	if h.syncer.SetHandlerOnRow != nil {

		return h.syncer.SetHandlerOnRow(e)
	}
	return h.syncer.ctx.Err()
}
