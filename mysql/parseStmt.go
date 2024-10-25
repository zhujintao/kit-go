package mysql

import (
	"github.com/pingcap/tidb/pkg/parser/ast"
)

type table struct {
	Schema string
	Name   string
}

type parser struct {
	Tables   []*table
	ddlction string
}

func (p *parser) IsDDlAction() bool {
	return p.ddlction != ""
}

func ParseSql(schema string, stmt ast.StmtNode) *parser {

	p := &parser{}
	schemaName := schema
	switch st := stmt.(type) {
	case *ast.RenameTableStmt:
		p.ddlction = "RenameTableStmt"
		for _, t := range st.TableToTables {
			if t.OldTable.Schema.O != "" {
				schemaName = t.OldTable.Schema.O
			}
			p.Tables = append(p.Tables, &table{Name: t.OldTable.Name.O, Schema: schemaName})
		}
	case *ast.AlterTableStmt:
		p.ddlction = "AlterTableStmt"
		if st.Table.Schema.O != "" {
			schemaName = st.Table.Schema.O
		}
		p.Tables = append(p.Tables, &table{Name: st.Table.Name.O, Schema: schemaName})
	case *ast.DropTableStmt:
		p.ddlction = "DropTableStmt"
		for _, t := range st.Tables {
			if t.Schema.O != "" {
				schemaName = t.Schema.O
			}
			p.Tables = append(p.Tables, &table{Name: t.Name.O, Schema: schemaName})
		}
	case *ast.CreateTableStmt:
		p.ddlction = "CreateTableStmt"
		if st.Table.Schema.O != "" {
			schemaName = st.Table.Schema.O
		}
		p.Tables = append(p.Tables, &table{Name: st.Table.Name.O, Schema: schemaName})
	case *ast.TruncateTableStmt:
		p.ddlction = "TruncateTableStmt"
		if st.Table.Schema.O != "" {
			schemaName = st.Table.Schema.O
		}
		p.Tables = append(p.Tables, &table{Name: st.Table.Name.O, Schema: schemaName})

	case *ast.CreateDatabaseStmt:
		p.ddlction = "CreateDatabaseStmt"
		if st.Name.O != "" {
			schemaName = st.Name.O
		}
		p.Tables = append(p.Tables, &table{Schema: schemaName})
	case *ast.DropDatabaseStmt:
		p.ddlction = "DropDatabaseStmt"
		if st.Name.O != "" {
			schemaName = st.Name.O
		}
		p.Tables = append(p.Tables, &table{Schema: schemaName})
	}
	return p
}

type node struct {
	Db    string
	Table string
}

type Nodes struct {
	StmtType string
	Nodes    []*node
}

func ParseStmt(stmt ast.StmtNode) (ns *Nodes) {

	switch t := stmt.(type) {
	case *ast.RenameTableStmt:
		var nodes []*node
		for _, tableInfo := range t.TableToTables {
			n := &node{
				Db:    tableInfo.OldTable.Schema.String(),
				Table: tableInfo.OldTable.Name.String(),
			}
			nodes = append(nodes, n)
		}
		ns = &Nodes{
			StmtType: "RenameTable",
			Nodes:    nodes,
		}
	case *ast.AlterTableStmt:

		n := &node{

			Db:    t.Table.Schema.String(),
			Table: t.Table.Name.String(),
		}
		ns = &Nodes{
			StmtType: "AlterTable",
			Nodes:    []*node{n},
		}
	case *ast.DropTableStmt:
		var nodes []*node
		for _, table := range t.Tables {
			n := &node{

				Db:    table.Schema.String(),
				Table: table.Name.String(),
			}
			nodes = append(nodes, n)
		}
		ns = &Nodes{
			StmtType: "DropTable",
			Nodes:    nodes,
		}
	case *ast.CreateTableStmt:
		n := &node{
			Db:    t.Table.Schema.String(),
			Table: t.Table.Name.String(),
		}

		ns = &Nodes{
			StmtType: "CreateTable",
			Nodes:    []*node{n},
		}
	case *ast.TruncateTableStmt:
		n := &node{

			Db:    t.Table.Schema.String(),
			Table: t.Table.Name.String(),
		}
		ns = &Nodes{
			StmtType: "TruncateTable",
			Nodes:    []*node{n},
		}

	}
	return ns
}
