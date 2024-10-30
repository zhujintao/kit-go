package mysql

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

type DmlInterface interface {
	Insert(tableInfo *TableInfo, row []interface{}) (string, []interface{})
	Update(tableInfo *TableInfo, beforeRows, afterRows []interface{}) (string, []interface{})
	Delete(tableInfo *TableInfo, beforeRows, afterRows []interface{}) (string, []interface{})
}

type DmlDefault struct {
	DmlInterface
}

func (d *DmlDefault) Insert(tableInfo *TableInfo, row []interface{}) (string, []interface{}) {

	db := tableInfo.Schema
	table := tableInfo.Name
	field := make([]string, len(row))
	pos := make([]string, len(row))
	value := make([]interface{}, len(row))

	for idx, v := range row {

		if v == nil {
			continue
		}
		field[idx] = tableInfo.Columns[idx].Name
		pos[idx] = "?"
		vstr := ValueToString(&tableInfo.Columns[idx], v)
		value[idx] = vstr

	}
	value = delNilI(value)
	sql := "insert into " + db + "." + table + " ( " + strings.Join(delNilS(field), ",") + ") values ( " + strings.Join(delNilS(pos), ",") + ")"

	return sql, value

}

// canal.RowsEvent
//
// beforeRows := e.Rows[0]
//
// afterRows := e.Rows[1]
func (d *DmlDefault) Update(tableInfo *TableInfo, beforeRows, afterRows []interface{}) (string, []interface{}) {
	return updateAndDelete("update", tableInfo, beforeRows, afterRows)
}

// canal.RowsEvent
//
// beforeRows := e.Rows[0]
//
// afterRows := e.Rows[1]
func (d *DmlDefault) Delete(tableInfo *TableInfo, beforeRows, afterRows []interface{}) (string, []interface{}) {
	return updateAndDelete("delete", tableInfo, beforeRows, afterRows)

}
func updateAndDelete(action string, tableInfo *TableInfo, beforeRows, afterRows []interface{}) (string, []interface{}) {

	db := tableInfo.Schema
	table := tableInfo.Name

	pkpos := make([]string, len(tableInfo.Columns))
	pos := make([]string, len(tableInfo.Columns))

	pkvalue := make([]interface{}, len(tableInfo.Columns))
	value := make([]interface{}, len(tableInfo.Columns))

	for idx, field := range tableInfo.Columns {

		if slices.Contains(tableInfo.PKColumns, idx) {
			vstr := ValueToString(&field, beforeRows[idx])
			pkvalue[idx] = vstr
			pkpos[idx] = field.Name + " = ?"
		}
		if action == "update" && reflect.DeepEqual(beforeRows[idx], afterRows[idx]) {
			continue
		}
		pos[idx] = field.Name + " = ?"
		vstr := fmt.Sprintf("%v", ValueToString(&field, afterRows[idx]))
		if afterRows[idx] == nil {
			pos[idx] = field.Name + " = NULL"
			continue
		}

		value[idx] = vstr

	}

	pkvalue = delNilI(pkvalue)
	value = delNilI(value)
	value = append(value, pkvalue...)

	if action == "update" {
		sql := "update " + db + "." + table + " set " + strings.Join(delNilS(pos), ",") + " where " + strings.Join(delNilS(pkpos), " AND ")
		return sql, value
	}

	if action == "delete" {
		sql := "DELETE FROM " + tableInfo.Schema + "." + tableInfo.Name + " WHERE " + strings.Join((delNilS(pkpos)), " AND ")

		return sql, pkvalue
	}

	return "", nil

}

func delNilI(s []interface{}) []interface{} {
	var n []interface{}
	for _, s_ := range s {
		if s_ == nil {
			continue
		}
		n = append(n, s_)
	}
	return n
}
func delNilS(s []string) []string {
	var n []string
	for _, s_ := range s {
		if s_ == "" {
			continue
		}
		n = append(n, s_)
	}
	return n

}
