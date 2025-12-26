package scanner

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"reconya/models"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type NativeScanner struct {
	timeout                  time.Duration
	concurrent               int
	enableMACLookup          bool
	enableHostnameLookup     bool
	enableOnlineVendorLookup bool
}

type ScanResult struct {
	IP       string
	Online   bool
	RTT      time.Duration
	MAC      string
	Vendor   string
	Hostname string
	Error    error
}

func NewNativeScanner() *NativeScanner {
	return &NativeScanner{
		timeout:                  time.Second * 3,
		concurrent:               50, // Concurrent goroutines for scanning
		enableMACLookup:          true,
		enableHostnameLookup:     true,
		enableOnlineVendorLookup: true, // Allow online vendor lookups
	}
}

// SetOptions allows configuring scanner behavior
func (s *NativeScanner) SetOptions(timeout time.Duration, concurrent int, enableMAC, enableHostname, enableOnlineVendor bool) {
	s.timeout = timeout
	s.concurrent = concurrent
	s.enableMACLookup = enableMAC
	s.enableHostnameLookup = enableHostname
	s.enableOnlineVendorLookup = enableOnlineVendor
}

// ScanNetwork performs a ping sweep on the given CIDR network
func (s *NativeScanner) ScanNetwork(network string) ([]models.Device, error) {
	log.Printf("Starting native Go network scan on: %s", network)

	// Parse the network CIDR
	_, ipNet, err := net.ParseCIDR(network)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %v", err)
	}

	// Generate all IPs in the network
	ips := s.generateIPList(ipNet)
	log.Printf("Scanning %d IP addresses", len(ips))

	// Create channels for work distribution
	ipChan := make(chan string, len(ips))
	resultChan := make(chan ScanResult, len(ips))

	// Fill the work channel
	for _, ip := range ips {
		ipChan <- ip
	}
	close(ipChan)

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < s.concurrent; i++ {
		wg.Add(1)
		go s.worker(ipChan, resultChan, &wg)
	}

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var devices []models.Device
	for result := range resultChan {
		if result.Online {
			device := models.Device{
				IPv4:   result.IP,
				Status: models.DeviceStatusOnline,
			}

			// Add MAC address if available
			if result.MAC != "" {
				device.MAC = &result.MAC
			}

			// Add vendor if available
			if result.Vendor != "" {
				device.Vendor = &result.Vendor
			}

			// Add hostname if available
			if result.Hostname != "" {
				device.Hostname = &result.Hostname
			}

			devices = append(devices, device)
			log.Printf("Found online device: %s (RTT: %v)", result.IP, result.RTT)
		}
	}

	log.Printf("Native scan completed. Found %d online devices", len(devices))
	return devices, nil
}

// worker is a goroutine that processes IPs from the channel
func (s *NativeScanner) worker(ipChan <-chan string, resultChan chan<- ScanResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for ip := range ipChan {
		result := s.scanIP(ip)
		resultChan <- result
	}
}

// scanIP performs various checks on a single IP address
func (s *NativeScanner) scanIP(ip string) ScanResult {
	result := ScanResult{IP: ip}

	// Try multiple detection methods
	online, rtt := s.tryPing(ip)
	if !online {
		online, rtt = s.tryTCPConnect(ip)
	}

	result.Online = online
	result.RTT = rtt

	if online {
		// Try to get additional information based on configuration
		if s.enableMACLookup {
			result.MAC, result.Vendor = s.getMACInfo(ip)
		}
		if s.enableHostnameLookup {
			result.Hostname = s.getHostname(ip)
		}
	}

	return result
}

// tryPing attempts to ping an IP address using ICMP
func (s *NativeScanner) tryPing(ip string) (bool, time.Duration) {
	// Note: ICMP ping requires raw sockets on most systems (root privileges)
	// For a more portable solution, we might want to use TCP connect instead

	start := time.Now()

	// Try to resolve the address first
	addr, err := net.ResolveIPAddr("ip4", ip)
	if err != nil {
		return false, 0
	}

	// Create ICMP connection (requires privileges on most systems)
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		// Fallback to TCP connect if ICMP fails
		return s.tryTCPConnect(ip)
	}
	defer conn.Close()

	// Create ICMP message
	message := &icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   1,
			Seq:  1,
			Data: []byte("reconYa ping"),
		},
	}

	data, err := message.Marshal(nil)
	if err != nil {
		return false, 0
	}

	// Set timeout
	conn.SetDeadline(time.Now().Add(s.timeout))

	// Send ICMP packet
	_, err = conn.WriteTo(data, addr)
	if err != nil {
		return false, 0
	}

	// Read response
	reply := make([]byte, 1500)
	n, _, err := conn.ReadFrom(reply)
	if err != nil {
		return false, 0
	}

	rtt := time.Since(start)

	// Parse ICMP reply
	rm, err := icmp.ParseMessage(int(ipv4.ICMPTypeEchoReply), reply[:n])
	if err != nil {
		return false, 0
	}

	if rm.Type == ipv4.ICMPTypeEchoReply {
		return true, rtt
	}

	return false, 0
}

