package clickhouse

import (
	"strings"
	"time"

	"github.com/zhujintao/kit-go/mysql"
)

type DmlClickhouse struct {
	mysql.DmlInterface
}

const (
	delKey     = "_del"
	versionKey = "_version"
	dataDelete = 1
	dataInsert = 0
)

func (d *DmlClickhouse) Insert(tableInfo *mysql.TableInfo, row []interface{}, dataVersion ...uint64) (string, []interface{}) {
	time.Now().UnixMicro()
	dv := uint64(time.Now().UnixMicro())
	if len(dataVersion) == 1 {
		dv = dataVersion[0]
	}
	return onCkInsert(tableInfo, row, false, 0, dv)

}

func (d *DmlClickhouse) Update(tableInfo *mysql.TableInfo, beforeRows, afterRows []interface{}, dataVersion ...uint64) []interface{} {

	dv := uint64(time.Now().UnixMicro())
	if len(dataVersion) == 1 {
		dv = dataVersion[0]
	}

	var l []interface{}
	s, v := onCkInsert(tableInfo, beforeRows, false, dataDelete, dv)
	del := []interface{}{s, v}
	l = append(l, del)
	s, v = onCkInsert(tableInfo, afterRows, true, dataInsert, dv)
	ins := []interface{}{s, v}
	l = append(l, ins)

	return l
}

func (d *DmlClickhouse) Delete(tableInfo *mysql.TableInfo, row []interface{}, dataVersion ...uint64) (string, []interface{}) {

	dv := uint64(time.Now().UnixMicro())
	if len(dataVersion) == 1 {
		dv = dataVersion[0]
	}

	return onCkInsert(tableInfo, row, false, dataDelete, dv)

}

func onCkInsert(tableInfo *mysql.TableInfo, row []interface{}, addNull bool, isdel int, version uint64) (string, []interface{}) {

	db := tableInfo.Schema
	table := tableInfo.Name
	value := make([]interface{}, len(row))
	field := make([]string, len(row))
	pos := make([]string, len(row))
	for idx, col := range tableInfo.Columns {

		if row[idx] == nil && !addNull {
			continue
		}
		field[idx] = col.Name
		pos[idx] = "?"
		vstr := mysql.ValueToString(&tableInfo.Columns[idx], row[idx])
		value[idx] = vstr
		if row[idx] == nil && addNull {
			pos[idx] = "NULL"
			value[idx] = nil
		}
	}
	value = mysql.DelNilI(value)
	field = mysql.DelNilS(field)
	pos = mysql.DelNilS(pos)
	pos = append(pos, "?", "?")
	field = append(field, delKey, versionKey)
	value = append(value, isdel, version)
	sql := "insert into " + db + "." + table + " (" + strings.Join(field, ",") + ") values (" + strings.Join(pos, ",") + ")"
	return sql, value
}

func (d *DmlClickhouse) Update2(tableInfo *mysql.TableInfo, beforeRows, afterRows []interface{}) (string, []interface{}) {
	dataVersion := time.Now().Unix()
	row := beforeRows
	db := tableInfo.Schema
	table := tableInfo.Name

	field := make([]string, len(row))
	apos := make([]string, len(row))
	bpos := make([]string, len(row))

	avalue := make([]interface{}, len(row))
	bvalue := make([]interface{}, len(row))
	var values []interface{}

	for idx, col := range tableInfo.Columns {

		//if reflect.DeepEqual(beforeRows[idx], afterRows[idx]) {
		//	continue
		//}
		field[idx] = col.Name
		bvstr := mysql.ValueToString(&tableInfo.Columns[idx], beforeRows[idx])
		bvalue[idx] = bvstr
		bpos[idx] = "?"

		if beforeRows[idx] == nil {
			bpos[idx] = "NULL"
			bvalue[idx] = nil
		}

		avstr := mysql.ValueToString(&tableInfo.Columns[idx], afterRows[idx])
		avalue[idx] = avstr
		apos[idx] = "?"
		if afterRows[idx] == nil {
			apos[idx] = "NULL"
			avalue[idx] = nil
		}

	}
	field = mysql.DelNilS(field)

	bvalue = mysql.DelNilI(bvalue)
	avalue = mysql.DelNilI(avalue)

	bpos = mysql.DelNilS(bpos)
	apos = mysql.DelNilS(apos)

	bpos = append(bpos, "?", "?")
	apos = append(apos, "?", "?")

	field = append(field, delKey, versionKey)

	bvalue = append(bvalue, dataDelete, dataVersion)
	avalue = append(avalue, dataInsert, dataVersion)

	sql := "insert into " + db + "." + table + " (" + strings.Join(field, ",") + ") values (" + strings.Join(bpos, ",") + "),(" + strings.Join(apos, ",") + ")"

	values = append(values, bvalue...)
	values = append(values, avalue...)
	return sql, values

}
