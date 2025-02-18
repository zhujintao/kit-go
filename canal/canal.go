package canal

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"math/rand"

	"github.com/go-mysql-org/go-mysql/canal"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/schema"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	gomysql "github.com/zhujintao/kit-go/mysql"
)

type RowsEvent = canal.RowsEvent
type Table = schema.Table

type syncer struct {
	// e.RowsEvent
	//
	// beforeRows := e.Rows[0]
	//
	// afterRows := e.Rows[1]

	rowFn func(e *RowsEvent) error

	ddlFn func(action DDlAction, schema, sql string) error

	syncCh chan interface{}
	ctx    context.Context
	canal  *canal.Canal
	cancel context.CancelFunc
	wg     sync.WaitGroup
	master *masterInfo
	cfg    *canal.Config
	path   string
	ddlacl *ddlAcl
}

type ddlAcl struct {
	include []DDlAction
	exclude []DDlAction
}

func (d *ddlAcl) Include(a ...DDlAction) {

	d.include = append(d.include, a...)

}
func (d *ddlAcl) Exclude(a ...DDlAction) {
	d.exclude = append(d.exclude, a...)
}

var defaultDDl []DDlAction = []DDlAction{
	CreateDatabase,
	DropDatabase,
	CreateTable,
	DropTable,
	RenameTable,
	TruncateTable,
	ast.AlterTableAddColumns,
	ast.AlterTableDropColumn,
	ast.AlterTableDropIndex,
	ast.AlterTableChangeColumn,
	ast.AlterTableModifyColumn,
	ast.AlterTableOption,
}

func GetDefaultDDl() []DDlAction {

	return defaultDDl
}

func (d *ddlAcl) DefaultDDl() *ddlAcl {

	d.include = defaultDDl
	return d

}

func (s *syncer) SetHandlerOnDDL(fn func(action DDlAction, schema, sql string) error) *ddlAcl {

	s.ddlFn = fn
	s.ddlacl = &ddlAcl{}
	return s.ddlacl

}

func (s *syncer) SetHandlerOnRow(fn func(e *RowsEvent) error) {
	s.rowFn = fn
}

// parse includeTables excludeTables (high priority)
func ParseMatchTable(s *[]string, schema, table string) {
	*s = append(*s, fmt.Sprintf(`%s\.%s$`, schema, table))
}

type Master struct {
	Addr      string
	User      string
	Password  string
	StorePath string
}

type Slave Master

type filterTable struct {
	include []string
	exclude []string
}

func FilterTable() *filterTable {

	return &filterTable{}
}

// [mysql\\..*]  is 'mysql' database all tables
func (f *filterTable) Include(table ...string) *filterTable {

	f.include = append(f.include, table...)
	return f

}
func (f *filterTable) Exclude(table ...string) *filterTable {

	f.exclude = append(f.exclude, table...)
	return f

}