// tryTCPConnect attempts to connect to common ports to detect if host is alive
func (s *NativeScanner) tryTCPConnect(ip string) (bool, time.Duration) {
	commonPorts := []int{80, 443, 22, 21, 23, 25, 53, 135, 139, 445, 161, 162}

	start := time.Now()

	for _, port := range commonPorts {
		address := fmt.Sprintf("%s:%d", ip, port)
		conn, err := net.DialTimeout("tcp", address, time.Millisecond*500)
		if err == nil {
			conn.Close()
			return true, time.Since(start)
		}
	}

	return false, 0
}

// getMACInfo attempts to get MAC address and vendor information
func (s *NativeScanner) getMACInfo(ip string) (string, string) {
	// Try multiple approaches to get MAC information

	// Approach 1: ARP table lookup
	if mac, vendor := s.getARPInfo(ip); mac != "" {
		return mac, vendor
	}

	// Approach 2: Wake-on-LAN packet trigger + ARP lookup
	if mac, vendor := s.triggerARPAndLookup(ip); mac != "" {
		return mac, vendor
	}

	// Approach 3: Network interface scanning for local subnet
	if mac, vendor := s.scanNetworkInterface(ip); mac != "" {
		return mac, vendor
	}

	return "", ""
}

// getARPInfo looks up MAC address from ARP table (cross-platform)
func (s *NativeScanner) getARPInfo(ip string) (string, string) {
	var mac string

	switch runtime.GOOS {
	case "linux":
		mac = s.getARPLinux(ip)
	case "darwin":
		mac = s.getARPMacOS(ip)
	case "windows":
		mac = s.getARPWindows(ip)
	default:
		return "", ""
	}

	if mac != "" {
		vendor := s.lookupVendor(mac)
		return mac, vendor
	}

	return "", ""
}

// getARPLinux reads /proc/net/arp on Linux
func (s *NativeScanner) getARPLinux(ip string) string {
	content, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines[1:] { // Skip header
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[0] == ip {
			mac := fields[3]
			if mac != "00:00:00:00:00:00" && mac != "<incomplete>" {
				return strings.ToUpper(mac)
			}
		}
	}
	return ""
}

// getARPMacOS uses arp command on macOS
func (s *NativeScanner) getARPMacOS(ip string) string {
	cmd := exec.Command("arp", "-n", ip)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, ip) {
			// Parse line like: "192.168.1.1 (192.168.1.1) at aa:bb:cc:dd:ee:ff on en0 ifscope [ethernet]"
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "at" && i+1 < len(parts) {
					mac := parts[i+1]
					if len(mac) == 17 && strings.Count(mac, ":") == 5 {
						return strings.ToUpper(mac)
					}
				}
			}
		}
	}
	return ""
}

// getARPWindows uses arp command on Windows
func (s *NativeScanner) getARPWindows(ip string) string {
	cmd := exec.Command("arp", "-a", ip)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, ip) {
			// Parse line like: "  192.168.1.1           aa-bb-cc-dd-ee-ff     dynamic"
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				mac := parts[1]
				// Convert Windows format (aa-bb-cc-dd-ee-ff) to standard format
				mac = strings.ReplaceAll(mac, "-", ":")
				if len(mac) == 17 && strings.Count(mac, ":") == 5 {
					return strings.ToUpper(mac)
				}
			}
		}
	}
	return ""
}

// triggerARPAndLookup sends a packet to trigger ARP resolution
func (s *NativeScanner) triggerARPAndLookup(ip string) (string, string) {
	// Send a UDP packet to a common port to trigger ARP resolution
	conn, err := net.DialTimeout("udp", ip+":53", time.Millisecond*100)
	if err == nil {
		conn.Close()
		// Small delay for ARP table update
		time.Sleep(time.Millisecond * 10)
		// Now try ARP lookup again
		return s.getARPInfo(ip)
	}
	return "", ""
}

// scanNetworkInterface attempts to get MAC via network interface (local subnet only)
func (s *NativeScanner) scanNetworkInterface(ip string) (string, string) {
	// This would require raw socket access for ARP requests
	// For now, return empty - could be implemented with raw sockets
	return "", ""
}

