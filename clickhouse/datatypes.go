package clickhouse

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	libcolumn "github.com/ClickHouse/clickhouse-go/v2/lib/column"
	mysqlschema "github.com/go-mysql-org/go-mysql/schema"
	"github.com/zhujintao/kit-go/mysql"
)

func ScanIRowFromMysql(columns []mysql.TableColumn, fields []string, row []any, dest []any, fixValue func(col mysqlschema.TableColumn, v *any)) error {
	for idx, col := range columns {

		if !slices.Contains(fields, col.Name) {
			continue
		}

		value := row[idx]
		b, err := parseType(col.Type, col.RawType, col.Name, value == nil)
		if err != nil {
			fmt.Println(err, col.Type)
			return err
		}

		if col.Type == mysqlschema.TYPE_TIMESTAMP || col.Type == mysqlschema.TYPE_DATETIME {

			if v, ok := value.([]byte); ok {
				value = string(v)
			}
		}

		if fixValue != nil {
			fixValue(col, &value)
		}

		err = b.AppendRow(value)
		if err != nil {
			fmt.Println(err)
			return err
		}
		dest[idx] = reflect.New(b.ScanType()).Interface()

		err = b.ScanRow(dest[idx], 0)
		if err != nil {
			fmt.Println(err)
			return err
		}

		//vv := reflect.ValueOf(dest[idx]).Elem()
		//fmt.Printf("%v \t%T", vv, value)

	}

	return nil
}
func ScanRowFromMysql(columns []mysql.TableColumn, fields []string, row []mysql.FieldValue, dest []any, fixValue func(col mysqlschema.TableColumn, v *any)) error {
	for idx, col := range columns {

		if !slices.Contains(fields, col.Name) {
			continue
		}

		value := row[idx].Value()

		b, err := parseType(col.Type, col.RawType, col.Name, value == nil)
		if err != nil {
			fmt.Println(err, col.Type)
			return err
		}

		if col.Type == mysqlschema.TYPE_TIMESTAMP || col.Type == mysqlschema.TYPE_DATETIME {
			if v, ok := value.([]byte); ok {
				value = string(v)
			}
		}

		if fixValue != nil {
			fixValue(col, &value)
		}

		err = b.AppendRow(value)
		if err != nil {
			fmt.Println(err)
			return err
		}
		dest[idx] = reflect.New(b.ScanType()).Interface()

		err = b.ScanRow(dest[idx], 0)
		if err != nil {
			fmt.Println(err)
			return err
		}

		//vv := reflect.ValueOf(dest[idx]).Elem()
		//fmt.Println(vv)
	}

	return nil

}

func parseType(mysql_type int, mysql_rawtype string, fieldName string, nullable bool) (libcolumn.Interface, error) {

	var base libcolumn.Interface

	switch mysql_type {
	case mysqlschema.TYPE_STRING:
		base = &libcolumn.String{}
	case mysqlschema.TYPE_NUMBER:
		base = &libcolumn.Int32{}
	case mysqlschema.TYPE_FLOAT:
		base = &libcolumn.Float64{}
	case mysqlschema.TYPE_DATETIME, mysqlschema.TYPE_TIMESTAMP:
		base, _ = libcolumn.Type("DateTime").Column(fieldName, time.Local)
	case mysqlschema.TYPE_DATE:
		base = &libcolumn.Date{}
	case mysqlschema.TYPE_ENUM:
		base = &libcolumn.Enum16{}
	case mysqlschema.TYPE_JSON:
		base = &libcolumn.JSON{}
	case mysqlschema.TYPE_DECIMAL:
		a := strings.ToUpper(mysql_rawtype[:1]) + strings.ToLower(mysql_rawtype[1:])
		base, _ = libcolumn.Type(a).Column(fieldName, time.Local)

	default:
		return nil, fmt.Errorf("mysql_type no match found: %d", mysql_type)
	}

	var field libcolumn.Type
	if base == nil {
		return nil, fmt.Errorf("type error")
	}
	field = base.Type()
	if nullable {
		field = libcolumn.Type("Nullable(" + (base).Type() + ")")
	}

	return field.Column(fieldName, time.Local)

}

var ckypemps map[int]string = map[int]string{
	mysqlschema.TYPE_NUMBER:    "Int32",
	mysqlschema.TYPE_FLOAT:     "Float64",
	mysqlschema.TYPE_BINARY:    "FixedString",
	mysqlschema.TYPE_DATETIME:  "DateTime",
	mysqlschema.TYPE_TIMESTAMP: "DateTime",
	mysqlschema.TYPE_DATE:      "Date",
	mysqlschema.TYPE_STRING:    "String",
	mysqlschema.TYPE_ENUM:      "Enum8",
	mysqlschema.TYPE_DECIMAL:   "Decimal(",
	mysqlschema.TYPE_JSON:      "JSON",
}
