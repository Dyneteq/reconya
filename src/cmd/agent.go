package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"reconya/models"
)

// Agent scan state
type scanDevice struct {
	IP       string
	MAC      string
	Vendor   string
	Hostname string
	Status   string
	LastSeen time.Time
	Ports    string
}

type sensorCommandResponse struct {
	ID         string                  `json:"id"`
	Status     models.SensorStatus     `json:"status"`
	ScanStatus models.SensorScanStatus `json:"scan_status"`
}

var (
	scanInterface      string
	serverURL          string
	sensorToken        string
	scanDevices        = make(map[string]*scanDevice)
	scanMutex          sync.RWMutex
	scanStatus         string
	scanLastAction     string
	scanLogs           []string
	scanLogsMutex      sync.RWMutex
	cloudMode          bool
	cloudScanEnabled   bool
	enrichedDevices    = make(map[string]bool) // Track devices already enriched this session
	enrichedMutex      sync.Mutex
	enrichSemaphore    = make(chan struct{}, 10) // Allow more concurrent enrichments
)

// agentServices holds services for agent mode
var agentServices *Services

// runAgentScan starts the network scan on the specified interface
func runAgentScan(cmd *cobra.Command, args []string) {
	if scanInterface == "" {
		fmt.Println("Error: interface required. Use -i <interface> or 'reconya agent primary'")
		fmt.Println("Run 'reconya agent detect' to see available interfaces.")
		os.Exit(1)
	}

	// Find the interface and get its network
	iface, err := net.InterfaceByName(scanInterface)
	if err != nil {
		errorLogger.Printf("Error: interface '%s' not found: %v", scanInterface, err)
		os.Exit(1)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		errorLogger.Printf("Error getting addresses for interface '%s': %v", scanInterface, err)
		os.Exit(1)
	}

	var networkCIDR string
	var localIP string
	for _, addr := range addrs {
		ip, ipNet, err := net.ParseCIDR(addr.String())
		if err != nil || ip.To4() == nil || ip.IsLoopback() {
			continue
		}
		ones, _ := ipNet.Mask.Size()
		networkIP := ip.Mask(ipNet.Mask)
		networkCIDR = fmt.Sprintf("%s/%d", networkIP.String(), ones)
		localIP = ip.String()
		break
	}

	if networkCIDR == "" {
		errorLogger.Printf("Error: no IPv4 address found on interface '%s'", scanInterface)
		os.Exit(1)
	}

	// Initialize services (database, OUI, etc.)
	fmt.Println("Initializing services...")
	agentServices, err = initServices()
	if err != nil {
		errorLogger.Printf("Error initializing services: %v", err)
		os.Exit(1)
	}
	defer agentServices.DB.Close()

	// Find or create the network
	network, err := agentServices.NetworkService.FindOrCreate(networkCIDR)
	if err != nil {
		errorLogger.Printf("Error finding/creating network: %v", err)
		os.Exit(1)
	}
	addScanLog("Using network: %s", networkCIDR)

	// Set up signal handling for clean exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	cloudMode = serverURL != "" && sensorToken != ""
	cloudScanEnabled = !cloudMode

	// Register with remote server if configured
	localMAC := iface.HardwareAddr.String()
	if cloudMode {
		fmt.Printf("Registering with server: %s\n", serverURL)
		if err := registerWithServer(scanInterface, networkCIDR, localIP, localMAC); err != nil {
			fmt.Printf("Warning: Initial registration failed: %v\n", err)
		} else {
			fmt.Println("Registered successfully!")
		}
	}

	// Suppress log output to prevent TUI glitches
	log.SetOutput(io.Discard)

	// Start the display goroutine
	stopDisplay := make(chan bool)
	go displayScanResults(scanInterface, networkCIDR, localIP, localMAC, stopDisplay)

	// Start heartbeat loop if configured
	stopHeartbeat := make(chan bool)
	go runHeartbeatLoop(scanInterface, networkCIDR, localIP, localMAC, stopHeartbeat)

	var stopCommandWatcher chan bool
	if cloudMode {
		stopCommandWatcher = make(chan bool)
		go runRemoteCommandWatcher(stopCommandWatcher)
	}

	// Start mDNS discovery for hostnames (runs in background)
	addScanLog("Starting mDNS discovery...")
	startMDNSDiscovery()

	// Load existing devices from database immediately (fast startup)
	addScanLog("Loading cached devices...")
	refreshDevicesFromDB(network.ID)

	// Wait for mDNS discovery to complete, then enrich devices with discovered hostnames
	go func() {
		<-mdnsDiscoveryDone
		addScanLog("mDNS discovery complete, enriching devices...")
		// Re-run enrichment for devices that might now have hostnames in cache
		devices, _ := agentServices.DeviceService.FindByNetworkID(network.ID)
		for _, device := range devices {
			if device.Hostname == nil || *device.Hostname == "" {
				mdnsHostnamesMu.RLock()
				hostname := mdnsHostnames[device.IPv4]
				mdnsHostnamesMu.RUnlock()
				if hostname != "" {
					d := device
					d.Hostname = &hostname
					if saved, err := agentServices.DeviceService.CreateOrUpdate(&d); err == nil && saved != nil {
						updateScanDevice(saved)
						addScanLog("Hostname: %s → %s", d.IPv4, hostname)
					}
				}
			}
		}
	}()

	// Start the scanning loop (respects cloud scan commands)
	go runAgentScanLoop(network.ID, stopDisplay)

	// Wait for interrupt
	<-sigChan
	cloudScanEnabled = false

	// Stop scan manager if running
	if agentServices.ScanManager.IsRunning() {
		agentServices.ScanManager.StopScan()
	}

	stopDisplay <- true
	close(stopHeartbeat)
	if stopCommandWatcher != nil {
		close(stopCommandWatcher)
	}
	close(agentServices.Done)
	time.Sleep(100 * time.Millisecond)

	// Clear screen and show final message
	fmt.Print("\033[2J\033[H")
	fmt.Println("\nScan stopped. Goodbye!")
}

