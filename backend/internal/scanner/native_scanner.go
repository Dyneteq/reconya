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
// Uses parallel probing for faster detection
func (s *NativeScanner) tryTCPConnect(ip string) (bool, time.Duration) {
	commonPorts := []int{80, 443, 22, 21, 23, 25, 53, 135, 139, 445, 3389, 8080}

	start := time.Now()

	// Use a context with timeout for the entire probe operation
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*800)
	defer cancel()

	// Channel to receive success signal
	resultChan := make(chan bool, len(commonPorts))

	// Probe all ports in parallel
	for _, port := range commonPorts {
		go func(p int) {
			address := fmt.Sprintf("%s:%d", ip, p)
			dialer := net.Dialer{Timeout: time.Millisecond * 500}
			conn, err := dialer.DialContext(ctx, "tcp", address)
			if err == nil {
				conn.Close()
				resultChan <- true
			} else {
				resultChan <- false
			}
		}(port)
	}

	// Wait for first success or all failures
	for i := 0; i < len(commonPorts); i++ {
		select {
		case success := <-resultChan:
			if success {
				return true, time.Since(start)
			}
		case <-ctx.Done():
			return false, 0
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

	// Get network address as 4-byte slice
	ip := ipNet.IP.To4()
	if ip == nil {
		return ips // Not an IPv4 address
	}

	mask := ipNet.Mask

	// Calculate number of host bits
	ones, bits := mask.Size()
	hostBits := bits - ones

	// Calculate total number of addresses (excluding network and broadcast)
	totalHosts := (1 << hostBits) - 2
	if totalHosts <= 0 {
		return ips
	}

	// Get the network address
	network := ip.Mask(mask)

	// Convert network address to uint32 for easier arithmetic
	networkInt := uint32(network[0])<<24 | uint32(network[1])<<16 | uint32(network[2])<<8 | uint32(network[3])

	// Generate all host IPs (skip network address at +0 and broadcast at end)
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

// PortScanResult represents the result of scanning a single port
type PortScanResult struct {
	Port     int
	Open     bool
	Service  string
	Protocol string
	Banner   string
}

// ScanPorts performs TCP connect scanning on the specified ports
func (s *NativeScanner) ScanPorts(ip string, ports []int) []PortScanResult {
	log.Printf("Starting native port scan on %s (%d ports)", ip, len(ports))

	portChan := make(chan int, len(ports))
	resultChan := make(chan PortScanResult, len(ports))

	// Fill port channel
	for _, port := range ports {
		portChan <- port
	}
	close(portChan)

	// Start workers - use more workers for port scanning since it's I/O bound
	numWorkers := 100
	if numWorkers > len(ports) {
		numWorkers = len(ports)
	}

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for port := range portChan {
				result := s.scanPort(ip, port)
				resultChan <- result
			}
		}()
	}

	// Close result channel when all workers done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect open ports
	var results []PortScanResult
	for result := range resultChan {
		if result.Open {
			results = append(results, result)
		}
	}

	log.Printf("Port scan completed for %s. Found %d open ports", ip, len(results))
	return results
}

// scanPort checks if a single port is open using TCP connect
func (s *NativeScanner) scanPort(ip string, port int) PortScanResult {
	result := PortScanResult{
		Port:     port,
		Protocol: "tcp",
		Service:  getServiceName(port),
	}

	address := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", address, time.Millisecond*500)
	if err != nil {
		return result
	}
	defer conn.Close()

	result.Open = true

	// Try to grab banner for certain services
	if shouldGrabBanner(port) {
		result.Banner = grabBanner(conn, port)
	}

	return result
}

// shouldGrabBanner determines if we should try banner grabbing for this port
func shouldGrabBanner(port int) bool {
	bannerPorts := map[int]bool{
		21: true, 22: true, 23: true, 25: true, 80: true,
		110: true, 143: true, 443: true, 587: true, 993: true,
		995: true, 3306: true, 5432: true, 6379: true, 8080: true,
	}
	return bannerPorts[port]
}

// grabBanner attempts to read a banner from the connection
func grabBanner(conn net.Conn, port int) string {
	conn.SetReadDeadline(time.Now().Add(time.Second * 2))

	// For HTTP ports, send a request
	if port == 80 || port == 8080 || port == 8000 || port == 8888 {
		conn.Write([]byte("HEAD / HTTP/1.0\r\nHost: localhost\r\n\r\n"))
	} else if port == 443 || port == 8443 {
		// Skip banner for HTTPS - would need TLS
		return ""
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return ""
	}

	banner := strings.TrimSpace(string(buf[:n]))
	// Limit banner length
	if len(banner) > 200 {
		banner = banner[:200]
	}
	return banner
}

