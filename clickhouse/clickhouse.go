package clickhouse

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unsafe"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/format"
	"github.com/pingcap/tidb/pkg/parser/mysql"
	"github.com/pingcap/tidb/pkg/parser/types"
)

var mappedTypes map[string]string = map[string]string{

	"tinyint":    "Int8",
	"smallint":   "Int16",
	"mediumint":  "Int32",
	"int":        "Int32",
	"integer":    "Int32",
	"bigint":     "Int64",
	"float":      "Float32",
	"double":     "Float64",
	"timestamp":  "DateTime",
	"datetime":   "DateTime",
	"boolean":    "Bool",
	"bit":        "UInt64",
	"set":        "UInt64",
	"year":       "Uint16",
	"Time":       "Int64",
	"date":       "Date32",
	"geometry":   "String",
	"varchar":    "String",
	"char":       "String",
	"text":       "String",
	"blob":       "String",
	"mediumtext": "String",
	"mediumblob": "String",
	"longtext":   "String",
	"decimal":    "Decimal",
	"enum":       "Enum8",
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
	relativeColumn            string
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
	ddlAction   ast.AlterTableType
}

func InErrCode(err error, code ...int32) bool {

	errCode, ok := err.(*clickhouse.Exception)
	if !ok {
		return false
	}
	return slices.Contains(code, errCode.Code)

}

// parser ddl, dml
func ParserMysqlSQL(sql string) (string, error) {
	pr := parser.New()
	stmt, err := pr.ParseOneStmt(sql, "", "")
	if err != nil {
		return "", err
	}
	t := &table{}
	var sb strings.Builder
	s := format.NewRestoreCtx(format.DefaultRestoreFlags, &sb)

	switch st := stmt.(type) {
	case *ast.CreateDatabaseStmt:
		s.WriteKeyWord("CREATE DATABASE ")
		s.WriteName(st.Name.O)

	case *ast.DropDatabaseStmt:

		s.WriteKeyWord("DROP DATABASE ")
		s.WriteName(st.Name.O)

	case *ast.CreateTableStmt:
		getName(t, st.Table)
		getColumns(t, st.Cols)
		addVersionColumn(t)
		getConstraint(t, st.Constraints)
		getStorage(t, st.Options)
		err := getOrderByPolicy((t))
		if err != nil {
			return "", err
		}
		getPartitionPolicy(t)
		buildCreateTable(t, st, s)
	case *ast.DropTableStmt:
		if st.TemporaryKeyword != ast.TemporaryNone {
			break
		}
		drop := "DROP TABLE "
		if st.IsView {
			drop = "DROP VIEW "
		}
		s.WriteKeyWord(drop)
		for i, table := range st.Tables {

			if i != 0 {
				s.WritePlain(",")
			}
			getName(t, table)
			if t.schema != "" {
				s.WriteName(t.schema)
				s.WritePlain(".")
			}
			s.WriteName(t.name)
		}

	case *ast.TruncateTableStmt:

		getName(t, st.Table)
		s.WriteKeyWord("TRUNCATE TABLE ")
		if t.schema != "" {
			s.WriteName(t.schema)
			s.WritePlain(".")
		}
		s.WriteName(t.name)

	case *ast.AlterTableStmt:

		getName(t, st.Table)

		err := getAlterTableSpec(t, st.Specs)
		if err != nil {
			return "", err
		}

		s.WriteKeyWord("ALTER TABLE ")
		switch t.ddlAction {
		case ast.AlterTableAddColumns:
			AddColumns(t, s)
		case ast.AlterTableModifyColumn:
			modifyColumn(t, s)
		case ast.AlterTableDropIndex:
			dropIndex(t, s)
		case ast.AlterTableDropColumn:
			dropColumn(t, s)

		}
	default:
		return "", fmt.Errorf("ddl action unknown")
	}

	return sb.String(), nil
}