// addScanLog adds a log entry
func addScanLog(format string, args ...interface{}) {
	scanLogsMutex.Lock()
	defer scanLogsMutex.Unlock()

	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("15:04:05")
	logEntry := fmt.Sprintf("[%s] %s", timestamp, msg)

	scanLogs = append(scanLogs, logEntry)
	// Keep only last 5 logs
	if len(scanLogs) > 5 {
		scanLogs = scanLogs[len(scanLogs)-5:]
	}
	scanLastAction = msg
}

// runAgentScanLoop uses existing infrastructure to scan
func runAgentScanLoop(networkID string, stop chan bool) {
	scanCount := 0

	// Get the network
	network, err := agentServices.NetworkService.FindByID(networkID)
	if err != nil || network == nil {
		addScanLog("Error: network not found")
		return
	}

	for {
		select {
		case <-stop:
			return
		default:
		}

		if cloudMode && !cloudScanEnabled {
			scanStatus = "IDLE"
			time.Sleep(2 * time.Second)
			continue
		}

		select {
		case <-stop:
			return
		default:
			scanCount++
			scanStatus = "SCANNING"
			addScanLog("Starting scan #%d on %s", scanCount, network.CIDR)

			// Use FAST scan mode for quick device discovery (no hostname/vendor lookups)
			devices, err := agentServices.ScanManager.GetPingSweepService().ExecuteFastSweepScanCommand(network.CIDR)
			if err != nil {
				addScanLog("Scan error: %v", err)
			} else {
				addScanLog("Found %d devices", len(devices))

				// Process each device using existing infrastructure
				for _, device := range devices {
					device.NetworkID = network.ID

					// Save to database using DeviceService
					savedDevice, err := agentServices.DeviceService.CreateOrUpdate(&device)
					if err != nil {
						continue
					}

					// Update local display immediately
					updateScanDevice(savedDevice)

					// Background enrichment: hostname and vendor lookup
					if shouldEnrichDevice(savedDevice.IPv4) {
						go enrichDeviceInBackground(savedDevice)
					}

					// Trigger port scan if eligible
					if agentServices.DeviceService.EligibleForPortScan(savedDevice) {
						addScanLog("Port scanning %s...", savedDevice.IPv4)
						go func(d models.Device) {
							agentServices.ScanManager.GetPortScanService().Run(d)
							// Refresh device after port scan
							refreshed, _ := agentServices.DeviceService.FindByID(d.ID)
							if refreshed != nil {
								updateScanDevice(refreshed)
								addScanLog("Port scan complete: %s", refreshed.IPv4)
							}
						}(*savedDevice)
					}
				}
			}

			scanStatus = "IDLE"
			addScanLog("Scan #%d complete", scanCount)

			// Refresh all devices from database
			refreshDevicesFromDB(network.ID)

			// Sync devices to remote server
			syncDevicesToServer(network.ID)

			// Wait between scans (30 seconds)
		waitLoop:
			for i := 0; i < 300; i++ {
				select {
				case <-stop:
					return
				default:
					if cloudMode && !cloudScanEnabled {
						break waitLoop
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
		}
	}
}

// updateScanDevice updates the local display with device info from database
func updateScanDevice(device *models.Device) {
	scanMutex.Lock()
	defer scanMutex.Unlock()

	hostname := ""
	if device.Hostname != nil {
		hostname = *device.Hostname
	}

	vendor := ""
	if device.Vendor != nil {
		vendor = *device.Vendor
	}

	mac := ""
	if device.MAC != nil {
		mac = *device.MAC
	}

	// Format ports as comma-separated list
	ports := ""
	if len(device.Ports) > 0 {
		portStrs := make([]string, 0, len(device.Ports))
		for _, p := range device.Ports {
			portStrs = append(portStrs, p.Number)
		}
		ports = strings.Join(portStrs, ",")
	}

	scanDevices[device.IPv4] = &scanDevice{
		IP:       device.IPv4,
		MAC:      mac,
		Vendor:   vendor,
		Hostname: hostname,
		Status:   string(device.Status),
		LastSeen: time.Now(),
		Ports:    ports,
	}
}

// refreshDevicesFromDB refreshes all devices from database and triggers enrichment for missing data
func refreshDevicesFromDB(networkID string) {
	devices, err := agentServices.DeviceService.FindByNetworkID(networkID)
	if err != nil {
		return
	}

	for _, device := range devices {
		updateScanDevice(&device)
		// Trigger background enrichment for devices missing hostname or vendor
		if (device.Hostname == nil || *device.Hostname == "") ||
			(device.MAC != nil && *device.MAC != "" && (device.Vendor == nil || *device.Vendor == "")) {
			d := device // copy for goroutine
			if shouldEnrichDevice(d.IPv4) {
				go enrichDeviceInBackground(&d)
			}
		}
	}
}

// shouldEnrichDevice checks and marks if device should be enriched (thread-safe)
func shouldEnrichDevice(ip string) bool {
	enrichedMutex.Lock()
	defer enrichedMutex.Unlock()
	if enrichedDevices[ip] {
		return false
	}
	enrichedDevices[ip] = true
	return true
}

// enrichDeviceInBackground performs hostname and vendor lookups asynchronously
// Note: Caller must use shouldEnrichDevice() before calling this
func enrichDeviceInBackground(device *models.Device) {
	if device == nil || agentServices == nil {
		return
	}

	// Acquire semaphore to limit concurrent operations
	enrichSemaphore <- struct{}{}
	defer func() { <-enrichSemaphore }()

	// Fetch fresh data from database to avoid stale data
	freshDevice, err := agentServices.DeviceService.FindByID(device.ID)
	if err != nil || freshDevice == nil {
		return
	}

	updated := false

	// Lookup vendor FIRST (instant, local OUI database)
	if freshDevice.MAC != nil && *freshDevice.MAC != "" && (freshDevice.Vendor == nil || *freshDevice.Vendor == "") {
		vendor := agentServices.DeviceService.LookupVendor(*freshDevice.MAC)
		if vendor != "" {
			freshDevice.Vendor = &vendor
			updated = true
			addScanLog("Vendor: %s → %s", freshDevice.IPv4, vendor)
		}
	}

	// Lookup hostname - first check mDNS cache, then network lookup
	if freshDevice.Hostname == nil || *freshDevice.Hostname == "" {
		// Check mDNS cache first (populated by background discovery)
		mdnsHostnamesMu.RLock()
		hostname := mdnsHostnames[freshDevice.IPv4]
		mdnsHostnamesMu.RUnlock()

		// If not in cache, try network lookup
		if hostname == "" {
			hostname = lookupHostname(freshDevice.IPv4)
		}

		if hostname != "" {
			freshDevice.Hostname = &hostname
			updated = true
			addScanLog("Hostname: %s → %s", freshDevice.IPv4, hostname)
		}
	}

	// Save and update UI if anything changed
	if updated {
		if saved, err := agentServices.DeviceService.CreateOrUpdate(freshDevice); err == nil && saved != nil {
			updateScanDevice(saved)
		}
	}
}

// mDNS hostname cache (populated by background discovery)
var (
	mdnsHostnames       = make(map[string]string) // IP -> hostname
	mdnsHostnamesMu     sync.RWMutex
	mdnsDiscoveryDone   = make(chan struct{})
	mdnsResolvedNames   = make(map[string]bool) // Track already resolved names
	mdnsResolvedNamesMu sync.Mutex
)

// startMDNSDiscovery runs background mDNS service browsing to discover hostnames
func startMDNSDiscovery() {
	go func() {
		defer close(mdnsDiscoveryDone)

		// Services to browse for hostname discovery
		services := []string{
			"_http._tcp",
			"_workstation._tcp",
			"_device-info._tcp",
			"_airplay._tcp",
			"_printer._tcp",
			"_ipp._tcp",
			"_smb._tcp",
			"_home-assistant._tcp",
		}

		// Discover all services in parallel for faster startup
		var wg sync.WaitGroup
		for _, svc := range services {
			wg.Add(1)
			go func(s string) {
				defer wg.Done()
				discoverMDNSService(s)
			}(svc)
		}
		wg.Wait()
	}()
}

// discoverMDNSService browses a specific mDNS service type and resolves hostnames
func discoverMDNSService(serviceType string) {
	if runtime.GOOS != "darwin" {
		return
	}

	// Use dns-sd to browse and get instance names with IPs
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "dns-sd", "-Z", serviceType, "local")
	// Use CombinedOutput to get output even if command times out
	output, _ := cmd.CombinedOutput()

	// Parse output for hostnames
	// Format: "service._tcp SRV 0 0 80 hostname.local. ; comment"
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// Look for SRV records that contain hostname.local.
		if strings.Contains(line, "SRV") && strings.Contains(line, ".local.") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasSuffix(part, ".local.") {
					hostname := strings.TrimSuffix(part, ".local.")
					// Skip service type entries (contain _)
					if hostname != "" && !strings.Contains(hostname, "_") && !strings.Contains(hostname, "\\") {
						// Deduplicate - only resolve each hostname once
						mdnsResolvedNamesMu.Lock()
						if mdnsResolvedNames[hostname] {
							mdnsResolvedNamesMu.Unlock()
							continue
						}
						mdnsResolvedNames[hostname] = true
						mdnsResolvedNamesMu.Unlock()

						go resolveAndCacheHostname(hostname) // Resolve in parallel
					}
				}
			}
		}
	}
}

