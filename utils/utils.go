package utils

import (
	"strconv"
)

func CheckNil(s interface{}) bool {

	return s == nil

}

/*
可转换的类型

	 s	    t
	 string  -> float64
		     -> bool
	 float64 -> float64
	 bool	 -> bool
*/
func ToType(s interface{}, t string) interface{} {

	switch v := s.(type) {
	case string:

		switch t {
		case "int32":
			if v, err := strconv.Atoi(v); err == nil {
				return int32(v)
			}

		case "float64":

			if v, err := strconv.ParseFloat(v, 64); err == nil {

				return v
			}
		case "float32":

			if v, err := strconv.ParseFloat(v, 32); err == nil {

				return float32(v)
			}

		case "bool":
			if v, err := strconv.ParseBool(v); err == nil {
				return v
			}
		case "string":
			return v
		}

	case float64:

		switch t {
		case "float64":
			return v
		case "string":
			return strconv.Itoa(int(v))

		case "float32":

			return float32(v)
		}

	case bool:
		switch t {
		case "bool":
			return v
		}
	case int64:
		switch t {
		case "int32":
			return int32(v)

		case "string":
			return strconv.Itoa(int(v))
		case "float64":
			return float64(v)

		}

	}
	return nil

}
