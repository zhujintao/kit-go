package jq

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/itchyny/gojq"
)

func trimSpace(s string) string {

	var ss []rune
	for _, r := range s {
		if unicode.IsSpace(r) {

			continue

		}

		ss = append(ss, r)

	}
	return string(ss)

}

// text is json string
//
// jsonPath get f1 and f2 path a:b:c:f1,f2 or {"a":"b":"c": "f1","f2"}
func PickJson(text string, jsonPath string) string {

	var runePath []rune
	if trimSpace(jsonPath) == "{}" {
		jsonPath = ""
		runePath = append(runePath, '.')
	}

	for _, r := range jsonPath {

		if unicode.IsSpace(r) {
			continue
		}
		if r == '{' {
			continue
		}
		if r == '}' {
			continue
		}
		if r == '"' {
			continue
		}
		runePath = append(runePath, r)
	}

	jsonPath = string(runePath)
	if jsonPath == "." {
		return parserJson(text, ".")
	}
	sl := strings.Split(jsonPath, ":")
	var sb strings.Builder
	sb.WriteString(".")

	if len(sl) > 1 {

		var jpath string
		for i, v := range sl[:len(sl)-1] {

			jpath = jpath + "." + v
			if i == 0 {
				jpath = v
			}
			sb.WriteString(fmt.Sprintf(`|=with_entries(select(.key == "%s"))|.%s`, v, jpath))
		}
	}

	keys := strings.Split(sl[len(sl)-1], ",")
	var key string
	for i, v := range keys {
		kv := `"` + v + `"`
		key = key + "," + kv
		if i == 0 {
			key = kv
		}
	}
	sb.WriteString(fmt.Sprintf(`|=with_entries(select(.key | IN (%s)))`, key))
	return parserJson(text, sb.String())

}

func parserJson(input, expression string) string {
	var obj map[string]any
	var s string
	json.Unmarshal([]byte(input), &obj)
	query, err := gojq.Parse(expression)
	if err != nil {
		fmt.Println("Parse", err)
		return ""
	}
	iter := query.Run(obj)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			fmt.Println("v.(error)", err)
			break
		}
		sv, err := json.Marshal(v)
		if err != nil {
			fmt.Println("json.Marshal", err)
			break
		}
		s = string(sv)

	}
	return s
}