// resolveAndCacheHostname resolves a .local hostname to IP and caches it
func resolveAndCacheHostname(hostname string) {
	var ip string

	if runtime.GOOS == "darwin" {
		// dns-sd -G doesn't exit on its own, so we run it with timeout
		// and then kill it after getting the result
		cmd := exec.Command("timeout", "2", "dns-sd", "-G", "v4", hostname+".local")
		output, _ := cmd.CombinedOutput()

		// Parse output for IP - format: "timestamp Add flags if hostname address ttl"
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			// Skip header lines
			if strings.Contains(line, "STARTING") || strings.Contains(line, "Timestamp") || strings.Contains(line, "DATE:") {
				continue
			}
			// Look for lines with IP addresses
			parts := strings.Fields(line)
			for _, part := range parts {
				parsed := net.ParseIP(part)
				if parsed != nil && parsed.To4() != nil {
					// Found IPv4 address
					ip = part
					break
				}
			}
			if ip != "" {
				break
			}
		}
	} else {
		// Fallback to Go resolver
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", hostname+".local")
		if err == nil && len(ips) > 0 {
			ip = ips[0].String()
		}
	}

	if ip == "" || ip == "127.0.0.1" || strings.HasPrefix(ip, "127.") {
		return
	}

	mdnsHostnamesMu.Lock()
	mdnsHostnames[ip] = hostname
	mdnsHostnamesMu.Unlock()

	addScanLog("mDNS: %s → %s", ip, hostname)

	// Update device in database and display
	if agentServices != nil {
		device, err := agentServices.DeviceService.FindByIPv4(ip)
		if err == nil && device != nil {
			device.Hostname = &hostname
			if saved, err := agentServices.DeviceService.CreateOrUpdate(device); err == nil && saved != nil {
				updateScanDevice(saved)
			}
		}
	}
}

