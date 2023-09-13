package mysql

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-mysql-org/go-mysql/client"
)

func RewriteMysqlQueryColum(c *client.Conn, database, table string) string {

	r, err := c.Execute("SELECT COLUMN_NAME AS column_name, COLUMN_TYPE AS column_type FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = 'ncc' AND TABLE_NAME = 'hl_loan' ORDER BY ORDINAL_POSITION")
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
	queryColumnsStr := queryColumns.String()
	return queryColumnsStr[0 : len(queryColumnsStr)-1]

}

func backQuote(s string) string {

	return fmt.Sprintf("`%s`", s)
}
