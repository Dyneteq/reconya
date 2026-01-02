package scanner

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ARPScanner performs ARP-based network discovery
type ARPScanner struct {
	timeout             time.Duration
	concurrent          int
	rawSocketsAvailable bool
}

// ARPScanResult represents a single ARP scan result
type ARPScanResult struct {
	IP    string
	MAC   string
	RTT   time.Duration
	Error error
}

// NewARPScanner creates a new ARP scanner
func NewARPScanner() *ARPScanner {
	s := &ARPScanner{
		timeout:    time.Second * 2,
		concurrent: 100,
	}
	s.rawSocketsAvailable = s.checkRawSocketCapability()
	if s.rawSocketsAvailable {
		log.Println("ARP Scanner: Raw socket access available")
	} else {
		log.Println("ARP Scanner: Raw sockets unavailable, using fallback method")
	}
	return s
}

// checkRawSocketCapability tests if raw sockets are available
func (s *ARPScanner) checkRawSocketCapability() bool {
	switch runtime.GOOS {
	case "linux":
		// On Linux, try to open a raw socket briefly
		// This requires CAP_NET_RAW or root
		return s.testRawSocketAccessLinux()
	case "darwin":
		// On macOS, check if we can access BPF devices
		return s.testRawSocketAccessDarwin()
	default:
		return false
	}
}

func (s *ARPScanner) testRawSocketAccessLinux() bool {
	// Try to create an AF_PACKET socket
	// This is a lightweight test that doesn't require full ARP implementation
	fd, err := createRawSocketLinux()
	if err != nil {
		return false
	}
	closeRawSocket(fd)
	return true
}

func (s *ARPScanner) testRawSocketAccessDarwin() bool {
	// On macOS, check if /dev/bpf* is accessible
	// For now, we'll rely on the fallback method which works without privileges
	return false
}

// ScanNetwork performs ARP scanning on the given network
func (s *ARPScanner) ScanNetwork(cidr string) ([]ARPScanResult, error) {
	log.Printf("ARP Scanner: Starting scan on %s", cidr)

	if s.rawSocketsAvailable {
		results, err := s.scanWithRawSockets(cidr)
		if err == nil && len(results) > 0 {
			return results, nil
		}
		log.Printf("ARP Scanner: Raw socket scan failed or empty, trying fallback: %v", err)
	}

	// Fallback: Use UDP trigger + ARP table read
	return s.scanWithFallback(cidr)
}