// lookupHostname tries to resolve hostname for an IP
func lookupHostname(ip string) string {
	// Check mDNS cache first
	mdnsHostnamesMu.RLock()
	if hostname, ok := mdnsHostnames[ip]; ok {
		mdnsHostnamesMu.RUnlock()
		return hostname
	}
	mdnsHostnamesMu.RUnlock()

	// Try reverse DNS with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err == nil && len(names) > 0 {
		hostname := strings.TrimSuffix(names[0], ".")
		hostname = strings.TrimSuffix(hostname, ".local")
		return hostname
	}

	return ""
}


// displayScanResults displays the scan results in terminal green style
func displayScanResults(ifaceName, networkCIDR, localIP, localMAC string, stop chan bool) {
	startTime := time.Now()
	frame := 0

	// Green terminal colors only
	green := "\033[32m"
	brightGreen := "\033[92m"
	dim := "\033[2m"
	reset := "\033[0m"
	bold := "\033[1m"

	// Spinner frames
	spinners := []string{"|", "/", "-", "\\"}

	// Switch to alternate screen buffer and hide cursor (prevents duplication on click)
	fmt.Print("\033[?1049h") // Enter alternate screen buffer
	fmt.Print("\033[?25l")   // Hide cursor
	fmt.Print("\033[2J")     // Clear screen once at start

	// Ensure we restore terminal on exit
	defer func() {
		fmt.Print("\033[?25h")   // Show cursor
		fmt.Print("\033[?1049l") // Exit alternate screen buffer
	}()

	for {
		select {
		case <-stop:
			return
		default:
			frame++
			spinner := spinners[frame%len(spinners)]

			// Move cursor to top-left (don't clear - just overwrite)
			fmt.Print("\033[H")

			elapsed := time.Since(startTime).Round(time.Second)

			// Clear to end of line escape sequence
			clr := "\033[K"

			// Header
			fmt.Printf("\n%s", clr)
			fmt.Printf("%s%s", brightGreen, bold)
			fmt.Printf("                                 __  __%s\n", clr)
			fmt.Printf("   ________  _________  ____    / / / /___ _%s\n", clr)
			fmt.Printf("  / ___/ _ \\/ ___/ __ \\/ __ \\  / / / / __ `/%s\n", clr)
			fmt.Printf(" / /  /  __/ /__/ /_/ / / / / / /_/ / /_/ /%s\n", clr)
			fmt.Printf("/_/   \\___/\\___/\\____/_/ /_/  \\__, /\\__,_/%s\n", clr)
			fmt.Printf("                             /____/%s\n", clr)
			fmt.Printf("%s%s\n", reset, clr)
			fmt.Printf("%s  Network Reconnaissance Agent%s%s\n", green, reset, clr)
			fmt.Printf("%s\n", clr)

			// Status line
			fmt.Printf("%s  [%s] SCANNING %s :: %s :: %s :: Elapsed: %s%s%s\n",
				green, spinner, ifaceName, networkCIDR, localIP, elapsed, reset, clr)
			fmt.Printf("%s  MAC: %s%s%s\n", dim, localMAC, reset, clr)
			fmt.Printf("%s\n", clr)

			// Table header
			fmt.Printf("%s  %-17s %-19s %-30s %-36s %s%s%s\n",
				brightGreen, "IP ADDRESS", "MAC ADDRESS", "VENDOR", "HOSTNAME", "STATUS", reset, clr)
			fmt.Printf("%s  ────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────%s%s\n", green, reset, clr)

			// Sort and display devices
			scanMutex.RLock()
			var sortedIPs []string
			for ip := range scanDevices {
				sortedIPs = append(sortedIPs, ip)
			}
			// Simple IP sort
			for i := 0; i < len(sortedIPs); i++ {
				for j := i + 1; j < len(sortedIPs); j++ {
					if compareIPs(sortedIPs[i], sortedIPs[j]) > 0 {
						sortedIPs[i], sortedIPs[j] = sortedIPs[j], sortedIPs[i]
					}
				}
			}

			onlineCount := 0
			offlineCount := 0

			for _, ip := range sortedIPs {
				dev := scanDevices[ip]

				// Check if device is online based on last seen time
				isOnline := time.Since(dev.LastSeen) <= 30*time.Second

				// Hide offline devices after 2 minutes of inactivity
				if !isOnline && time.Since(dev.LastSeen) > 2*time.Minute {
					continue
				}

				if isOnline {
					onlineCount++
				} else {
					offlineCount++
				}

				mac := dev.MAC
				if mac == "" {
					mac = "--:--:--:--:--:--"
				}

				vendor := dev.Vendor
				if vendor == "" {
					vendor = "-"
				}
				if len(vendor) > 28 {
					vendor = vendor[:28]
				}

				hostname := dev.Hostname
				if hostname == "" {
					hostname = "-"
				}
				if len(hostname) > 34 {
					hostname = hostname[:34]
				}

				// Status indicator
				var statusStr string
				var lineColor string
				if isOnline {
					statusStr = "● ONLINE"
					lineColor = green
				} else {
					statusStr = "○ IDLE"
					lineColor = "\033[2;32m" // dim green
				}

				fmt.Printf("%s  %-17s %-19s %-30s %-36s %s%s%s\n",
					lineColor, ip, mac, vendor, hostname, statusStr, reset, clr)

				// Show ports on next line if present (green when online, dim when offline)
				if dev.Ports != "" {
					fmt.Printf("%s    └─ ports: %s%s%s\n", lineColor, dev.Ports, reset, clr)
				}
			}
			scanMutex.RUnlock()

			// If no devices yet, show scanning message
			if onlineCount == 0 && offlineCount == 0 {
				fmt.Printf("%s  %s Scanning network...%s%s\n", dim, spinner, reset, clr)
			}

			fmt.Printf("%s  ────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────%s%s\n", green, reset, clr)

			fmt.Printf("%s  Devices: %d  |  Online: %d  |  Offline: %d%s%s\n",
				green, onlineCount+offlineCount, onlineCount, offlineCount, reset, clr)
			fmt.Printf("%s\n", clr)

			// Activity log
			fmt.Printf("%s  Activity:%s%s\n", brightGreen, reset, clr)
			scanLogsMutex.RLock()
			if len(scanLogs) == 0 {
				fmt.Printf("%s    %s Waiting for activity...%s%s\n", dim, spinner, reset, clr)
			} else {
				for _, logEntry := range scanLogs {
					fmt.Printf("%s    %s%s%s\n", dim, logEntry, reset, clr)
				}
			}
			scanLogsMutex.RUnlock()

			fmt.Printf("%s\n", clr)
			fmt.Printf("%s  [CTRL+C] Exit%s%s\n", dim, reset, clr)
			fmt.Print("\033[J") // Clear from cursor to end of screen (removes old content)

			time.Sleep(150 * time.Millisecond) // Fast refresh for responsive updates
		}
	}
}

