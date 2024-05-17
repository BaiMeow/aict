package netcfg

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"net"
)

func ApplyNet(iface string, ip *net.IPNet, routes []*net.IPNet) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return err
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("set link up: %v", err)
	}

	if len(routes) == 1 {
		err := netlink.AddrAdd(link, &netlink.Addr{
			IPNet: ip,
			Peer:  routes[0],
		})
		if err != nil {
			return fmt.Errorf("apply ip peer: %v", err)
		}
	} else {
		err := netlink.AddrAdd(link, &netlink.Addr{
			IPNet: ip,
		})
		if err != nil {
			return fmt.Errorf("apply ip: %v", err)
		}
		for _, route := range routes {
			err := netlink.RouteAdd(&netlink.Route{
				LinkIndex: link.Attrs().Index,
				Dst:       route,
			})
			if err != nil {
				return fmt.Errorf("add route: %v", err)
			}
		}
	}
	return nil
}
