package clickhouse

import (
	"fmt"
	"math"
	"strings"
	"unsafe"

	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/format"
	"github.com/pingcap/tidb/pkg/parser/mysql"
	"github.com/pingcap/tidb/pkg/parser/types"
	_ "github.com/pingcap/tidb/pkg/types/parser_driver"
)

var mappedTypes map[string]string = map[string]string{

	"tinyint":   "Int8",
	"smallint":  "Int16",
	"mediumint": "Int32",
	"int":       "Int32",
	"integer":   "Int32",
	"bigint":    "Int64",
	"float":     "Float32",
	"double":    "Float64",
	"timestamp": "DateTime",
	"boolean":   "Bool",
	"bit":       "UInt64",
	"set":       "UInt64",
	"year":      "Uint16",
	"Time":      "Int64",
	"date":      "Date32",
	"geometry":  "String",
	"varchar":   "String",
	"char":      "String",
	"text":      "String",
	"decimal":   "Decimal",
}

type column struct {
	name                      string
	dataType                  string
	increment                 bool
	comment                   string
	nullable                  bool
	primaryKey, index, unique bool
	precision                 int
	scale                     int
}

type table struct {
	schema      string
	name        string
	comment     string
	colpos      map[string]int
	columns     []*column
	storage     string
	versionName string
	orders      []string
	partition   string
}

func ParserMysqlCreateTableSql(sql string) string {

	pr := parser.New()
	stmt, err := pr.ParseOneStmt(sql, "", "")
	if err != nil {
		return ""
	}
	st := stmt.(*ast.CreateTableStmt)

	t := &table{}
	getName(t, st.Table)
	getColumns(t, st.Cols)
	getConstraint(t, st.Constraints)
	getStorage(t, st.Options)
	getOrderByPolicy((t))
	getPartitionPolicy(t)

	var sb strings.Builder
	s := format.NewRestoreCtx(format.DefaultRestoreFlags, &sb)

	s.WriteKeyWord("CREATE TABLE ")
	if st.IfNotExists {
		s.WriteKeyWord("IF NOT EXISTS ")
	}
	if !s.Flags.HasWithoutSchemaNameFlag() {

		if t.schema != "" {
			s.WriteName(t.schema)
			s.WritePlain(".")
		}
	}
	s.WriteName(t.name)
	s.WritePlainf(" (\n")

	for i, col := range t.columns {
		if i > 0 {
			s.WritePlainf(",\n")
		}
		s.WritePlain("  ")
		s.WriteName(col.name)
		s.WritePlain(" ")

		dataType := col.dataType
		if col.nullable {
			dataType = "Nullable(" + dataType + ")"
		}
		s.WritePlain(dataType)
		s.WritePlain(" ")
		if col.comment != "" {
			s.WriteKeyWord("COMMENT ")
			s.WriteString(col.comment)
		}

	}
	s.WritePlain(",\n")
	s.WritePlainf("  INDEX %s %s TYPE minmax GRANULARITY 1", t.versionName, t.versionName)
	s.WritePlainf("\n)\n")
	s.WriteKeyWord("ENGINE ")
	s.WritePlainf("%s(%s)", t.storage, t.versionName)
	s.WritePlain("\n")

	if t.partition != "" {
		s.WriteKeyWord("PARTITION BY ")
		s.WritePlain(t.partition)
		s.WritePlain("\n")
	}

	s.WriteKeyWord("ORDER BY ")

	if len(t.orders) > 0 {
		s.WritePlain("(")
		for i, col := range t.orders {

			if i > 0 {
				s.WritePlainf(",\n")
			}
			s.WritePlain(col)
		}
		s.WritePlain(")")
	} else {
		s.WritePlain("tuple()")
	}
	return sb.String()
}