// lookupVendor looks up vendor information from MAC address
func (s *NativeScanner) lookupVendor(mac string) string {
	if len(mac) < 8 {
		return ""
	}

	// Extract OUI (first 3 octets)
	oui := strings.ReplaceAll(mac[:8], ":", "")
	oui = strings.ToUpper(oui)

	// Built-in vendor database (most common vendors)
	vendors := map[string]string{
		"000040": "Applicon",
		"0000FF": "Camtec Electronics",
		"000020": "Dataindustrier Diab AB",
		"001B63": "Apple",
		"8C859":  "Apple",
		"F0189":  "Apple",
		"00226B": "Cisco Systems",
		"0007EB": "Cisco Systems",
		"5C5948": "Samsung Electronics",
		"002454": "Intel Corporate",
		"84FDD1": "Netgear",
		"001E58": "Netgear",
		"00095B": "Netgear",
		"3C37E6": "Intel Corporate",
		"7085C2": "Intel Corporate",
		"DC85DE": "Intel Corporate",
		"00D0C9": "Intel Corporate",
		"E45F01": "Intel Corporate",
		"38D547": "Apple",
		"A4C361": "Apple",
		"F02475": "Apple",
		"14109F": "Apple",
		"3451C9": "Apple",
		"BC52B7": "Apple",
		"E8802E": "Apple",
		"E06267": "Apple",
		"90B21F": "Apple",
		"F86214": "Apple",
		"68A86D": "Apple",
		"7C6DF8": "Apple",
		"DC86D8": "Apple",
		"B065BD": "Apple",
		"609AC1": "Apple",
		"C82A14": "Apple",
		"F0B479": "Apple",
		"6C4008": "Apple",
		"E0F847": "Apple",
		"009EC8": "Apple",
		"002332": "Apple",
		"002608": "Apple",
	}

	if vendor, exists := vendors[oui]; exists {
		return vendor
	}

	// Try online OUI lookup if local database doesn't have it and online lookup is enabled
	if s.enableOnlineVendorLookup {
		return s.lookupVendorOnline(oui)
	}

	return ""
}

// lookupVendorOnline attempts to lookup vendor from online OUI database
func (s *NativeScanner) lookupVendorOnline(oui string) string {
	// For production use, you might want to cache these lookups
	// This is a simple implementation

	client := &http.Client{Timeout: time.Second * 2}
	url := fmt.Sprintf("https://api.macvendors.com/%s", oui)

	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			vendor := strings.TrimSpace(string(body))
			// Clean up the vendor name
			if vendor != "Not found" && vendor != "" {
				return vendor
			}
		}
	}

	return ""
}

// getHostname attempts to resolve hostname for the IP using multiple methods
func (s *NativeScanner) getHostname(ip string) string {
	// Method 1: Standard reverse DNS lookup
	if hostname := s.reverseDNSLookup(ip); hostname != "" {
		return hostname
	}

	// Method 2: NetBIOS name resolution (Windows networks)
	if hostname := s.netBIOSLookup(ip); hostname != "" {
		return hostname
	}

	// Method 3: mDNS/Bonjour lookup (Apple/local networks)
	if hostname := s.mDNSLookup(ip); hostname != "" {
		return hostname
	}

	// Method 4: SNMP system name (if available)
	if hostname := s.snmpSystemName(ip); hostname != "" {
		return hostname
	}

	// Method 5: HTTP banner grabbing
	if hostname := s.httpBannerHostname(ip); hostname != "" {
		return hostname
	}

	return ""
}

// reverseDNSLookup performs standard reverse DNS resolution
func (s *NativeScanner) reverseDNSLookup(ip string) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return ""
	}

	// Return first hostname, trimmed
	hostname := strings.TrimSuffix(names[0], ".")
	// Clean up common suffixes
	hostname = strings.TrimSuffix(hostname, ".local")
	return hostname
}

// netBIOSLookup attempts NetBIOS name resolution (Windows)
func (s *NativeScanner) netBIOSLookup(ip string) string {
	// Try nmblookup command if available (Linux/macOS with Samba)
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		cmd := exec.Command("nmblookup", "-A", ip)
		cmd.Env = append(os.Environ(), "LC_ALL=C") // Ensure English output
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				// Look for lines like: "HOSTNAME        <00> -         B <ACTIVE>"
				if strings.Contains(line, "<00>") && strings.Contains(line, "B <ACTIVE>") {
					parts := strings.Fields(line)
					if len(parts) > 0 {
						hostname := strings.TrimSpace(parts[0])
						if hostname != "" && !strings.Contains(hostname, "__MSBROWSE__") {
							return hostname
						}
					}
				}
			}
		}
	}

	// For Windows, could use nbtstat command
	if runtime.GOOS == "windows" {
		cmd := exec.Command("nbtstat", "-A", ip)
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				// Parse Windows nbtstat output
				if strings.Contains(line, "<00>") && strings.Contains(line, "UNIQUE") {
					parts := strings.Fields(strings.TrimSpace(line))
					if len(parts) > 0 {
						hostname := strings.TrimSpace(parts[0])
						if hostname != "" {
							return hostname
						}
					}
				}
			}
		}
	}

	return ""
}