func New(id string, cfg Master, filter *filterTable) *syncer {

	s := &syncer{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	dialer := &net.Dialer{}
	//streamHandler, _ := log.NewStreamHandler(os.Stdout)
	config := &canal.Config{
		Addr:              cfg.Addr,
		User:              cfg.User,
		Password:          cfg.Password,
		Charset:           mysql.DEFAULT_CHARSET,
		ServerID:          uint32(rand.New(rand.NewSource(time.Now().Unix())).Intn(1000)) + 1001,
		Flavor:            mysql.DEFAULT_FLAVOR,
		Dialer:            dialer.DialContext,
		IncludeTableRegex: filter.include,
		ExcludeTableRegex: filter.exclude,
		ReadTimeout:       time.Hour * 24,

		//Logger:   log.NewDefault(streamHandler),
	}
	c, err := canal.NewCanal(config)
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}

	s.cfg = config
	s.syncCh = make(chan interface{}, 4096)
	s.canal = c
	//s.ddlacl = &ddlAcl{}
	s.path = filepath.Join(cfg.StorePath, id)
	master, err := loadMasterInfo(s.path)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	s.master = master
	c.SetEventHandler(&defaultHandler{syncer: s})

	return s
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

func (s *syncer) GetPath() string {
	return s.path

}
func (s *syncer) GetMasterInfo(path string) *masterInfo {

	masterinfo, err := loadMasterInfo(path)
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

	//
	//
	return s.canal.ExecuteSelectStreaming(cmd, perRowCallback, perResultCallback)
}
func (s *syncer) GetMasterGTIDSet() (mysql.GTIDSet, error) {
	return s.canal.GetMasterGTIDSet()
}

type setgset struct {
	syncer *syncer
	gset   []string
}

func (s *setgset) Force() {
	if len(s.gset) == 1 {
		s.syncer.master.Save(s.gset[0])
	}

}

func (s *syncer) SetGTID(gset ...string) *setgset {

	if s.master.Gtidset() == "" && len(gset) == 1 {
		s.master.Save(gset[0])
	}

	if s.master.Gtidset() == "" && len(gset) == 0 {
		g, _ := s.canal.GetMasterGTIDSet()
		s.master.Save(g.String())
	}
	return &setgset{syncer: s, gset: gset}

}
func (s *syncer) CheckTableMatch(key string) bool {
	return s.canal.CheckTableMatch(key)

}

func (s *syncer) Run() error {

	s.wg.Add(1)
	go s.writeMasterInfo()
	gset, _ := mysql.ParseMysqlGTIDSet(s.master.GtidSet)
	if err := s.canal.StartFromGTID(gset); err != nil {

		s.Close()
		fmt.Println("Run", err)
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

func CallbackHandlerOnRow(insert func(tableInfo *Table, row []interface{}) error, update func(tableInfo *Table, beforeRows []interface{}, afterRows []interface{}) error, delete func(tableInfo *Table, row []interface{}) error) func(e *RowsEvent) error {

	return func(e *RowsEvent) error {

		switch e.Action {
		case canal.InsertAction:
			if insert == nil {
				return nil
			}
			err := insert(e.Table, e.Rows[0])
			if err != nil {
				return err
			}

		case canal.UpdateAction:
			if update == nil {
				return nil
			}
			err := update(e.Table, e.Rows[0], e.Rows[1])
			if err != nil {
				return err
			}
		case canal.DeleteAction:
			if delete == nil {
				return nil
			}
			err := delete(e.Table, e.Rows[0])
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid rows action %s", e.Action)
		}

		return nil

	}

}

func DefaultHandlerOnRow(e *RowsEvent) error {
	dml := &gomysql.DmlDefault{}
	switch e.Action {
	case canal.InsertAction:
		s, v := dml.Insert(e.Table, e.Rows[0])
		fmt.Println(e.Header.LogPos, s, v)
	case canal.UpdateAction:
		s, v := dml.Update(e.Table, e.Rows[0], e.Rows[1])
		fmt.Println(e.Header.LogPos, s, v)
	case canal.DeleteAction:
		s, v := dml.Delete(e.Table, e.Rows[0])
		fmt.Println(e.Header.LogPos, s, v)
	default:
		return fmt.Errorf("invalid rows action %s", e.Action)
	}

	return nil
}

// semlimit fetch table concurrent size
// where is sql where
func (s *syncer) FetchFullData(semlimit int64, write func(tableInfo *gomysql.TableInfo, row []interface{}) error, where ...string) error {

	if len(s.GetMasterInfo(s.path).Gtidset()) != 0 {
		fmt.Println("inc data")
		return nil
	}

	fmt.Println("full data")

	query := "select table_schema as database_name, table_name from information_schema.tables where table_type != 'view'  order by database_name, table_name"
	s.SetGTID()
	r, err := s.Execute(query)
	if err != nil {
		return fmt.Errorf("Execute(query): %v", err)
	}

	var dbs map[string][]string = map[string][]string{}
	for _, row := range r.Values {

		db := string(row[0].AsString())
		table := string(row[1].AsString())

		if !s.CheckTableMatch(db + "." + table) {
			continue
		}

		dbs[db] = append(dbs[db], table)

	}

	eg, ctx := errgroup.WithContext(s.ctx)
	for db, tables := range dbs {
		eg.Go(func() error {
			limiter := semaphore.NewWeighted(semlimit)

			for _, table := range tables {
				if err := limiter.Acquire(ctx, 1); err != nil {
					fmt.Println("limiter.Acquire", err)
					return err
				}

				eg.Go(func() error {

					var select_all bytes.Buffer
					c := gomysql.NewClient(&gomysql.Config{Addr: s.cfg.Addr, User: s.cfg.User, Password: s.cfg.Password})
					r, err := c.Execute("SELECT COLUMN_NAME AS column_name, COLUMN_TYPE AS column_type FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION", db, table)
					if err != nil {

						fmt.Println("Execute(SELECT COLUMN_NAME AS): ", err)
						return fmt.Errorf("Execute(SELECT COLUMN_NAME AS): %v", err)

					}
					select_all.WriteString("SELECT ")
					for i, row := range r.Values {
						if i > 0 {
							select_all.WriteString(",")
						}
						column_name := string(row[0].AsString())
						column_type := string(row[1].AsString())

						if strings.HasPrefix(column_type, "set") {

							select_all.WriteString(backQuote(column_name) + " + 0")

						} else {
							select_all.WriteString(backQuote(column_name))
						}
					}
					select_all.WriteString(" FROM ")
					select_all.WriteString(backQuote(db))
					select_all.WriteString(".")
					select_all.WriteString(backQuote(table))

					for _, s := range where {
						select_all.WriteString(s)
					}

					sql := select_all.String()
					tableInfo, err := s.canal.GetTable(db, table)
					if err != nil {
						limiter.Release(1)
						fmt.Println("GetTable", err)
						return err
					}

					err = c.ExecuteSelectStreaming(sql, func(row []gomysql.FieldValue) error {
						if write == nil {
							return nil
						}
						var _row []interface{}
						for _, v := range row {

							// modify source code delete b.WriteByte('\'')
							_row = append(_row, string(v.String()))
						}
						return write(tableInfo, _row)

					}, nil)
					limiter.Release(1)
					if err != nil {
						fmt.Println(db, tableInfo, "-", err)
						return err
					}

					fmt.Println(db, tableInfo, "- ok")

					return nil
				})

			}
			return nil
		})

	}

	return eg.Wait()

}
func backQuote(s string) string {
	return fmt.Sprintf("`%s`", s)
}

// skip table, not create
func (s *syncer) GetAllCreateSql(skip ...string) []string {

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
		key := db + "." + table
		if !s.CheckTableMatch(key) {
			continue
		}

		//reg.MatchString(key)

		if slices.Contains(skip, db) || slices.Contains(skip, key) {
			continue
		}
		flag := false
		for _, s := range skip {

			k, err := regexp.Compile(s)
			if err != nil {
				fmt.Println("skip ", err)
				continue
			}
			if k.MatchString(key) {
				flag = true
			}

		}
		if flag {
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