// compareIPs compares two IP addresses for sorting
func compareIPs(a, b string) int {
	ipA := net.ParseIP(a).To4()
	ipB := net.ParseIP(b).To4()
	if ipA == nil || ipB == nil {
		return strings.Compare(a, b)
	}
	for i := 0; i < 4; i++ {
		if ipA[i] < ipB[i] {
			return -1
		}
		if ipA[i] > ipB[i] {
			return 1
		}
	}
	return 0
}

// runAgentDetect detects and displays available network interfaces
func runAgentDetect(cmd *cobra.Command, args []string) {
	interfaces, err := net.Interfaces()
	if err != nil {
		errorLogger.Printf("Error getting network interfaces: %v", err)
		os.Exit(1)
	}

	// Print header
	fmt.Println()
	fmt.Println(" reconYa Network Interface Detection")
	fmt.Println(" ════════════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf(" %-20s %-18s %-20s %-15s\n", "INTERFACE", "IPv4", "MAC", "STATUS")
	fmt.Println(" ────────────────────────────────────────────────────────────────────")

	var hasInterfaces bool
	for _, iface := range interfaces {
		// Skip loopback
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		// Get status
		status := "DOWN"
		if iface.Flags&net.FlagUp != 0 {
			status = "UP"
		}

		// Get MAC address
		mac := iface.HardwareAddr.String()
		if mac == "" {
			mac = "N/A"
		}

		// Get IPv4 addresses
		for _, addr := range addrs {
			ip, ipNet, err := net.ParseCIDR(addr.String())
			if err != nil || ip.To4() == nil {
				continue
			}

			if ip.IsLoopback() {
				continue
			}

			hasInterfaces = true

			// Calculate network CIDR
			ones, _ := ipNet.Mask.Size()
			networkIP := ip.Mask(ipNet.Mask)
			networkCIDR := fmt.Sprintf("%s/%d", networkIP.String(), ones)

			// Check if it's a Docker network
			dockerFlag := ""
			if isDockerNetwork(ip.String()) {
				dockerFlag = " [Docker]"
			}

			fmt.Printf(" %-20s %-18s %-20s %-15s%s\n",
				iface.Name,
				ip.String(),
				mac,
				status,
				dockerFlag,
			)

			// Show network info
			fmt.Printf(" %-20s └─ Network: %s\n", "", networkCIDR)
		}
	}

	if !hasInterfaces {
		fmt.Println(" No active network interfaces found.")
	}

	fmt.Println()
	fmt.Println(" ────────────────────────────────────────────────────────────────────")
	fmt.Println(" Use 'reconya agent -i <interface>' or 'reconya agent primary' to start scanning.")
	fmt.Println()
}

// isDockerNetwork checks if an IP belongs to Docker ranges
func isDockerNetwork(ip string) bool {
	dockerRanges := []string{
		"172.17.0.0/16", "172.18.0.0/16", "172.19.0.0/16",
		"172.20.0.0/16", "172.21.0.0/16", "172.22.0.0/16",
		"172.23.0.0/16", "172.24.0.0/16", "172.25.0.0/16",
		"172.26.0.0/16", "172.27.0.0/16", "172.28.0.0/16",
		"172.29.0.0/16", "172.30.0.0/16", "172.31.0.0/16",
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	for _, cidr := range dockerRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(parsedIP) {
			return true
		}
	}
	return false
}

// runAgent runs agent scan if -i is provided, otherwise shows help
func runAgent(cmd *cobra.Command, args []string) {
	if scanInterface != "" {
		runAgentScan(cmd, args)
	} else {
		cmd.Help()
	}
}

// runAgentPrimary auto-detects and uses the primary network interface
func runAgentPrimary(cmd *cobra.Command, args []string) {
	iface := detectPrimaryInterface()
	if iface == "" {
		fmt.Println("Error: Could not detect primary network interface.")
		fmt.Println("Run 'reconya agent detect' to see available interfaces.")
		os.Exit(1)
	}

	fmt.Printf("Detected primary interface: %s\n", iface)
	scanInterface = iface
	runAgentScan(cmd, args)
}

// detectPrimaryInterface finds the primary network interface
func detectPrimaryInterface() string {
	// Try to find the default route interface
	if runtime.GOOS == "darwin" {
		// macOS: use route to find default interface
		cmd := exec.Command("route", "-n", "get", "default")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "interface:") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						return parts[1]
					}
				}
			}
		}
	} else {
		// Linux: use ip route
		cmd := exec.Command("ip", "route", "show", "default")
		output, err := cmd.Output()
		if err == nil {
			// Format: default via 192.168.1.1 dev eth0
			parts := strings.Fields(string(output))
			for i, part := range parts {
				if part == "dev" && i+1 < len(parts) {
					return parts[i+1]
				}
			}
		}
	}

	// Fallback: find first non-loopback interface with IPv4
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil || ip.To4() == nil || ip.IsLoopback() {
				continue
			}

			// Skip Docker networks
			if isDockerNetwork(ip.String()) {
				continue
			}

			return iface.Name
		}
	}

	return ""
}

