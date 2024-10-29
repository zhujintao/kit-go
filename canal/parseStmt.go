package canal

import (
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

type DDlAction string

const (
	CreateDatabase DDlAction = "CreateDatabase"
	DropDatabase   DDlAction = "DropDatabase"
	CreateTable    DDlAction = "CreateTable"
	DropTable      DDlAction = "DropTable"
	RenameTable    DDlAction = "RenameTable"
	TruncateTable  DDlAction = "TruncateTable"
	AlterTable     DDlAction = "AlterTable"
	AddColumn      DDlAction = "AddColumn"
	DropColumn     DDlAction = "DropColumn"
	DropIndex      DDlAction = "DropIndex"
)

func (p *parserx) IsAction() bool {
	return p.ddlaction != ""
}

func (p *parserx) GetAction() DDlAction {
	return p.ddlaction
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
			switch spec.Tp {
			case ast.AlterTableDropIndex:
				p.ddlaction = DropIndex
			case ast.AlterTableAddColumns:
				p.ddlaction = AddColumn
			case ast.AlterTableDropColumn:
				p.ddlaction = DropColumn
			}
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
