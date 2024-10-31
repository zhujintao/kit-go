package clickhouse

import (
	"github.com/zhujintao/kit-go/mysql"
)

type DmlClickhouse struct {
	mysql.DmlInterface
}

func (d *DmlClickhouse) Insert(tableInfo *mysql.TableInfo, row []interface{}) (string, []interface{}) {

}

func (d *DmlClickhouse) Update(tableInfo *mysql.TableInfo, beforeRows, afterRows []interface{}) (string, []interface{}) {

}
func (d *DmlClickhouse) Delete(tableInfo *mysql.TableInfo, row []interface{}) (string, []interface{}) {

}