// scanWithRawSockets uses raw sockets for active ARP requests
func (s *ARPScanner) scanWithRawSockets(cidr string) ([]ARPScanResult, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	// Get local interface for this network
	iface, localIP, localMAC, err := s.getInterfaceForNetwork(ipNet)
	if err != nil {
		return nil, fmt.Errorf("no interface for network: %w", err)
	}

	switch runtime.GOOS {
	case "linux":
		return s.arpScanLinux(iface, localIP, localMAC, ipNet)
	case "darwin":
		return s.arpScanDarwin(iface, localIP, localMAC, ipNet)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// arpScanLinux uses AF_PACKET raw sockets on Linux
func (s *ARPScanner) arpScanLinux(iface *net.Interface, localIP net.IP, localMAC net.HardwareAddr, network *net.IPNet) ([]ARPScanResult, error) {
	ips := s.generateIPList(network)
	results := make([]ARPScanResult, 0)

	// Create raw socket
	fd, err := createRawSocketLinux()
	if err != nil {
		return nil, fmt.Errorf("failed to create raw socket: %w", err)
	}
	defer closeRawSocket(fd)

	// Bind to interface
	if err := bindToInterface(fd, iface.Index); err != nil {
		return nil, fmt.Errorf("failed to bind to interface: %w", err)
	}

	// Send ARP requests for all IPs
	for _, ip := range ips {
		targetIP := net.ParseIP(ip).To4()
		if targetIP == nil {
			continue
		}
		sendARPRequest(fd, iface.HardwareAddr, localIP.To4(), targetIP)
	}

	// Wait for responses
	time.Sleep(500 * time.Millisecond)

	// Read ARP table for responses
	arpTable := s.readARPTable()
	for ip, mac := range arpTable {
		parsedIP := net.ParseIP(ip)
		if parsedIP != nil && network.Contains(parsedIP) {
			results = append(results, ARPScanResult{
				IP:  ip,
				MAC: mac,
			})
		}
	}

	return results, nil
}

// arpScanDarwin uses BPF on macOS (placeholder - uses fallback)
func (s *ARPScanner) arpScanDarwin(iface *net.Interface, localIP net.IP, localMAC net.HardwareAddr, network *net.IPNet) ([]ARPScanResult, error) {
	// macOS BPF implementation is complex, use fallback
	return nil, fmt.Errorf("raw socket not implemented for darwin, using fallback")
}

// scanWithFallback uses command-line tools when raw sockets unavailable
func (s *ARPScanner) scanWithFallback(cidr string) ([]ARPScanResult, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	ips := s.generateIPList(ipNet)
	log.Printf("ARP Scanner: Triggering ARP for %d IPs", len(ips))

	// Send UDP packets in parallel to trigger ARP resolution
	var wg sync.WaitGroup
	sem := make(chan struct{}, s.concurrent)

	for _, ip := range ips {
		wg.Add(1)
		go func(ipAddr string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			s.triggerARP(ipAddr)
		}(ip)
	}

	wg.Wait()

	// Wait for ARP table to populate - give devices time to respond
	time.Sleep(500 * time.Millisecond)

	// Read entire ARP table
	arpTable := s.readARPTable()
	log.Printf("ARP Scanner: Read %d entries from ARP table", len(arpTable))

	// Filter to our network
	results := make([]ARPScanResult, 0)
	for ip, mac := range arpTable {
		parsedIP := net.ParseIP(ip)
		if parsedIP != nil && ipNet.Contains(parsedIP) {
			results = append(results, ARPScanResult{
				IP:  ip,
				MAC: mac,
			})
		}
	}

	log.Printf("ARP Scanner: Found %d devices in network %s", len(results), cidr)
	return results, nil
}

// triggerARP sends UDP packets to multiple ports to trigger ARP resolution
func (s *ARPScanner) triggerARP(ip string) {
	// Send UDP packets to multiple ports - this increases the chance of ARP resolution
	// We don't care about responses, just need the kernel to resolve the MAC address
	ports := []string{":7", ":53", ":67", ":137", ":33434"}
	for _, port := range ports {
		conn, err := net.DialTimeout("udp", ip+port, 20*time.Millisecond)
		if err == nil {
			conn.Write([]byte{0})
			conn.Close()
		}
	}
}

// readARPTable reads the system ARP table
func (s *ARPScanner) readARPTable() map[string]string {
	switch runtime.GOOS {
	case "linux":
		return s.readARPTableLinux()
	case "darwin":
		return s.readARPTableDarwin()
	default:
		return make(map[string]string)
	}
}

// readARPTableLinux reads /proc/net/arp on Linux
func (s *ARPScanner) readARPTableLinux() map[string]string {
	result := make(map[string]string)

	content, err := exec.Command("cat", "/proc/net/arp").Output()
	if err != nil {
		return result
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines[1:] { // Skip header
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			ip := fields[0]
			mac := fields[3]
			// Skip incomplete entries
			if mac != "00:00:00:00:00:00" && mac != "<incomplete>" {
				result[ip] = strings.ToUpper(mac)
			}
		}
	}

	return result
}

// readARPTableDarwin parses output of 'arp -an' on macOS
func (s *ARPScanner) readARPTableDarwin() map[string]string {
	result := make(map[string]string)

	output, err := exec.Command("arp", "-an").Output()
	if err != nil {
		return result
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// Format: "? (192.168.1.1) at aa:bb:cc:dd:ee:ff on en0 ifscope [ethernet]"
		// or: "? (192.168.1.2) at (incomplete) on en0 ifscope [ethernet]"
		if !strings.Contains(line, " at ") {
			continue
		}

		// Extract IP from parentheses
		start := strings.Index(line, "(")
		end := strings.Index(line, ")")
		if start == -1 || end == -1 || start >= end {
			continue
		}
		ip := line[start+1 : end]

		// Extract MAC after "at"
		atIndex := strings.Index(line, " at ")
		if atIndex == -1 {
			continue
		}
		rest := line[atIndex+4:]
		fields := strings.Fields(rest)
		if len(fields) < 1 {
			continue
		}
		mac := fields[0]

		// Skip incomplete entries
		if mac == "(incomplete)" || len(mac) != 17 {
			continue
		}

		result[ip] = strings.ToUpper(mac)
	}

	return result
}

// getInterfaceForNetwork finds the network interface for a given subnet
func (s *ARPScanner) getInterfaceForNetwork(network *net.IPNet) (*net.Interface, net.IP, net.HardwareAddr, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, nil, nil, err
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip := ipNet.IP.To4()
			if ip == nil {
				continue
			}

			if network.Contains(ip) {
				return &iface, ip, iface.HardwareAddr, nil
			}
		}
	}

	return nil, nil, nil, fmt.Errorf("no interface found for network")
}

// generateIPList generates all IP addresses in a CIDR range
func (s *ARPScanner) generateIPList(ipNet *net.IPNet) []string {
	var ips []string

	ip := ipNet.IP.To4()
	if ip == nil {
		return ips
	}

	mask := ipNet.Mask
	ones, bits := mask.Size()
	hostBits := bits - ones
	totalHosts := (1 << hostBits) - 2

	if totalHosts <= 0 {
		return ips
	}

	network := ip.Mask(mask)
	networkInt := uint32(network[0])<<24 | uint32(network[1])<<16 | uint32(network[2])<<8 | uint32(network[3])

	for i := 1; i <= totalHosts; i++ {
		hostIP := networkInt + uint32(i)
		ips = append(ips, fmt.Sprintf("%d.%d.%d.%d",
			(hostIP>>24)&0xFF,
			(hostIP>>16)&0xFF,
			(hostIP>>8)&0xFF,
			hostIP&0xFF))
	}

	return ips
}
