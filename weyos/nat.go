package weyos

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"
)

type WanInterface string

var (
	WanAll = WanInterface("")
	Wan1   = WanInterface("0")
	Wan2   = WanInterface("1")
	Wan3   = WanInterface("2")
	Wan4   = WanInterface("3")
)

type DnatPorto string

var (
	DnatPortoTCP    = DnatPorto("1")
	DnatPortoUDP    = DnatPorto("2")
	DnatPortoTCPUDP = DnatPorto("3")
)

type Proto string
type Ipaddr string
type Port string

func (i *Ipaddr) Ip(ip ...string) Ipaddr {
	s := strings.Join(ip, ",")
	return Ipaddr(s)

}
func (i *Ipaddr) IpRange(start, end string) Ipaddr {
	return Ipaddr("")
}

type port struct {
	Start int
	End   int
}

func NewPort(start, end int) port {
	return port{Start: start, End: end}
}

func parseProto(perfix string, src, dst port) Proto {
	var s strings.Builder
	s.WriteString(perfix)

	if src.End == 0 {
		src.End = src.Start
	}
	if dst.End == 0 {
		dst.End = dst.Start
	}

	if src.End == 0 || src.Start > src.End {
		src.End = src.Start
	}

	if dst.End == 0 || dst.Start > dst.End {
		dst.End = dst.Start
	}
	if src.End != 0 {
		s.WriteString(fmt.Sprintf(":%d-%d", src.Start, src.End))
	}

	if src.End == 0 {
		s.WriteString(":")
	}

	if dst.End != 0 {
		s.WriteString(fmt.Sprintf(":%d-%d", dst.Start, dst.End))
	}

	if dst.End == 0 {
		s.WriteString(":")
	}
	return Proto(s.String())
}

// arg1 scrPort, arg2 dstPort
func (p *Proto) Tcp(src, dst port) Proto {
	return parseProto("TCP", src, dst)

}

func (p *Proto) Udp(src, dst port) Proto {
	return parseProto("UDP", src, dst)
}

func (p *Proto) TcpUdp(src, dst port) Proto {
	return parseProto("TCP/UDP", src, dst)
}

func (p *Proto) Icmp(src, dst port) Proto {
	return parseProto("ICMP", src, dst)
}

type SnatProto string

func NewIpRange(start, end string) Ipaddr {

	sip, _ := netip.ParseAddr(start)
	eip, _ := netip.ParseAddr(end)
	i := sip.Compare(eip)
	if i == 1 || i == 0 {
		return Ipaddr("")
	}

	return Ipaddr(sip.String() + "-" + eip.String())

}
func NewIpaddr(sip []string, iprange ...Ipaddr) Ipaddr {
	ips := strings.Join(sip, ",")
	var s strings.Builder

	for i, p := range iprange {
		if p == Ipaddr("") {
			continue
		}
		if i != 0 {
			s.WriteString(",")
		}
		s.WriteString(string(p))
	}

	if s.Len() != 0 {
		return Ipaddr(ips + "," + s.String())
	}
	return Ipaddr(ips)

}

func NewWan(iface ...WanInterface) WanInterface {

	var s strings.Builder
	for i, p := range iface {
		if p == "" {
			continue
		}

		if i != 0 {
			s.WriteString(",")
		}

		s.WriteString(string("WAN" + p))
	}

	return WanInterface(s.String())

}

// []weyos.Proto{weyos.NewPort()}
func NewSnatProto(dstIps []string, protos []Proto) SnatProto {

	if len(dstIps) == 0 && len(protos) == 0 {
		return SnatProto("")
	}
	ips := strings.Join(dstIps, ",")

	var s strings.Builder
	for i, p := range protos {
		if i != 0 {
			s.WriteString(",")
		}
		s.WriteString(string(p))
	}
	s.WriteString(">")

	// base DNS
	s.WriteString(">0")

	//
	ss := SnatProto("0>" + ips + ">" + s.String())
	return ss
}

func NewDnatPort(p port) Port {

	if p.Start == p.End {
		return Port(fmt.Sprintf("%d", p.Start))
	}
	if p.Start > p.End {
		return ""
	}
	return Port(fmt.Sprintf("%d:%d", p.Start, p.End))
}

func NewIp(s string) Ipaddr {
	return Ipaddr(s)
}

type DNat struct {
	Name         string       `json:"name"`
	WanInterface WanInterface `json:"wans"`
	SrcIps       Ipaddr       `json:"src"`
	SrcPorts     Port         `json:"fport"`
	LanIp        Ipaddr       `json:"inip"`
	LanPorts     Port         `json:"inport"`
	Status       int          `json:"en"`
	Protocol     DnatPorto    `json:"prot"`
}
type SNat struct {
	Name         string       `json:"name"`
	Protocol     SnatProto    `json:"ipport"`
	LanIps       Ipaddr       `json:"ips"`
	LogEnabled   int          `json:"log"`
	Status       int          `json:"en"`
	Failovered   string       `json:"no_change"`
	Priority     int          `json:"rpri"`
	WanInterface WanInterface `json:"wans"`
	Schedule     string       `json:"time"` //"0,2-3;09:00:00-18:00:00"
	AppProtocol  string       `json:"shibie"`
	Thdtype      string       `json:"thd_type"`
	Thd          string       `json:"thd"`
}
type nat struct {
	dNats []DNat
	sNats []SNat
}

func (n *nat) FindSNat(id string) *SNat {

	idx := slices.IndexFunc(n.sNats, func(s SNat) bool {

		if s.Name == id {
			return true
		}
		return false
	})

	if idx == -1 {
		return nil
	}

	return &n.sNats[idx]

}
func (n *nat) FindDNat(id string) *DNat {

	idx := slices.IndexFunc(n.dNats, func(s DNat) bool {

		if s.Name == id {
			return true
		}
		return false
	})

	if idx == -1 {
		return nil
	}

	return &n.dNats[idx]

}
