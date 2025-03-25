package canal

import (
	"slices"

	"github.com/pingcap/tidb/pkg/parser/ast"
)

type table struct {
	Schema string
	Name   string
}

type parserx struct {
	Tables    []*table
	ddlaction DDlAction
}

// ast.AlterTableType 1
//
// custom 1000
type DDlAction = ast.AlterTableType

const (
	CreateDatabase DDlAction = 1000 + iota + 1
	DropDatabase
	RenameTable
	CreateTable
	AlterTable
	DropTable
	TruncateTable
)

func (p *parserx) ddlFilter(a *ddlAcl) bool {

	if len(a.exclude) == 0 && len(a.include) == 0 {
		return true
	}
	matchFlag := false

	if slices.Contains(a.include, p.ddlaction) {
		matchFlag = true
	}
	if slices.Contains(a.exclude, p.ddlaction) {
		matchFlag = false
	}

	return matchFlag

}

func parseSql(schema string, stmt ast.StmtNode) *parserx {

	p := &parserx{}
	schemaName := schema
	switch st := stmt.(type) {
	case *ast.RenameTableStmt:
		p.ddlaction = RenameTable
		for _, t := range st.TableToTables {
			if t.OldTable.Schema.O != "" {
				schemaName = t.OldTable.Schema.O
			}
			p.Tables = append(p.Tables, &table{Name: t.OldTable.Name.O, Schema: schemaName})
		}
	case *ast.AlterTableStmt:

		p.ddlaction = AlterTable
		if st.Table.Schema.O != "" {
			schemaName = st.Table.Schema.O
		}
		p.Tables = append(p.Tables, &table{Name: st.Table.Name.O, Schema: schemaName})
		for _, spec := range st.Specs {
			p.ddlaction = spec.Tp //ast.AlterTableType((reflect.ValueOf(spec.Tp).Int()))
		}

	case *ast.DropTableStmt:
		p.ddlaction = DropTable

		for _, t := range st.Tables {
			if t.Schema.O != "" {
				schemaName = t.Schema.O
			}
			p.Tables = append(p.Tables, &table{Name: t.Name.O, Schema: schemaName})
		}

	case *ast.CreateTableStmt:
		p.ddlaction = CreateTable

		if st.Table.Schema.O != "" {
			schemaName = st.Table.Schema.O
		}
		p.Tables = append(p.Tables, &table{Name: st.Table.Name.O, Schema: schemaName})
	case *ast.TruncateTableStmt:
		p.ddlaction = TruncateTable

		if st.Table.Schema.O != "" {
			schemaName = st.Table.Schema.O
		}
		p.Tables = append(p.Tables, &table{Name: st.Table.Name.O, Schema: schemaName})

	case *ast.CreateDatabaseStmt:
		p.ddlaction = CreateDatabase

		if st.Name.O != "" {
			schemaName = st.Name.O
		}
		p.Tables = append(p.Tables, &table{Schema: schemaName, Name: "*"})
	case *ast.DropDatabaseStmt:
		p.ddlaction = DropDatabase

		if st.Name.O != "" {
			schemaName = st.Name.O
		}
		p.Tables = append(p.Tables, &table{Schema: schemaName, Name: "*"})
	}

	return p
}
