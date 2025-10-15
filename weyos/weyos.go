package weyos

import (
	"reflect"
	"strings"
)

type SNat struct {
	Description  string `weyos:"name"`
	Status       string `weyos:"enabled"`
	Priority     string `weyos:"priority"`
	WanInterface string `weyos:"wan"`
	Protocol     string `weyos:"proto"`
	LanIps       string `weyos:"lanip_range"`
	Schedule     string `weyos:"time_range"`
	Failover     string `weyos:"no_change"`
	Logging      string `weyos:"log_enabled"`
	L7Protocol   string `weyos:"l7proto"`
	Thdtype      string `weyos:"thd_type"`
	Opt          string `weyos:"opt"`
}

type DNat struct {
	Status       string `weyos:"enabled"`
	Description  string `weyos:"name"`
	Protocol     string `weyos:"proto"`
	AllowedSrcIp string `weyos:"src_ip"`
	WanPort      string `weyos:"ext_port"`
	LanPort      string `weyos:"int_port"`
	LanIp        string `weyos:"int_ip"`
	WanInterface string `weyos:"wan"`
}

func UnmarshalDNAT(s string, v any) {
	unmarshal(s, v, unmkDnat)
}

func UnmarshalSNAT(s string, v any) {
	unmarshal(s, v, unmkSnat)
}

func MarshalDNAT(v any) string {
	return marshal(v, mkDnat)
}

func marshal(v any, f func(numField int, v reflect.Value, ss *[]string) func(ss []string) string) string {

	var ss []string
	value := reflect.ValueOf(v)

	var call func(ss []string) string
	if value.Kind() == reflect.Slice {

		for i := range value.Len() {

			value := value.Index(i)
			call = f(value.NumField(), value, &ss)
		}
		return call(ss)
	}

	call = f(value.NumField(), value, &ss)
	return call(ss)

}

func mkDnat(numField int, v reflect.Value, ss *[]string) func(ss []string) string {
	var s []string = make([]string, 8)
	t := v.Type()
	for i := range numField {

		field := t.Field(i)
		tag := field.Tag.Get("weyos")
		value := v.Field(i)

		switch tag {
		case "enabled":
			s[0] = value.String()
		case "proto":
			s[1] = value.String()
		case "src_ip":
			s[2] = value.String()
		case "ext_port":
			s[3] = value.String()
		case "int_port":
			s[4] = value.String()
		case "int_ip":
			s[5] = value.String()
		case "name":
			s[6] = value.String()
		case "wan":
			s[7] = value.String()
		}

	}
	*ss = append(*ss, strings.Join(s, "<"))

	return func(ss []string) string {
		return strings.Join(ss, ">")
	}
}

func unmarshal(s string, v any, f func(s string, numField int, t reflect.Type, v reflect.Value, value *reflect.Value)) {

	t := reflect.TypeOf(v)
	value := reflect.ValueOf(v)

	if t.Kind() != reflect.Ptr {
		return
	}
	t = t.Elem()
	value = value.Elem()

	if t.Kind() == reflect.Slice {

		v := reflect.New(t.Elem()).Elem()
		n := value.Type().Elem().NumField()
		t = t.Elem()

		f(s, n, t, v, &value)
		return
	}

	f(s, t.NumField(), t, value, nil)
}

func unmkSnat(s string, numField int, t reflect.Type, v reflect.Value, value *reflect.Value) {

	for _, record := range strings.Split(s, "<") {
		if len(record) == 0 {
			continue
		}

		field := strings.Split(record, "|")

		if len(field) != 12 {
			continue
		}

		for i := range numField {
			f := v.Field(i)
			tag := t.Field(i).Tag.Get("weyos")
			if tag == "" || tag == "-" {
				continue
			}

			switch tag {
			case "name":
				f.SetString(field[0])
			case "enabled":
				f.SetString(field[1])
			case "lanip_range":
				f.SetString(field[2])
			case "time_range":
				f.SetString(field[3])
			case "log_enabled":
				f.SetString(field[4])
			case "priority":
				f.SetString(field[5])
			case "wan":
				f.SetString(field[6])
			case "thd_type":
				f.SetString(field[7])
			case "opt":
				f.SetString(field[8])
			case "l7proto":
				f.SetString(field[9])
			case "proto":
				f.SetString(field[10])
			case "no_change":
				f.SetString(field[11])
			}
		}
		if value != nil {
			value.Set(reflect.Append(*value, v))
		}

	}

}

func unmkDnat(s string, numField int, t reflect.Type, v reflect.Value, value *reflect.Value) {

	for _, record := range strings.Split(s, ">") {

		field := strings.Split(record, "<")
		if len(field) != 8 {
			continue
		}

		for i := range numField {
			f := v.Field(i)
			tag := t.Field(i).Tag.Get("weyos")
			if tag == "" || tag == "-" {
				continue
			}

			switch tag {

			case "enabled":
				f.SetString(field[0])
			case "proto":
				f.SetString(field[1])
			case "src_ip":
				f.SetString(field[2])
			case "ext_port":
				f.SetString(field[3])
			case "int_port":
				f.SetString(field[4])
			case "int_ip":
				f.SetString(field[5])
			case "name":
				f.SetString(field[6])
			case "wan":
				f.SetString(field[7])
			}
		}

		if value != nil {
			value.Set(reflect.Append(*value, v))
		}

	}

}
