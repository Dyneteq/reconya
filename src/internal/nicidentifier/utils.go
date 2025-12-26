package nicidentifier

import (
	"fmt"
	"net"
	"reconya/models"
	"strings"
)

// extractCIDR extracts the CIDR from an IPv4 address by looking up the actual network interface
func extractCIDR(ipv4 string) string {
	// Try to find the actual network interface and subnet mask
	interfaces, err := net.Interfaces()
	if err != nil {
		// Fallback to /24 assumption
		parts := strings.Split(ipv4, ".")
		if len(parts) < 3 {
			return ""
		}
		return fmt.Sprintf("%s.%s.%s.0/24", parts[0], parts[1], parts[2])
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				if ipNet.IP.To4() != nil && ipNet.IP.String() == ipv4 {
					// Found the matching interface, return the actual CIDR
					return ipNet.String()
				}
			}
		}
	}

	// Fallback to /24 assumption if not found
	parts := strings.Split(ipv4, ".")
	if len(parts) < 3 {
		return ""
	}
	return fmt.Sprintf("%s.%s.%s.0/24", parts[0], parts[1], parts[2])
}

// extractIpRange generates IP range from a given NIC based on actual subnet
func extractIpRange(nic models.NIC) []string {
	var addresses []string
	
	// Try to get the actual CIDR for this IP
	cidr := extractCIDR(nic.IPv4)
	if cidr == "" {
		// Fallback to /24 assumption
		parts := strings.Split(nic.IPv4, ".")
		base := strings.Join(parts[:3], ".")
		for i := 1; i <= 254; i++ {
			addresses = append(addresses, fmt.Sprintf("%s.%d", base, i))
		}
		return addresses
	}

	// Parse the CIDR to generate proper range
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		// Fallback to /24
		parts := strings.Split(nic.IPv4, ".")
		base := strings.Join(parts[:3], ".")
		for i := 1; i <= 254; i++ {
			addresses = append(addresses, fmt.Sprintf("%s.%d", base, i))
		}
		return addresses
	}

	// Generate all IPs in the subnet
	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); incrementIP(ip) {
		// Skip network and broadcast addresses
		if !ip.Equal(ipNet.IP) && !isBroadcast(ip, ipNet) {
			addresses = append(addresses, ip.String())
		}
		// Limit to reasonable number for scanning
		if len(addresses) >= 1000 {
			break
		}
	}

	return addresses
}

// incrementIP increments an IP address
func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// isBroadcast checks if an IP is the broadcast address for the network
func isBroadcast(ip net.IP, ipNet *net.IPNet) bool {
	broadcast := make(net.IP, len(ip))
	copy(broadcast, ipNet.IP)
	for i := range broadcast {
		broadcast[i] |= ^ipNet.Mask[i]
	}
	return ip.Equal(broadcast)
}
