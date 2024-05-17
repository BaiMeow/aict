package netcfg

import (
	"fmt"
	"golang.org/x/sys/windows"
	"log"
	"net"
	"os/exec"
	"path/filepath"
)

func ApplyNet(iface string, ip *net.IPNet, routes []*net.IPNet) error {
	systemDir, err := windows.GetSystemDirectory()
	if err != nil {
		return fmt.Errorf("get system directory: %v", err)
	}
	netsh := filepath.Join(systemDir, "netsh.exe")

	if output, err := exec.Command(netsh, "interface", "ip", "set", "address", iface, "source=static", fmt.Sprintf("address=%s", ip.String()), "mask=255.255.255.255").CombinedOutput(); err != nil {
		log.Printf("exec err: %s", output)
		return fmt.Errorf("apply ip: %v", err)
	}
	for _, route := range routes {
		if output, err := exec.Command(netsh, "interface", "ip", "add", "route", route.String(), iface).CombinedOutput(); err != nil {
			log.Printf("exec err: %s", output)
			return fmt.Errorf("add route: %v", err)
		}
	}
	return nil
}
