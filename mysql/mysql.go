package mysql

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/schema"
)

type FieldValue = mysql.FieldValue
type TableInfo = schema.Table
type Dialer = client.Dialer

// type Conn = client.Conn
type SelectPerRowCallback func(client *Conn, table *TableInfo, row []mysql.FieldValue) error

type Config struct {
	Addr         string
	User         string
	Password     string
	PoolMaxAlive int
	Dialer       client.Dialer
	TLSConfig    *tls.Config
}

func RewriteMysqlQueryColumn(c *Conn, schema, table string) string {

	r, err := c.Execute("SELECT COLUMN_NAME AS column_name, COLUMN_TYPE AS column_type FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION", schema, table)
	if err != nil {
		return ""
	}

	var queryColumns bytes.Buffer

	for _, row := range r.Values {

		column_name := string(row[0].AsString())
		column_type := string(row[1].AsString())

		if strings.HasPrefix(column_type, "set") {

			queryColumns.WriteString(backQuote(column_name) + " + 0")

		} else {
			queryColumns.WriteString(backQuote(column_name))
		}

		queryColumns.WriteString(",")

	}
	if queryColumns.Len() == 0 {
		fmt.Println(schema, table, "not exist")
		return ""
	}

	queryColumnsStr := queryColumns.String()

	return queryColumnsStr[0 : len(queryColumnsStr)-1]

}

func backQuote(s string) string {
	return fmt.Sprintf("`%s`", s)
}

func FetchTablesCreateQuery(c *client.Conn, database_name string, fetch_tables []string, tables_create_query map[string]string) error {

	for _, fetch_table_name := range fetch_tables {

		sql := "SHOW CREATE TABLE " + backQuote(database_name) + "." + backQuote(fetch_table_name)

		r, err := c.Execute(sql)
		if err != nil {
			return err
		}
		for _, row := range r.Values {

			s := strings.ReplaceAll(string(row[1].AsString()), fmt.Sprintf("CREATE TABLE `%s`", fetch_table_name), fmt.Sprintf("CREATE TABLE `%s`.`%s`", database_name, fetch_table_name))

			tables_create_query[string(row[0].AsString())] = s
		}
	}
	return nil

}

func GetDbCreateSql(c *Conn, dbname string) string {

	r, err := c.Execute("SHOW CREATE DATABASE " + backQuote(dbname))
	if err != nil {
		return ""
	}
	return string(r.Values[0][1].AsString())

}

func GetTableCreateSql(c *Conn, dbname, tablename string) string {

	r, err := c.Execute("SHOW CREATE TABLE " + backQuote(dbname) + "." + backQuote(tablename))
	if err != nil {
		return ""
	}

	return strings.ReplaceAll(string(r.Values[0][1].AsString()), fmt.Sprintf("CREATE TABLE `%s`", tablename), fmt.Sprintf("CREATE TABLE `%s`.`%s`", dbname, tablename))

}

func FetchDbCreateQuery(c *Conn, fetch_dbs []string, dbs_create_query map[string]string) error {

	for _, fetch_db_name := range fetch_dbs {

		r, err := c.Execute("SHOW CREATE DATABASE " + backQuote(fetch_db_name))
		if err != nil {
			return err
		}
		for _, row := range r.Values {

			dbs_create_query[string(row[0].AsString())] = string(row[1].AsString())
		}
	}
	return nil

}

func FetchTableSchema(c *Conn, database, table string, tables_schema map[string]*schema.Table) error {

	s, err := schema.NewTable(c, database, table)
	if err != nil {
		fmt.Println(err)
		return err
	}

	tables_schema[table] = s

	return nil

}
func FetchMasterStatus(c *Conn) (string, error) {

	r, err := c.Execute("SHOW MASTER STATUS;")

	if err != nil {

		return "", err
	}

	binlog_file, _ := r.GetStringByName(0, "File")
	binlog_position, _ := r.GetIntByName(0, "Position")
	binlog_do_db, _ := r.GetStringByName(0, "Binlog_Do_DB")
	binlog_ignore_db, _ := r.GetStringByName(0, "Binlog_Ignore_DB")
	executed_gtid_set, _ := r.GetStringByName(0, "Executed_Gtid_Set")
	data_version := 1

	fmt.Println(data_version, binlog_file, binlog_position, binlog_do_db, binlog_ignore_db, executed_gtid_set)
	return executed_gtid_set, nil
}

func ValueToString(col *schema.TableColumn, value interface{}) string {
	vv := valueType(col, value)
	return fmt.Sprintf("%v", vv)
}

func valueType(col *schema.TableColumn, value interface{}) interface{} {
	switch col.Type {
	case schema.TYPE_ENUM:
		switch value := value.(type) {
		case int64:
			// for binlog, ENUM may be int64, but for dump, enum is string
			eNum := value - 1
			if eNum < 0 || eNum >= int64(len(col.EnumValues)) {
				// we insert invalid enum value before, so return empty
				fmt.Printf("invalid binlog enum index %d, for enum %v\n", eNum, col.EnumValues)
				return ""
			}

			return col.EnumValues[eNum]
		}
	case schema.TYPE_SET:
		switch value := value.(type) {
		case int64:
			// for binlog, SET may be int64, but for dump, SET is string
			bitmask := value
			sets := make([]string, 0, len(col.SetValues))
			for i, s := range col.SetValues {
				if bitmask&int64(1<<uint(i)) > 0 {
					sets = append(sets, s)
				}
			}
			return strings.Join(sets, ",")
		}
	case schema.TYPE_BIT:
		switch value := value.(type) {
		case string:
			// for binlog, BIT is int64, but for dump, BIT is string
			// for dump 0x01 is for 1, \0 is for 0
			if value == "\x01" {
				return int64(1)
			}

			return int64(0)
		}
	case schema.TYPE_STRING:
		switch value := value.(type) {
		case []byte:
			return string(value[:])
		}
	case schema.TYPE_JSON:
		var f interface{}
		var err error
		switch v := value.(type) {
		case string:
			err = json.Unmarshal([]byte(v), &f)
		case []byte:
			err = json.Unmarshal(v, &f)
		}
		if err == nil && f != nil {
			return f
		}
		/*
			case schema.TYPE_DATETIME, schema.TYPE_TIMESTAMP:
				switch v := value.(type) {
				case string:
					vt, err := time.ParseInLocation(mysql.TimeFormat, string(v), time.Local)
					if err != nil || vt.IsZero() { // failed to parse date or zero date
						return nil
					}
					return vt.Format(time.RFC3339)
				}

					case schema.TYPE_DATE:
						switch v := value.(type) {
						case string:
							vt, err := time.Parse(mysqlDateFormat, string(v))
							if err != nil || vt.IsZero() { // failed to parse date or zero date
								return nil
							}
							return vt.Format(mysqlDateFormat)
						}
		*/
	}

	return value
}

func ErrorCode(errMsg string) (code int) {
	var tmpStr string
	// golang scanf doesn't support %*,so I used a temporary variable
	fmt.Sscanf(errMsg, "%s%d", &tmpStr, &code)
	return
}

func ParseConfDSN(cfg *Config, dsn string) error {
	pos := strings.LastIndex(dsn, "@")
	if pos == -1 {
		return fmt.Errorf("dsn format error: user:password@ip:port")
	}
	account := dsn[:pos]
	address := dsn[pos+1:]
	pos = strings.Index(account, ":")
	user := account[:pos]
	password := account[pos+1:]
	cfg.Addr = address
	cfg.User = user
	cfg.Password = password
	return nil
}