// getServiceName returns the common service name for a port
func getServiceName(port int) string {
	services := map[int]string{
		20:    "ftp-data",
		21:    "ftp",
		22:    "ssh",
		23:    "telnet",
		25:    "smtp",
		53:    "domain",
		67:    "dhcps",
		68:    "dhcpc",
		69:    "tftp",
		80:    "http",
		88:    "kerberos",
		110:   "pop3",
		111:   "rpcbind",
		119:   "nntp",
		123:   "ntp",
		135:   "msrpc",
		137:   "netbios-ns",
		138:   "netbios-dgm",
		139:   "netbios-ssn",
		143:   "imap",
		161:   "snmp",
		162:   "snmptrap",
		179:   "bgp",
		194:   "irc",
		389:   "ldap",
		443:   "https",
		445:   "microsoft-ds",
		464:   "kpasswd",
		465:   "smtps",
		500:   "isakmp",
		514:   "syslog",
		515:   "printer",
		520:   "route",
		521:   "ripng",
		543:   "klogin",
		544:   "kshell",
		548:   "afp",
		554:   "rtsp",
		587:   "submission",
		631:   "ipp",
		636:   "ldapssl",
		646:   "ldp",
		873:   "rsync",
		902:   "vmware-auth",
		993:   "imaps",
		995:   "pop3s",
		1080:  "socks",
		1194:  "openvpn",
		1433:  "ms-sql-s",
		1434:  "ms-sql-m",
		1521:  "oracle",
		1723:  "pptp",
		1883:  "mqtt",
		1900:  "upnp",
		2049:  "nfs",
		2082:  "cpanel",
		2083:  "cpanel-ssl",
		2181:  "zookeeper",
		2222:  "ssh-alt",
		2375:  "docker",
		2376:  "docker-ssl",
		3000:  "grafana",
		3128:  "squid",
		3268:  "globalcat",
		3269:  "globalcat-ssl",
		3306:  "mysql",
		3389:  "ms-wbt-server",
		3690:  "svn",
		4000:  "remoteanything",
		4443:  "https-alt",
		4444:  "krb524",
		4567:  "tram",
		4848:  "glassfish",
		5000:  "upnp",
		5001:  "synology",
		5060:  "sip",
		5061:  "sips",
		5222:  "xmpp-client",
		5269:  "xmpp-server",
		5432:  "postgresql",
		5555:  "adb",
		5672:  "amqp",
		5900:  "vnc",
		5984:  "couchdb",
		6000:  "x11",
		6379:  "redis",
		6443:  "kubernetes",
		6667:  "irc",
		7001:  "weblogic",
		7002:  "weblogic-ssl",
		8000:  "http-alt",
		8008:  "http-alt",
		8080:  "http-proxy",
		8081:  "http-alt",
		8083:  "us-srv",
		8086:  "influxdb",
		8087:  "riak",
		8088:  "radan-http",
		8443:  "https-alt",
		8888:  "http-alt",
		9000:  "cslistener",
		9001:  "tor-orport",
		9042:  "cassandra",
		9090:  "zeus-admin",
		9091:  "transmission",
		9092:  "kafka",
		9100:  "jetdirect",
		9200:  "elasticsearch",
		9300:  "elasticsearch",
		9418:  "git",
		9999:  "abyss",
		10000: "webmin",
		10050: "zabbix-agent",
		10051: "zabbix-trapper",
		11211: "memcached",
		15672: "rabbitmq-mgmt",
		27017: "mongodb",
		27018: "mongodb",
		28017: "mongodb-web",
		50000: "db2",
		50070: "hadoop-namenode",
	}

	if name, ok := services[port]; ok {
		return name
	}
	return "unknown"
}

// GetDefaultPorts returns the default list of ports to scan (top ports + common services)
func GetDefaultPorts() []int {
	return []int{
		// Well-known ports
		21, 22, 23, 25, 53, 80, 110, 111, 135, 139, 143, 161, 162, 389, 443, 445,
		465, 514, 548, 554, 587, 631, 636, 873, 993, 995,
		// Common application ports
		1080, 1194, 1433, 1434, 1521, 1723, 1883, 1900,
		2049, 2082, 2083, 2181, 2222, 2375, 2376,
		3000, 3128, 3268, 3306, 3389, 3690,
		4000, 4443, 4848,
		5000, 5001, 5060, 5222, 5432, 5555, 5672, 5900, 5984,
		6000, 6379, 6443, 6667,
		7001, 7002,
		8000, 8008, 8080, 8081, 8083, 8086, 8088, 8443, 8888,
		9000, 9001, 9042, 9090, 9091, 9092, 9100, 9200, 9418, 9999,
		10000, 10050, 10051, 11211,
		15672,
		27017, 27018,
		50000, 50070,
	}
}
