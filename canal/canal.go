package canal

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-mysql-org/go-mysql/canal"
	"github.com/go-mysql-org/go-mysql/mysql"
	gomysql "github.com/zhujintao/kit-go/mysql"
)

const (
	UpdateAction = "update"
	InsertAction = "insert"
	DeleteAction = "delete"
)

type RowsEvent = canal.RowsEvent

type syncer struct {
	// e.RowsEvent
	//
	// beforeRows := e.Rows[0]
	//
	// afterRows := e.Rows[1]
	SetHandlerOnRow func(e *RowsEvent) error

	SetHandlerOnDDL func(action DDlAction, schema, sql string) error

	syncCh chan interface{}
	ctx    context.Context
	canal  *canal.Canal
	cancel context.CancelFunc
	wg     sync.WaitGroup
	master *masterInfo
	cfg    *canal.Config
}

// parse includeTables excludeTables (high priority)
func ParseMatchTable(s *[]string, schema, table string) {
	*s = append(*s, fmt.Sprintf(`%s\.%s$`, schema, table))
}

// delete source code 90~93
//
// includeTables, excludeTables use ParseMatchTable method, db.table db.table$ db.table1 db.table2
// masterInfoPath (*option default is current path)
func NewCanal(ctx context.Context, cancel context.CancelFunc, id, addr, user, passwrod string, includeTables, excludeTables []string, masterInfoPath ...string) *syncer {

	cfg := canal.NewDefaultConfig()
	cfg.Addr = addr
	cfg.User = user
	cfg.Password = passwrod
	fmt.Println(includeTables, excludeTables)
	cfg.IncludeTableRegex = includeTables
	cfg.ExcludeTableRegex = excludeTables

	c, err := canal.NewCanal(cfg)
	if err != nil {
		fmt.Println("NewCanal", err)
		return nil
	}

	s := &syncer{canal: c, cfg: cfg}
	s.syncCh = make(chan interface{}, 4096)

	s.ctx = ctx
	s.cancel = cancel

	mpath := "."
	if len(masterInfoPath) == 1 {
		mpath = masterInfoPath[0]
	}

	masterinfo, err := loadMasterInfo(filepath.Join(mpath, id))
	if err != nil {
		fmt.Println("loadMasterInfo", err)
		return nil
	}

	s.master = masterinfo

	c.SetEventHandler(&defaultHandler{syncer: s})

	return s
}
func (s *syncer) GetMasterInfo(path, id string) *masterInfo {

	masterinfo, err := loadMasterInfo(filepath.Join(path, id))
	if err != nil {
		fmt.Println("loadMasterInfo", err)
		return nil
	}
	return masterinfo
}
func (s *syncer) Execute(cmd string, args ...interface{}) (rr *mysql.Result, err error) {
	return s.canal.Execute(cmd, args...)
}
func (s *syncer) ExecuteSelectStreaming(cmd string, perRowCallback func(row []gomysql.FieldValue) error, perResultCallback func(result *mysql.Result) error) (err error) {
	return s.canal.ExecuteSelectStreaming(cmd, perRowCallback, perResultCallback)
}
func (s *syncer) GetMasterGTIDSet() (mysql.GTIDSet, error) {
	return s.canal.GetMasterGTIDSet()
}
func (s *syncer) SetMasterInfo(gset string) {

	s.master.Save(gset)

}
func (s *syncer) CheckTableMatch(key string) bool {
	return s.canal.CheckTableMatch(key)

}

func (s *syncer) Run() error {

	s.wg.Add(1)
	go s.writeMasterInfo()
	gset, _ := mysql.ParseMysqlGTIDSet(s.master.GtidSet)
	if err := s.canal.StartFromGTID(gset); err != nil {
		return err
	}

	return nil

}

func (s *syncer) Ctx() context.Context {
	return s.ctx
}
func (s *syncer) Close() {

	s.cancel()
	s.canal.Close()
	s.master.Close()
	s.wg.Wait()

}

func (s *syncer) writeMasterInfo() {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	defer s.wg.Done()
	lastSavedTime := time.Now()
	var gset string

	for {

		var needSavePos bool
		select {

		case v := <-s.syncCh:
			switch v := v.(type) {
			case gsetSaver:
				now := time.Now()
				if v.force || now.Sub(lastSavedTime) > 3*time.Second {
					lastSavedTime = now
					needSavePos = true
					gset = v.gset
				}
			}
		case <-ticker.C:
			needSavePos = true
		case <-s.ctx.Done():
			return
		}
		if needSavePos {
			if err := s.master.Save(gset); err != nil {
				fmt.Printf("save sync gset %s err %v, close sync\n", gset, err)
				s.cancel()
				return
			}
		}

	}

}

func DefaultOnRow(e *RowsEvent) error {
	dml := &gomysql.DmlDefault{}
	switch e.Action {
	case InsertAction:
		s, v := dml.Insert(e.Table, e.Rows[0])
		fmt.Println(s, v)
	case UpdateAction:
		s, v := dml.Update(e.Table, e.Rows[0], e.Rows[1])
		fmt.Println(s, v)
	case DeleteAction:
		s, v := dml.Delete(e.Table, e.Rows[0])
		fmt.Println(s, v)
	default:
		return fmt.Errorf("invalid rows action %s", e.Action)
	}

	return nil
}

func (s *syncer) GetAllCreateSql() []string {

	query := "select table_schema as database_name, table_name from information_schema.tables where table_type != 'view'  order by database_name, table_name"

	r, err := s.Execute(query)
	if err != nil {
		return nil
	}

	var creates []string

	for _, row := range r.Values {

		var indb map[string]bool = map[string]bool{}
		db := string(row[0].AsString())
		table := string(row[1].AsString())
		if !s.CheckTableMatch(db + "." + table) {
			continue
		}

		if _, ok := indb[db]; !ok {

			if sql := s.fetchDBCreateSql(db); sql != "" {
				creates = append(creates, sql)
				indb[db] = true
			}

		}

		if sql := s.fetchTableCreteSql(db, table); sql != "" {
			creates = append(creates, sql)
		}

	}
	return creates

}

func (s *syncer) fetchTableCreteSql(db, table string) string {
	query := "SHOW CREATE TABLE " + db + "." + table
	r, err := s.Execute(query)
	if err != nil {
		return ""
	}
	ss, err := r.GetString(0, 1)
	if err != nil {
		return ""
	}

	return strings.ReplaceAll(ss, fmt.Sprintf("CREATE TABLE `%s`", table), fmt.Sprintf("CREATE TABLE `%s`.`%s`", db, table))

}

func (s *syncer) fetchDBCreateSql(db string) string {

	query := "SHOW CREATE DATABASE " + db
	r, err := s.Execute(query)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	a, err := r.GetString(0, 1)

	if err != nil {
		fmt.Println(err)
		return ""
	}
	return a

}
