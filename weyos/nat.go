package weyos

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type nat struct {
	dNats []DNat
	sNats []SNat
}

type Entry interface {
}

// entry use DNat and SNat struct
func (n *nat) AddEntry(e Entry) {

	if e, ok := e.(DNat); ok {
		n.dNats = append(n.dNats, e)
	}
	if e, ok := e.(SNat); ok {
		n.sNats = append(n.sNats, e)
	}

}

func (n *nat) FindSNat(name string) *SNat {
	for i, e := range n.sNats {

		if e.Name == name {
			return &n.sNats[i]
		}
	}
	return nil
}

func (n *nat) FindDNat(name string) *DNat {

	i := slices.IndexFunc(n.dNats, func(e DNat) bool {
		return e.Name == name
	})
	if i == -1 {
		return nil
	}

	return &n.dNats[i]
}
func (n *nat) DelEntry(e Entry) bool {

	var yes bool

	if e, ok := e.(DNat); ok {
		n.dNats = slices.DeleteFunc(n.dNats, func(ee DNat) bool {
			if ee.Name == e.Name {
				yes = true
				return true
			}

			if ee.Status == e.Status {
				yes = true
				return true
			}

			return false

		})

	}
	if e, ok := e.(SNat); ok {
		n.sNats = slices.DeleteFunc(n.sNats, func(ee SNat) bool {
			if ee.Name == e.Name {
				yes = true
				return true
			}
			if ee.Status == e.Status {
				yes = true
				return true
			}
			return false
		})
	}

	return yes

}

func (n *nat) DNatString(gb2312 bool) string {
	s := MarshalDNAT(n.dNats)
	if gb2312 {
		s = StringGB2312(s)
	}
	return s
}

func (n *nat) GetEntrys(v any) {

	if e, ok := v.(*[]DNat); ok {
		*e = n.dNats
	}
	if e, ok := v.(*[]SNat); ok {
		*e = n.sNats
	}
}

func NewNatRecod() *nat {
	return &nat{}
}

func (n *nat) DNatfrom(s string) {
	UnmarshalDNAT(s, &n.dNats)
}
func (n *nat) SNatfrom(s string) {
	UnmarshalSNAT(s, &n.sNats)
}

var snatTagField map[string]string = map[string]string{
	"name":        "name",
	"enabled":     "en",
	"src":         "ips",
	"time_range":  "time",
	"log_enabled": "log",
	"priority":    "rpri",
	"out_if":      "wans",
	"thd":         "thd_type",
	"service":     "shibie",
	"proto":       "ipport",
	"failover":    "no_change",
}
var snatFieldPos map[string]int = map[string]int{
	"name":        0,
	"enabled":     1,
	"src":         2,
	"time_range":  3,
	"log_enabled": 4,
	"priority":    5,
	"out_if":      6,
	"thd":         7,
	"service":     9,
	"proto":       10,
	"failover":    11,
}

type SNat struct {
	Name         string `weyos:"name"`
	Status       string `weyos:"enabled"`
	Priority     string `weyos:"priority"`
	WanInterface string `weyos:"out_if"`
	Protocol     string `weyos:"proto"`
	LanIps       string `weyos:"src"`
	Schedule     string `weyos:"time_range"`
	Failover     string `weyos:"failover"`
	Logging      string `weyos:"log_enabled"`
	Application  string `weyos:"service"`
	Thdtype      string `weyos:"thd"`
}

var dnatFieldPos map[string]int = map[string]int{
	"enabled":  0,
	"proto":    1,
	"src":      2,
	"dst_port": 3,
	"to_port":  4,
	"to_addr":  5,
	"name":     6,
	"ext_if":   7,
}

type DNat struct {
	Name         string `weyos:"name"`
	Status       string `weyos:"enabled"`
	Protocol     string `weyos:"proto"`
	LanPort      string `weyos:"to_port"`
	LanIp        string `weyos:"to_addr"`
	WanIps       string `weyos:"src"`
	WanPort      string `weyos:"dst_port"`
	WanInterface string `weyos:"ext_if"`
}

func (s *SNat) Map(gb2312 bool) map[string]string {
	var m map[string]string = map[string]string{}
	v := reflect.ValueOf(s).Elem()
	t := v.Type()
	for i := range v.NumField() {
		tag := t.Field(i).Tag.Get("weyos")
		if tag == "" || tag == "-" {
			continue
		}
		value := v.Field(i)
		vv := value.String()
		if gb2312 {
			vv = StringGB2312(vv)
		}
		if name, ok := snatTagField[tag]; ok {
			m[name] = vv
		}

	}
	return m
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

func MarshalSNAT(v any) string {
	return marshal(v, mkSnat)
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
		if len(ss) == 0 {
			return ""
		}
		return call(ss)
	}

	call = f(value.NumField(), value, &ss)
	if len(ss) == 0 {
		return ""
	}
	return call(ss)

}

func mkSnat(numField int, v reflect.Value, ss *[]string) func(ss []string) string {
	var s []string = make([]string, 12)
	t := v.Type()
	for i := range numField {

		field := t.Field(i)
		tag := field.Tag.Get("weyos")
		if tag == "" || tag == "-" {
			continue
		}
		value := v.Field(i)

		name, ok := snatTagField[tag]
		if !ok {
			continue
		}
		pos, ok := snatFieldPos[tag]
		if !ok {
			continue
		}
		s[pos] = name + "=" + value.String()

	}
	*ss = append(*ss, strings.Join(s, "&"))

	return func(ss []string) string {
		return strings.Join(ss, " ")
	}
}

func mkDnat(numField int, v reflect.Value, ss *[]string) func(ss []string) string {
	var s []string = make([]string, 8)
	t := v.Type()
	for i := range numField {

		field := t.Field(i)
		tag := field.Tag.Get("weyos")
		if tag == "" || tag == "-" {
			continue
		}
		value := v.Field(i)

		pos, ok := dnatFieldPos[tag]
		if !ok {
			continue
		}
		s[pos] = value.String()

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

			if pos, ok := snatFieldPos[tag]; ok {
				if len(field) < pos {
					fmt.Println("pos error")
					return
				}
				f.SetString(field[pos])
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

			if pos, ok := dnatFieldPos[tag]; ok {
				if len(field) < pos {
					fmt.Println("pos error")
					return
				}
				f.SetString(field[pos])
			}

		}

		if value != nil {
			value.Set(reflect.Append(*value, v))
		}

	}

}

func StringGB2312(s string) string {

	gb2312 := simplifiedchinese.GBK.NewEncoder()
	sgb, _, err := transform.String(gb2312, s)
	if err != nil {
		fmt.Println("GK2312 error", err)
		return ""
	}
	return sgb
}