func getPublicIP() string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
}

func registerWithServer(ifaceName, networkCIDR, localIP, localMAC string) error {
	if serverURL == "" || sensorToken == "" {
		return nil
	}

	hostname, _ := os.Hostname()
	publicIP := getPublicIP()

	payload := map[string]string{
		"hostname":     hostname,
		"ip":           localIP,
		"public_ip":    publicIP,
		"mac":          localMAC,
		"interface":    ifaceName,
		"network_cidr": networkCIDR,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal registration data: %w", err)
	}

	url := fmt.Sprintf("%s/api/sensors/register?token=%s", strings.TrimSuffix(serverURL, "/"), sensorToken)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to register with server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registration failed with status: %d", resp.StatusCode)
	}

	var sensor models.Sensor
	if err := json.NewDecoder(resp.Body).Decode(&sensor); err == nil {
		applyRemoteScanStatus(sensor.ScanStatus)
	}

	return nil
}

// runHeartbeatLoop sends periodic heartbeats to the server
func runHeartbeatLoop(ifaceName, networkCIDR, localIP, localMAC string, stop chan bool) {
	if serverURL == "" || sensorToken == "" {
		return // Not configured for remote registration
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if err := registerWithServer(ifaceName, networkCIDR, localIP, localMAC); err != nil {
				addScanLog("Heartbeat failed: %v", err)
			}
		}
	}
}

