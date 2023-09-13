package cni

import (
	"fmt"
	"net"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/utils/sysctl"
	"github.com/vishvananda/netlink"
)

type nic struct {
	masterNic netlink.Link
}
type macvlan struct {
	vlanNic netlink.Link
}

type ipaddr struct {
	netNsPath string
}

// Nic("eth0").Vlan(10).MacVlan(pid)

func Nic(nicName string) *nic {

	n, err := netlink.LinkByName(nicName)
	if err != nil {
		fmt.Println(nicName, err)
		return nil
	}
	return &nic{masterNic: n}
}

func (n *nic) DelVlan(vlanId int, nicName ...string) {

	if n == nil {
		return
	}

	if len(nicName) != 1 {
		nicName = []string{fmt.Sprintf("%s.%d", n.masterNic.Attrs().Name, vlanId)}
	}

	vlinic, err := netlink.LinkByName(nicName[0])
	if err != nil {
		fmt.Println(nicName[0], err)
		return
	}

	err = netlink.LinkDel(vlinic)
	if err != nil {
		fmt.Println(vlinic.Attrs().Name, err)
		return
	}
	fmt.Println(vlinic.Attrs().Name, "delete ok")

}

func (n *nic) Vlan(vlanId int, nicName ...string) *macvlan {

	if n == nil {
		return nil
	}

	if len(nicName) != 1 {
		nicName = []string{fmt.Sprintf("%s.%d", n.masterNic.Attrs().Name, vlanId)}
	}

	vlinic, err := netlink.LinkByName(nicName[0])
	if err != nil {
		linknic := &netlink.Vlan{LinkAttrs: netlink.LinkAttrs{Name: nicName[0], ParentIndex: n.masterNic.Attrs().Index}, VlanId: vlanId, VlanProtocol: netlink.VLAN_PROTOCOL_8021Q}
		err := netlink.LinkAdd(linknic)

		if err != nil {
			return nil
		}
		vlinic, _ := netlink.LinkByName(nicName[0])
		netlink.LinkSetUp(vlinic)
		fmt.Println(nicName[0], "create success")
		return &macvlan{vlanNic: vlinic}
	}
	netlink.LinkSetUp(vlinic)
	fmt.Println(nicName[0], "already exists")
	return &macvlan{vlanNic: vlinic}

}

// pid process id
func (n *macvlan) MacVlan(pid int) *ipaddr {

	if n == nil {
		return nil
	}

	netNsPath := fmt.Sprintf("/proc/%d/ns/net", pid)
	netns, err := ns.GetNS(netNsPath)
	if err != nil {
		return nil
	}

	defer netns.Close()

	ifName := "eth0"
	tmpName, _ := ip.RandomVethName()
	linkAttrs := netlink.LinkAttrs{Name: tmpName,
		ParentIndex: n.vlanNic.Attrs().Index,
		Namespace:   netlink.NsFd(int(netns.Fd())),
	}
	mvnic := &netlink.Macvlan{LinkAttrs: linkAttrs}
	err = netlink.LinkAdd(mvnic)
	if err != nil {
		fmt.Println("linkadd mvnic", err)
		return nil
	}

	err = netns.Do(func(_ ns.NetNS) error {

		//nic, err := netlink.LinkByName(ifName)

		err = ip.RenameLink(tmpName, ifName)
		if err != nil {
			netlink.LinkDel(mvnic)
			return err
		}
		nic, _ := netlink.LinkByName(ifName)
		err = netlink.LinkSetUp(nic)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil
	}
	sysctl.Sysctl(fmt.Sprintf("net/ipv4/conf/%s/arp_notify"), "1")
	return &ipaddr{netNsPath: netNsPath}

}

func (n *ipaddr) SetIpAddr(ipaddr, gw string) {

	if n == nil {
		return
	}
	netns, err := ns.GetNS(n.netNsPath)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer netns.Close()

	ip, ipnet, err := net.ParseCIDR(ipaddr)
	if err != nil {
		fmt.Println(err)
		return
	}

	netip := &net.IPNet{
		IP:   ip,
		Mask: ipnet.Mask,
	}
	ifName := "eth0"

	netns.Do(func(_ ns.NetNS) error {

		link, err := netlink.LinkByName(ifName)

		if err != nil {
			fmt.Println(err)
			return err
		}

		addr := &netlink.Addr{IPNet: netip}
		err = netlink.AddrReplace(link, addr)
		if err != nil {
			fmt.Println("xxx", err)
			return err
		}

		route := netlink.Route{
			Dst: &net.IPNet{IP: net.IPv4(0, 0, 0, 0), Mask: net.CIDRMask(0, 32)},
			Gw:  net.ParseIP(gw),
		}

		err = netlink.RouteAddEcmp(&route)
		if err != nil {
			fmt.Println("aaa", err)
			return err
		}

		return nil
	})

}

func CreateVlan(masterNicName string, vlanId int, nicName ...string) *nic {

	if len(nicName) != 1 {
		nicName = []string{fmt.Sprintf("%s.%d", masterNicName, vlanId)}
	}

	_, err := netlink.LinkByName(nicName[1])
	if err == nil {
		return &nic{}
	}

	mNic, err := netlink.LinkByName(masterNicName)
	if err != nil {
		return nil
	}
	linknic := &netlink.Vlan{LinkAttrs: netlink.LinkAttrs{Name: nicName[1], ParentIndex: mNic.Attrs().Index}, VlanId: vlanId, VlanProtocol: netlink.VLAN_PROTOCOL_8021Q}
	netlink.LinkAdd(linknic)

	return &nic{}
}

func BindMacvlan(masterNic, pid string) {

	netNsPath := fmt.Sprintf("/proc/%d/ns/net", pid)

	netns, err := ns.GetNS(netNsPath)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer netns.Close()

}
