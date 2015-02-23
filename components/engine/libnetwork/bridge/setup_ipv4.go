package bridge

import (
	"fmt"
	"net"

	log "github.com/Sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

var bridgeNetworks []*net.IPNet

func init() {
	// Here we don't follow the convention of using the 1st IP of the range for the gateway.
	// This is to use the same gateway IPs as the /24 ranges, which predate the /16 ranges.
	// In theory this shouldn't matter - in practice there's bound to be a few scripts relying
	// on the internal addressing or other stupid things like that.
	// They shouldn't, but hey, let's not break them unless we really have to.
	for _, addr := range []string{
		"172.17.42.1/16", // Don't use 172.16.0.0/16, it conflicts with EC2 DNS 172.16.0.23
		"10.0.42.1/16",   // Don't even try using the entire /8, that's too intrusive
		"10.1.42.1/16",
		"10.42.42.1/16",
		"172.16.42.1/24",
		"172.16.43.1/24",
		"172.16.44.1/24",
		"10.0.42.1/24",
		"10.0.43.1/24",
		"192.168.42.1/24",
		"192.168.43.1/24",
		"192.168.44.1/24",
	} {
		ip, net, err := net.ParseCIDR(addr)
		if err != nil {
			log.Errorf("Failed to parse address %s", addr)
			continue
		}
		net.IP = ip
		bridgeNetworks = append(bridgeNetworks, net)
	}
}

func SetupBridgeIPv4(i *Interface) error {
	bridgeIPv4, err := electBridgeIPv4(i.Config)
	if err != nil {
		return err
	}

	log.Debugf("Creating bridge interface %q with network %s", i.Config.BridgeName, bridgeIPv4)
	return netlink.AddrAdd(i.Link, &netlink.Addr{bridgeIPv4, ""})
}

func electBridgeIPv4(config *Configuration) (*net.IPNet, error) {
	// Use the requested IPv4 IP and mark when available.
	if config.AddressIPv4 != nil {
		return config.AddressIPv4, nil
	}

	// Try to automatically elect appropriate brige IPv4 settings.
	for _, n := range bridgeNetworks {
		// TODO CheckNameserverOverlaps
		// TODO CheckRouteOverlaps
		return n, nil
	}

	return nil, fmt.Errorf("Couldn't find an address range for interface %q", config.BridgeName)
}