// runRemoteCommandWatcher polls the server for scan commands to keep remote control responsive.
func runRemoteCommandWatcher(stop chan bool) {
	if serverURL == "" || sensorToken == "" {
		return
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			status, err := fetchRemoteScanStatus()
			if err != nil {
				errorLogger.Printf("Command poll failed: %v", err)
				continue
			}
			applyRemoteScanStatus(status)
		}
	}
}

func fetchRemoteScanStatus() (models.SensorScanStatus, error) {
	url := fmt.Sprintf("%s/api/sensors/command?token=%s", strings.TrimSuffix(serverURL, "/"), sensorToken)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return models.SensorScanStatusIdle, fmt.Errorf("build command request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return models.SensorScanStatusIdle, fmt.Errorf("command request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.SensorScanStatusIdle, fmt.Errorf("command endpoint returned %s", resp.Status)
	}

	var payload sensorCommandResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return models.SensorScanStatusIdle, fmt.Errorf("decode command response: %w", err)
	}

	return payload.ScanStatus, nil
}

func applyRemoteScanStatus(status models.SensorScanStatus) {
	if !cloudMode {
		return
	}

	requested := status == models.SensorScanStatusRunning
	if requested == cloudScanEnabled {
		return
	}

	cloudScanEnabled = requested
	if requested {
		addScanLog("Remote command received: start scanning")
	} else {
		addScanLog("Remote command received: stop scanning")
	}
}

