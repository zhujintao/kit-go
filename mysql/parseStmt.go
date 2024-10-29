package mysql

import (
	"slices"

	"github.com/pingcap/tidb/pkg/parser/ast"
)

type table struct {
	Schema string
	Name   string
}

type parser struct {
	Tables []*table

	ddlaction []DdlAction
}

func (p *parser) IsAction() bool {
	return len(p.ddlaction) != 0
}

type DdlAction int

const (
	CreateDatabase DdlAction = iota + 1
	DropDatabase
	CreateTable
	DropTable
	RenameTable
	TruncateTable
	AlterTable
	AddColumn
	DropColumn
	DropIndex
)

func (p *parser) GetAction() []DdlAction {
	return p.ddlaction
}

func (p *parser) IsVaild(action ...DdlAction) bool {
	var ok bool

	if len(action) == 0 {
		ok = len(p.ddlaction) != 0
	}
	for _, a := range action {
		ok = slices.Contains(p.ddlaction, a)
	}
	return ok

}

func ParseSql(schema string, stmt ast.StmtNode) *parser {

	p := &parser{}
	schemaName := schema
	switch st := stmt.(type) {
	case *ast.RenameTableStmt:
		p.ddlaction = []DdlAction{RenameTable}
		for _, t := range st.TableToTables {
			if t.OldTable.Schema.O != "" {
				schemaName = t.OldTable.Schema.O
			}
			p.Tables = append(p.Tables, &table{Name: t.OldTable.Name.O, Schema: schemaName})
		}
	case *ast.AlterTableStmt:

		p.ddlaction = []DdlAction{AlterTable}
		if st.Table.Schema.O != "" {
			schemaName = st.Table.Schema.O
		}
		p.Tables = append(p.Tables, &table{Name: st.Table.Name.O, Schema: schemaName})
		for _, spec := range st.Specs {
			switch spec.Tp {
			case ast.AlterTableDropIndex:
				p.ddlaction = append(p.ddlaction, DropIndex)
			case ast.AlterTableAddColumns:
				p.ddlaction = append(p.ddlaction, AddColumn)
			case ast.AlterTableDropColumn:
				p.ddlaction = append(p.ddlaction, DropColumn)
			}
		}
	case *ast.DropTableStmt:
		p.ddlaction = []DdlAction{DropTable}

		for _, t := range st.Tables {
			if t.Schema.O != "" {
				schemaName = t.Schema.O
			}
			p.Tables = append(p.Tables, &table{Name: t.Name.O, Schema: schemaName})
		}

	case *ast.CreateTableStmt:
		p.ddlaction = []DdlAction{CreateTable}

		if st.Table.Schema.O != "" {
			schemaName = st.Table.Schema.O
		}
		p.Tables = append(p.Tables, &table{Name: st.Table.Name.O, Schema: schemaName})
	case *ast.TruncateTableStmt:
		p.ddlaction = []DdlAction{TruncateTable}

		if st.Table.Schema.O != "" {
			schemaName = st.Table.Schema.O
		}
		p.Tables = append(p.Tables, &table{Name: st.Table.Name.O, Schema: schemaName})

	case *ast.CreateDatabaseStmt:
		p.ddlaction = []DdlAction{CreateDatabase}

		if st.Name.O != "" {
			schemaName = st.Name.O
		}
		p.Tables = append(p.Tables, &table{Schema: schemaName, Name: "*"})
	case *ast.DropDatabaseStmt:
		p.ddlaction = []DdlAction{DropDatabase}

		if st.Name.O != "" {
			schemaName = st.Name.O
		}
		p.Tables = append(p.Tables, &table{Schema: schemaName, Name: "*"})
	}
	return p
}
