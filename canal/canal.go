package canal

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-mysql-org/go-mysql/canal"
	cmysql "github.com/go-mysql-org/go-mysql/mysql"
	mysql "github.com/zhujintao/kit-go/mysql"
)

const (
	UpdateAction = "update"
	InsertAction = "insert"
	DeleteAction = "delete"
)

type RowsEvent = canal.RowsEvent

type syncer struct {
	SetHandlerOnRow func(e *RowsEvent) error
	SetHandlerOnDDL func(schema, sql string) error

	syncCh chan interface{}
	ctx    context.Context
	canal  *canal.Canal
	cancel context.CancelFunc
	wg     sync.WaitGroup
	master *masterInfo
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

	s := &syncer{canal: c}
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
func (s *syncer) Execute(cmd string, args ...interface{}) (rr *cmysql.Result, err error) {
	return s.canal.Execute(cmd, args...)
}
func (s *syncer) ExecuteSelectStreaming(cmd string, perRowCallback func(row []mysql.FieldValue) error, perResultCallback func(result *cmysql.Result) error) (err error) {
	return s.canal.ExecuteSelectStreaming(cmd, perRowCallback, perResultCallback)
}
func (s *syncer) GetMasterGTIDSet() (cmysql.GTIDSet, error) {
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
	gset, _ := cmysql.ParseMysqlGTIDSet(s.master.GtidSet)
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