// syncDevicesToServer sends discovered devices to the remote server
func syncDevicesToServer(networkID string) {
	if !cloudMode || serverURL == "" || sensorToken == "" {
		return
	}

	devices, err := agentServices.DeviceService.FindByNetworkID(networkID)
	if err != nil || len(devices) == 0 {
		return
	}

	// Build device payload
	type portInfo struct {
		Number   string `json:"number"`
		Protocol string `json:"protocol"`
		Service  string `json:"service,omitempty"`
	}
	type devicePayload struct {
		IPv4     string     `json:"ipv4"`
		MAC      *string    `json:"mac,omitempty"`
		Vendor   *string    `json:"vendor,omitempty"`
		Hostname *string    `json:"hostname,omitempty"`
		Status   string     `json:"status"`
		Ports    []portInfo `json:"ports,omitempty"`
	}

	var payload []devicePayload
	for _, d := range devices {
		dp := devicePayload{
			IPv4:     d.IPv4,
			MAC:      d.MAC,
			Vendor:   d.Vendor,
			Hostname: d.Hostname,
			Status:   string(d.Status),
		}
		for _, p := range d.Ports {
			dp.Ports = append(dp.Ports, portInfo{
				Number:   p.Number,
				Protocol: p.Protocol,
				Service:  p.Service,
			})
		}
		payload = append(payload, dp)
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		addScanLog("Failed to marshal devices: %v", err)
		return
	}

	url := fmt.Sprintf("%s/api/sensors/devices?token=%s", strings.TrimSuffix(serverURL, "/"), sensorToken)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		addScanLog("Failed to sync devices: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		addScanLog("Synced %d devices to server", len(devices))
	} else {
		addScanLog("Device sync failed: %s", resp.Status)
	}
}