// mDNSLookup attempts mDNS/Bonjour resolution
func (s *NativeScanner) mDNSLookup(ip string) string {
	// Try common mDNS queries
	mdnsNames := []string{
		ip + ".local",
		// Could add more patterns here
	}

	for _, name := range mdnsNames {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*500)
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, name)
		cancel()

		if err == nil {
			for _, addr := range addrs {
				if addr.IP.String() == ip {
					return strings.TrimSuffix(name, ".local")
				}
			}
		}
	}

	return ""
}

// snmpSystemName attempts to get system name via SNMP
func (s *NativeScanner) snmpSystemName(ip string) string {
	// This would require an SNMP library - simplified version
	// In practice, you'd use a library like "github.com/soniah/gosnmp"

	// Try connecting to SNMP port to see if it's available
	conn, err := net.DialTimeout("udp", ip+":161", time.Millisecond*500)
	if err != nil {
		return ""
	}
	conn.Close()

	// For now, return empty - would need proper SNMP implementation
	return ""
}

// httpBannerHostname attempts to extract hostname from HTTP headers
func (s *NativeScanner) httpBannerHostname(ip string) string {
	client := &http.Client{
		Timeout: time.Second * 2,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}

	// Try common HTTP ports
	ports := []string{"80", "8080", "443", "8443"}

	for _, port := range ports {
		url := fmt.Sprintf("http://%s:%s/", ip, port)

		resp, err := client.Head(url)
		if err != nil {
			continue
		}
		resp.Body.Close()

		// Check Server header for hostname hints
		if server := resp.Header.Get("Server"); server != "" {
			// Look for hostname patterns in server header
			if hostname := s.extractHostnameFromServer(server); hostname != "" {
				return hostname
			}
		}

		// Check Location header for redirects that might contain hostname
		if location := resp.Header.Get("Location"); location != "" {
			if hostname := s.extractHostnameFromURL(location); hostname != "" {
				return hostname
			}
		}
	}

	return ""
}

// extractHostnameFromServer extracts hostname hints from Server header
func (s *NativeScanner) extractHostnameFromServer(server string) string {
	// Look for patterns like "Apache/2.4.41 (hostname.domain.com)"
	if idx := strings.Index(server, "("); idx != -1 {
		if idx2 := strings.Index(server[idx:], ")"); idx2 != -1 {
			hostname := strings.TrimSpace(server[idx+1 : idx+idx2])
			if strings.Contains(hostname, ".") && !strings.Contains(hostname, " ") {
				return hostname
			}
		}
	}
	return ""
}

// extractHostnameFromURL extracts hostname from a URL
func (s *NativeScanner) extractHostnameFromURL(urlStr string) string {
	if u, err := url.Parse(urlStr); err == nil && u.Host != "" {
		hostname := u.Hostname()
		// Only return if it's not an IP address
		if net.ParseIP(hostname) == nil {
			return hostname
		}
	}
	return ""
}

// generateIPList generates all IP addresses in a CIDR range
func (s *NativeScanner) generateIPList(ipNet *net.IPNet) []string {
	var ips []string

	// Get network and mask
	ip := ipNet.IP.To4()
	mask := ipNet.Mask

	// Calculate network start and end
	network := ip.Mask(mask)
	broadcast := make(net.IP, len(network))
	copy(broadcast, network)

	// Set all host bits to 1 for broadcast
	for i := 0; i < len(broadcast); i++ {
		broadcast[i] |= ^mask[i]
	}

	// Generate all IPs between network and broadcast (excluding them)
	for ip := make(net.IP, len(network)); ; {
		copy(ip, network)

		// Skip network address (.0) and broadcast address (.255)
		if !ip.Equal(network) && !ip.Equal(broadcast) {
			ips = append(ips, ip.String())
		}

		// Increment IP
		for j := len(ip) - 1; j >= 0; j-- {
			ip[j]++
			if ip[j] != 0 {
				break
			}
		}

		// Check if we've reached broadcast
		if ip.Equal(broadcast) {
			break
		}
	}

	return ips
}
