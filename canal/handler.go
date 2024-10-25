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

	pr := parser.New()
	stmt, _, err := pr.Parse(sql, "", "")
	if err != nil {
		fmt.Println(err)
		return err

	}

	node := mysql_.ParseStmt(stmt[0])

	return h.syncer.SetHandlerOnDDL(node, string(queryEvent.Schema), sql)

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