func getName(table *table, t *ast.TableName) {

	table.schema = t.Schema.String()
	table.name = t.Name.String()

}
func getColumns(table *table, cols []*ast.ColumnDef) {

	table.columns = make([]*column, len(cols))
	table.colpos = map[string]int{}

	for i, c := range cols {
		colName := c.Name.Name.O
		table.colpos[colName] = i
		col := &column{
			name:     colName,
			nullable: true,
		}

		ft := c.Tp

		col.dataType = mappedTypes[types.TypeToStr(ft.GetType(), ft.GetCharset())]
		col.precision = types.UnspecifiedLength
		col.scale = types.UnspecifiedLength

		switch ft.GetType() {
		case mysql.TypeEnum, mysql.TypeSet:
			fmt.Println("(")
			for i, e := range ft.GetElems() {
				if i != 0 {
				}
				fmt.Println(e)
			}
			fmt.Println(")")
		case mysql.TypeTimestamp, mysql.TypeDatetime, mysql.TypeDuration:
			col.precision = ft.GetDecimal()
		case mysql.TypeUnspecified, mysql.TypeFloat, mysql.TypeDouble, mysql.TypeNewDecimal:
			col.precision = ft.GetFlen()
			col.scale = ft.GetDecimal()
		default:
			//precision = ft.GetFlen()

		}

		for _, opt := range c.Options {

			switch opt.Tp {

			case ast.ColumnOptionNotNull:
				col.nullable = false
			case ast.ColumnOptionAutoIncrement:
				col.increment = true
			case ast.ColumnOptionComment:
				col.comment = opt.Expr.(ast.ValueExpr).GetString()
			}
		}
		table.columns[i] = col
	}

	sign_colName := getUniqueColumnName(table.colpos, "_sign")
	version_colName := getUniqueColumnName(table.colpos, "_version")
	table.columns = append(table.columns, &column{name: sign_colName, dataType: "Int8 MATERIALIZED 1"})
	table.columns = append(table.columns, &column{name: version_colName, dataType: "UInt64 MATERIALIZED 1"})
	table.versionName = version_colName
}

func getUniqueColumnName(cols map[string]int, prefix string) string {

	isUnique := func(prefix string) bool {
		_, ok := cols[prefix]
		return !ok
	}
	if isUnique(prefix) {
		return prefix
	}

	for index := 0; ; index++ {
		curName := fmt.Sprintf("%s_%d", prefix, index)
		if isUnique(curName) {
			return curName
		}
	}
}

func findColumn(table *table, colName string) *column {
	if i, ok := table.colpos[colName]; ok {
		return table.columns[i]
	}
	return nil
}
func getConstraint(table *table, constraints []*ast.Constraint) {

	for _, c := range constraints {

		for _, key := range c.Keys {
			colName := key.Column.Name.String()
			col := findColumn(table, colName)
			if col == nil {
				continue
			}
			switch c.Tp {
			case ast.ConstraintPrimaryKey:
				col.primaryKey = true
				col.nullable = false
			case ast.ConstraintKey, ast.ConstraintIndex:
				col.index = true
			case ast.ConstraintUniqKey:
				col.unique = true
			}

		}
	}

}

func getStorage(table *table, opts []*ast.TableOption) {
	table.storage = "ReplacingMergeTree"
	for _, t := range opts {

		switch t.Tp {
		case ast.TableOptionComment:
			table.comment = t.StrValue
		}
	}

}

func getOrderByPolicy(table *table) {

	var orderbycol []string

	var incrementKey []string
	var nonincrementKey []string

	for _, col := range table.columns {
		colName := col.name
		if col.nullable {
			colName = "assumeNotNull(" + colName + ")"
		}
		if col.primaryKey || col.index || col.unique {
			if col.increment {

				if col.nullable {
				}
				incrementKey = append(incrementKey, colName)
			} else {
				nonincrementKey = append(nonincrementKey, colName)
			}
		}
	}

	orderbycol = append(orderbycol, nonincrementKey...)
	orderbycol = append(orderbycol, incrementKey...)
	table.orders = append(table.orders, orderbycol...)

}
func getPartitionPolicy(table *table) {

	numbers_partition := func(column_name string, type_max_size uint) string {

		if type_max_size <= 1000 {
			return column_name
		}

		return fmt.Sprintf("intDiv(%s,%d)", column_name, type_max_size/1000)
	}

	for _, col := range table.columns {
		if !col.primaryKey {
			continue
		}
		switch col.dataType {
		case "Date32", "DateTime":
			table.partition = fmt.Sprintf("toYYYYMM(%s)", col.name)
		case "Int8", "Uint8":
			table.partition = numbers_partition(col.name, math.MaxUint8)
		case "Int16", "Uint16":
			table.partition = numbers_partition(col.name, math.MaxUint16)
		case "Int32", "Uint32":
			table.partition = numbers_partition(col.name, math.MaxUint32)
		case "Int64", "Uint64":
			table.partition = numbers_partition(col.name, math.MaxUint64)
		}
	}

}

func getSizwOfValueInMemory(i interface{}) int {

	return int(unsafe.Sizeof(i))

}
