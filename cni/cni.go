package cni

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

type nic struct {
	masterNic netlink.Link
}

type vlan struct {
	masterNic netlink.Link
}

type ipconfig struct {
	nic netlink.Link
}

func (i *ipconfig) SetIp(ip string, gateway string) {

	_, ipnet, err := net.ParseCIDR(ip)
	if err != nil {
		return
	}

	addr := &netlink.Addr{IPNet: ipnet}
	err = netlink.AddrReplace(i.nic, addr)
	if err != nil {
		fmt.Println("xxx", err)
		return
	}

	route := netlink.Route{
		Dst: &net.IPNet{IP: net.IPv4(0, 0, 0, 0), Mask: net.CIDRMask(0, 32)},
		Gw:  net.ParseIP(gateway),
	}

	err = netlink.RouteAddEcmp(&route)
	if err != nil {
		fmt.Println("aaa", err)
		return
	}
}

// pid process id
func (v *vlan) MacVlan(pid ...int) *ipconfig {
	macvlan(v.masterNic)
	return &ipconfig{}
}

func Nic(networkInterface ...string) *nic {
	nicName := "eth0"
	if len(networkInterface) == 1 {
		nicName = networkInterface[0]
	}
	n, err := netlink.LinkByName(nicName)
	if err != nil {
		fmt.Println(nicName, err)
		return nil
	}
	return &nic{masterNic: n}
}

// vlanId vlan id.
// nicName if spect. default nic.vlanid like eth0.10
func (n *nic) Vlan(vlanId int) *vlan {
	nicName := fmt.Sprintf("%s.%d", n.masterNic.Attrs().Name, vlanId)

	//vlan, err := netlink.LinkByName(nicName)
	//if err != nil {
	//	fmt.Println(err)
	//	return nil
	//}

	nic := &netlink.Vlan{LinkAttrs: netlink.LinkAttrs{Name: nicName, ParentIndex: n.masterNic.Attrs().Index}, VlanId: vlanId, VlanProtocol: netlink.VLAN_PROTOCOL_8021Q}
	netlink.LinkAdd(nic)
	netlink.LinkSetUp(nic)

	return &vlan{masterNic: nic}
}

// pid process id
func (n *nic) MacVlan(pid ...int) *ipconfig {

	macvlan(n.masterNic)

	return &ipconfig{}
}

func macvlan(masterNic netlink.Link) {
	nic := &netlink.Macvlan{LinkAttrs: netlink.LinkAttrs{Name: "ss", ParentIndex: masterNic.Attrs().Index}}

	err := netlink.LinkAdd(nic)
	netlink.LinkSetUp(nic)
}
