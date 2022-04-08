package tun

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

	tun_util "github.com/hxdcloud/gost-x/internal/util/tun"
	"github.com/songgao/water"
)

func (l *tunListener) createTun() (ifce *water.Interface, ip net.IP, err error) {
	ip, _, err = net.ParseCIDR(l.md.config.Net)
	if err != nil {
		return
	}

	ifce, err = water.New(water.Config{
		DeviceType: water.TUN,
	})
	if err != nil {
		return
	}

	peer := l.md.config.Peer
	if peer == "" {
		peer = ip.String()
	}

	cmd := fmt.Sprintf("ifconfig %s inet %s %s mtu %d up",
		ifce.Name(), l.md.config.Net, l.md.config.Peer, l.md.config.MTU)
	l.logger.Debug(cmd)

	args := strings.Split(cmd, " ")
	if err = exec.Command(args[0], args[1:]...).Run(); err != nil {
		return
	}

	if err = l.addRoutes(ifce.Name(), l.md.config.Routes...); err != nil {
		return
	}

	return
}

func (l *tunListener) addRoutes(ifName string, routes ...tun_util.Route) error {
	for _, route := range routes {
		cmd := fmt.Sprintf("route add -net %s -interface %s", route.Net.String(), ifName)
		l.logger.Debug(cmd)
		args := strings.Split(cmd, " ")
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			return err
		}
	}
	return nil
}