func getAlterTableSpec(t *table, specs []*ast.AlterTableSpec) error {
	t.colpos = map[string]int{}
	for i, spec := range specs {
		table := &table{}

		switch spec.Tp {
		case ast.AlterTableAddColumns:
			colName := spec.NewColumns[0].Name.Name.O
			getColumns(table, spec.NewColumns)
			t.colpos[colName] = i
			getRelativePosition(table, colName, spec.Position)
			t.ddlAction = ast.AlterTableAddColumns

		case ast.AlterTableModifyColumn:
			colName := spec.NewColumns[0].Name.Name.O
			getColumns(table, spec.NewColumns)
			t.colpos[colName] = i
			getRelativePosition(table, colName, spec.Position)
			t.ddlAction = ast.AlterTableModifyColumn

		case ast.AlterTableChangeColumn:

			return fmt.Errorf("[CHANGE COLUMN] not supported")

		//	var cols []*ast.ColumnDef

		//	colName := spec.NewColumns[0].Name.Name.O
		//	cols = append(cols, &ast.ColumnDef{Name: &ast.ColumnName{Name: model.NewCIStr(spec.OldColumnName.Name.O)}}, spec.NewColumns[0])
		//	t.colpos[colName] = 1
		//	getColumns(table, cols)
		//	getRelativePosition(table, colName, spec.Position)
		//	t.ddlAction = ast.AlterTableChangeColumn

		case ast.AlterTableDropColumn:

			var cols []*ast.ColumnDef
			cols = append(cols, &ast.ColumnDef{Name: &ast.ColumnName{Name: ast.NewCIStr(spec.OldColumnName.Name.O)}})
			getColumns(table, cols)
			t.ddlAction = ast.AlterTableDropColumn

		case ast.AlterTableRenameColumn:
			var cols []*ast.ColumnDef

			cols = append(cols, &ast.ColumnDef{Name: &ast.ColumnName{Name: ast.NewCIStr(spec.OldColumnName.Name.O)}})
			cols = append(cols, &ast.ColumnDef{Name: &ast.ColumnName{Name: ast.NewCIStr(spec.NewColumnName.Name.O)}})
			getColumns(table, cols)
			t.ddlAction = ast.AlterTableRenameColumn

		case ast.AlterTableDropIndex:

			getColumns(table, []*ast.ColumnDef{{Name: &ast.ColumnName{Name: ast.NewCIStr(spec.Name)}}})
			t.colpos[spec.Name] = i
			t.ddlAction = ast.AlterTableDropIndex

		default:
			return fmt.Errorf("ddl action unknown %T", spec.Tp)
		}

		t.columns = append(t.columns, table.columns...)

	}
	return nil

}

func AddColumns(t *table, s *format.RestoreCtx) {
	if t.schema != "" {
		s.WriteName(t.schema)
		s.WritePlain(".")
	}
	s.WriteName(t.name)
	s.WritePlain(" ")

	for i, col := range t.columns {

		if i > 0 {
			s.WritePlain(", ")
		}

		s.WriteKeyWord("ADD COLUMN ")
		s.WriteName(col.name)
		s.WritePlain(" ")
		dataType := col.dataType
		if col.nullable {
			dataType = "Nullable(" + dataType + ")"
		}
		s.WritePlain(dataType)

		if col.comment != "" {
			s.WritePlain(" ")
			s.WriteKeyWord("COMMENT ")
			s.WriteString(col.comment)
		}
		if col.relativeColumn != "" {
			s.WritePlain(" ")
			s.WritePlain(col.relativeColumn)
		}
	}
}

func dropColumn(t *table, s *format.RestoreCtx) {
	if t.schema != "" {
		s.WriteName(t.schema)
		s.WritePlain(".")
	}
	s.WriteName(t.name)
	s.WritePlain(" ")

	for i, col := range t.columns {
		if i > 0 {
			s.WritePlain(", ")
		}

		s.WriteKeyWord("DROP COLUMN ")
		s.WriteName(col.name)
	}

}

func modifyColumn(t *table, s *format.RestoreCtx) {

	if t.schema != "" {
		s.WriteName(t.schema)
		s.WritePlain(".")
	}
	s.WriteName(t.name)
	s.WritePlain(" ")
	s.WriteKeyWord("MODIFY COLUMN ")
	col := t.columns[0]

	colBuild(col, s)
}

func dropIndex(t *table, s *format.RestoreCtx) {

	if t.schema != "" {
		s.WriteName(t.schema)
		s.WritePlain(".")
	}
	s.WriteName(t.name)
	s.WritePlain(" ")

	for i, col := range t.columns {
		if i > 0 {
			s.WritePlain(", ")
		}

		s.WriteKeyWord("DROP INDEX ")
		s.WriteName(col.name)
	}

}

// not implemented
func changeColumn(t *table, s *format.RestoreCtx) {
	if t.schema != "" {
		s.WriteName(t.schema)
		s.WritePlain(".")
	}
	s.WriteName(t.name)
	s.WritePlain(" ")

	s.WriteKeyWord("CHANGE COLUMN ")
	s.WriteName(t.columns[0].name)
	s.WritePlain(" ")

	colBuild(t.columns[1], s)

}

func colBuild(col *column, s *format.RestoreCtx) {

	s.WriteName(col.name)
	s.WritePlain(" ")

	if col.dataType != "" {

		dataType := col.dataType
		if col.precision != types.UnspecifiedLength {

			dataType = fmt.Sprintf("%s(%d", dataType, col.precision)

			if col.scale != types.UnspecifiedLength {
				dataType = fmt.Sprintf("%s,%d", dataType, col.scale)

			}
			dataType = dataType + ")"
		}

		if col.nullable {
			dataType = "Nullable(" + dataType + ")"
		}
		s.WritePlain(dataType)
	}
	if col.comment != "" {
		s.WritePlain(" ")
		s.WriteKeyWord("COMMENT ")
		s.WriteString(col.comment)
	}

	if col.relativeColumn != "" {
		s.WritePlain(" ")
		s.WritePlain(col.relativeColumn)
	}

}

