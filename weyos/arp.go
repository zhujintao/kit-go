package weyos

import "reflect"

type Arp struct {
	Name   string `json:"n"`
	Iface  string `json:"d"`
	Mac    string `json:"m"`
	Ip     string
	IpUint string `json:"i"`
	NoNum  int    `json:"no"`
	Ttype  string `json:"t"` // 类型 1 静态 0 动态 2唯一
	Status string `json:"s"` //状态 0 擦测 1 正常
}

type arp struct {
	entrys []Arp
}

func (a *arp) Arpfrom(s string) {

}

func (s *Arp) Map(gb2312 bool) map[string]string {
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
