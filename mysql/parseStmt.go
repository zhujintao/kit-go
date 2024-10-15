package mysql

import "github.com/pingcap/tidb/pkg/parser/ast"

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