func buildCreateTable(t *table, st *ast.CreateTableStmt, s *format.RestoreCtx) {
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
		colBuild(col, s)

	}
	s.WritePlain(",\n")
	s.WritePlainf("  INDEX %s %s TYPE minmax GRANULARITY 1", versionKey, versionKey)
	s.WritePlainf("\n)\n")
	s.WriteKeyWord("ENGINE ")
	s.WritePlainf("%s(%s,%s)", t.storage, versionKey, delKey)
	s.WritePlain("\n")

	if t.partition != "" {
		s.WriteKeyWord("PARTITION BY ")
		s.WritePlain(t.partition)
		s.WritePlain("\n")
	}

	s.WriteKeyWord("ORDER BY ")
	if len(t.orders) == 0 {
		s.WritePlain("tuple()")
	}
	if len(t.orders) == 1 {
		s.WritePlain(t.orders[0])
	}
	if len(t.orders) > 1 {
		s.WritePlain("(")
		for i, col := range t.orders {
			if i > 0 {
				s.WritePlainf(",\n")
			}
			s.WritePlain(col)
		}
		s.WritePlain(")")
	}
	s.WritePlain("SETTINGS allow_experimental_replacing_merge_with_cleanup=1")
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

		if c.Tp != nil {

			ft := c.Tp
			col.dataType = mappedTypes[types.TypeToStr(ft.GetType(), ft.GetCharset())]
			col.precision = types.UnspecifiedLength
			col.scale = types.UnspecifiedLength

			switch ft.GetType() {
			case mysql.TypeEnum:
				var sb strings.Builder
				for i, e := range ft.GetElems() {
					if i != 0 {
						sb.WriteString(",")
					}
					sb.WriteString(fmt.Sprintf("'%s'=%d", e, i+1))
				}
				enum := "Enum8"
				if len(ft.GetElems()) > 127 {
					enum = "Enum16"
				}
				col.dataType = enum + "(" + sb.String() + ")"

			case mysql.TypeTimestamp, mysql.TypeDatetime, mysql.TypeDuration:
				col.precision = ft.GetDecimal()
			case mysql.TypeUnspecified, mysql.TypeFloat, mysql.TypeDouble, mysql.TypeNewDecimal:
				col.precision = ft.GetFlen()
				col.scale = ft.GetDecimal()
			default:
				//precision = ft.GetFlen()

			}
		}
		for _, opt := range c.Options {

			switch opt.Tp {

			case ast.ColumnOptionNotNull:
				col.nullable = false
			case ast.ColumnOptionAutoIncrement:
				col.increment = true
			case ast.ColumnOptionComment:
				col.comment = opt.Expr.(ast.ValueExpr).GetString()
			case ast.ColumnOptionPrimaryKey:
				col.primaryKey = true
			}
		}

		table.columns[i] = col
	}

}

func addVersionColumn(table *table) {
	//sign_colName := getUniqueColumnName(table.colpos, delKey)
	//version_colName := getUniqueColumnName(table.colpos, versionKey)
	table.columns = append(table.columns, &column{name: delKey, dataType: "UInt8 MATERIALIZED 1", scale: types.UnspecifiedLength, precision: types.UnspecifiedLength})
	table.columns = append(table.columns, &column{name: versionKey, dataType: "UInt64 MATERIALIZED 1", scale: types.UnspecifiedLength, precision: types.UnspecifiedLength})
	table.versionName = versionKey
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

func getOrderByPolicy(table *table) error {

	var orders []string
	var backs []string
	var fronts []string
	var pkname string
	var pknum int

	for _, col := range table.columns {

		colName := col.name
		if slices.Contains(orders, colName) {
			continue
		}

		if !col.primaryKey && !col.index && !col.unique {
			continue
		}

		if col.nullable {
			colName = "assumeNotNull(" + colName + ")"
		}

		if col.primaryKey {
			pkname = colName
			pknum++
		}

		if col.increment {
			backs = append(backs, colName)
		} else {
			fronts = append(fronts, colName)
		}

		orders = append(orders, colName)

	}

	if pknum == 0 {

		return fmt.Errorf("error: %s %s lost primary key", table.schema, table.name)
	}

	table.orders = append(table.orders, fronts...)
	table.orders = append(table.orders, backs...)
	if pknum == 1 && len(orders) == 1 {
		table.orders = []string{"tuple(" + pkname + ")"}
	}
	return nil

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

func getRelativePosition(table *table, colName string, position *ast.ColumnPosition) {
	if position != nil {
		col := findColumn(table, colName)
		var relativeColumn string
		switch position.Tp {
		case ast.ColumnPositionAfter:
			relativeColumn = "AFTER " + position.RelativeColumn.Name.O
		case ast.ColumnPositionFirst:
			relativeColumn = "FIRST"
		}
		col.relativeColumn = relativeColumn
	}
}
