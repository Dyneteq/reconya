package scanner

import (
	"net"
	"sync"
)

// localSubnets caches the directly-attached IPv4 subnets of this host. Interface
// addresses change rarely and re-enumerating them for each of the 254 addresses
// in a /24 would be wasteful, so the list is resolved once per process.
var (
	localSubnetsOnce sync.Once
	localSubnetsVal  []*net.IPNet
)

func localSubnets() []*net.IPNet {
	localSubnetsOnce.Do(func() {
		localSubnetsVal = resolveLocalSubnets()
	})
	return localSubnetsVal
}

func resolveLocalSubnets() []*net.IPNet {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var subnets []*net.IPNet
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		// A point-to-point link (VPN/tunnel) has no broadcast domain, so ARP is
		// not available there even though the address may look "local".
		if iface.Flags&net.FlagPointToPoint != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}
			subnets = append(subnets, ipNet)
		}
	}
	return subnets
}

// isOnLink reports whether ip sits inside a subnet this host is directly
// attached to, i.e. whether it shares a broadcast domain with us.
//
// This matters because it decides how much a bare TCP handshake is worth. On
// link, a genuinely live host must answer ARP, so a TCP connect that succeeds
// without a MAC is a middlebox answering on the host's behalf rather than the
// host itself. Off link (routed or VPN) ARP is unavailable, so TCP has to stay
// sufficient on its own.
func isOnLink(ip string) bool {
	return isOnLinkWithin(ip, localSubnets())
}

// isOnLinkWithin is the testable core of isOnLink.
func isOnLinkWithin(ip string, subnets []*net.IPNet) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	parsed = parsed.To4()
	if parsed == nil {
		return false
	}

	for _, subnet := range subnets {
		if subnet.Contains(parsed) {
			return true
		}
	}
	return false
}
