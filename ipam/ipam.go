package ipam

import (
	"fmt"
	"net"
	"strings"

	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend/allocator"
)

type ipam struct {
	//allocator    *allocator.IPAllocator
	//requestedIPs map[string]net.IP
	rangeset allocator.RangeSet
	s        *Store
}

type ipRange allocator.Range
type result *current.IPConfig

// subnet subnet. like 192.168.1.0/24
//
// gateway gateway. like 192.168.1.254
//
// [option] start_end start end. like 192.168.1.1-192.168.1.100
func RangeConfig(subnet string, gateway string, start_end ...string) *ipRange {

	sb, err := types.ParseCIDR(subnet)
	if err != nil {
		fmt.Println("start_end format error.")
		return nil
	}
	err = canonicalizeIP(&sb.IP)
	if err != nil {
		fmt.Println("start_end format error.")
		return nil
	}

	var s net.IP
	var e net.IP

	if len(start_end) == 1 {
		se := strings.Split(start_end[0], "-")
		if len(se) == 2 {
			s = net.ParseIP(se[0])
			e = net.ParseIP(se[1])
		}
	}

	g := net.ParseIP(gateway)
	rset := &ipRange{
		RangeStart: s,
		RangeEnd:   e,
		Subnet:     types.IPNet(*sb),
		Gateway:    g,
	}

	return rset
}
func canonicalizeIP(ip *net.IP) error {
	if ip.To4() != nil {
		*ip = ip.To4()
		return nil
	} else if ip.To16() != nil {
		*ip = ip.To16()
		return nil
	}
	return fmt.Errorf("IP %s not v4 nor v6", *ip)
}

// etcdEndpoint edtcd endpoint, comma separated multiple. like 192.168.1.10:2379,192.168.1.11:2379,192.168.1.13:2379
//
// ipRange usr RangeConfig()
func Etcd(etcdEndpoint string, ipRange *ipRange) *ipam {
	if ipRange == nil {
		panic("ipRange config error")
	}
	s, err := newEtcd(1, strings.Split(etcdEndpoint, ","))
	if err != nil {
		panic(err)
	}
	return &ipam{s: s, rangeset: allocator.RangeSet{allocator.Range(*ipRange)}}
}

func (i *ipam) allocIP(id, ifName string, ip ...string) (result, error) {

	requestedIPs := map[string]net.IP{}
	allocs := []*allocator.IPAllocator{}
	i.rangeset.Canonicalize()

	allocator := allocator.NewIPAllocator(&i.rangeset, i.s, 0)

	if len(ip) == 1 {
		requestedIPs[ip[0]] = net.ParseIP(ip[0])
	}

	var requestedIP net.IP

	for k, ip := range requestedIPs {

		if i.rangeset.Contains(ip) {
			requestedIP = ip
			delete(requestedIPs, k)
			break
		}
	}

	ipConf, err := allocator.Get(id, ifName, requestedIP)

	if err != nil {

		if !strings.Contains(err.Error(), fmt.Sprintf("has been allocated to %s, duplicate allocation is not allowed", id)) {
			for _, alloc := range allocs {
				_ = alloc.Release(id, ifName)
			}

			return nil, fmt.Errorf("failed to allocate for range %d: %v", 0, err)
		}

		allocatedIPs := i.s.GetByID(id, ifName)
		for _, allocatedIP := range allocatedIPs {
			if r, err := i.rangeset.RangeFor(allocatedIP); err == nil {
				ipConf = &current.IPConfig{
					Address: net.IPNet{IP: allocatedIP, Mask: r.Subnet.Mask},
					Gateway: r.Gateway,
				}
			}
		}

	}

	allocs = append(allocs, allocator)

	if len(requestedIPs) != 0 {
		for _, alloc := range allocs {

			_ = alloc.Release(id, ifName)
		}
	}

	return result(ipConf), nil
}
func (i *ipam) Dhcp(id, ifName string) result {
	r, err := i.allocIP(id, ifName)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return r
}
func (i *ipam) Static(id, ifName, ip string) result {

	r, err := i.allocIP(id, ifName, ip)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return r

}
