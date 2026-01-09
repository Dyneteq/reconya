package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"reconya/internal/config"
	"reconya/models"
)

type scanDevice struct {
	IP       string
	MAC      string
	Vendor   string
	Hostname string
	Status   string
	LastSeen time.Time
	Ports    string
}

var (
	scanInterface      string
	scanDevices        = make(map[string]*scanDevice)
	scanMutex          sync.RWMutex
	scanStatus         string
	scanLastAction     string
	scanLogs           []string
	scanLogsMutex      sync.RWMutex
	enrichedDevices    = make(map[string]bool)
	enrichedMutex      sync.Mutex
	enrichSemaphore    = make(chan struct{}, 10)
	systemStatusMu     sync.RWMutex
	systemStatusIP     string
	systemStatusGeo    string
)

var agentServices *Services

func runAgentScan(cmd *cobra.Command, args []string) {
	if scanInterface == "" {
		fmt.Println("Error: interface required. Use -i <interface> or 'reconya agent primary'")
		fmt.Println("Run 'reconya agent detect' to see available interfaces.")
		os.Exit(1)
	}

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

	fmt.Println("Initializing services...")
	agentServices, err = initServices()
	if err != nil {
		errorLogger.Printf("Error initializing services: %v", err)
		os.Exit(1)
	}
	defer agentServices.DB.Close()

	network, err := agentServices.NetworkService.FindOrCreate(networkCIDR)
	if err != nil {
		errorLogger.Printf("Error finding/creating network: %v", err)
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	localMAC := iface.HardwareAddr.String()
	startAgentWithServices(agentServices, scanInterface, network.ID, networkCIDR, localIP, localMAC)

	<-sigChan
	if agentServices.ScanManager.IsRunning() {
		agentServices.ScanManager.StopScan()
	}

	close(agentServices.Done)
	time.Sleep(config.AgentShutdownPause)

	fmt.Print("\033[2J\033[H")
	fmt.Println("\nScan stopped. Goodbye!")
}

func startAgentWithServices(svc *Services, ifaceName, networkID, networkCIDR, localIP, localMAC string) {
	agentServices = svc
	addScanLog("Using network: %s", networkCIDR)

	log.SetOutput(io.Discard)

	stopDisplay := make(chan bool)
	stopOnce := sync.Once{}
	stop := func() {
		stopOnce.Do(func() {
			close(stopDisplay)
		})
	}

	go displayScanResults(ifaceName, networkCIDR, localIP, localMAC, stopDisplay)

	addScanLog("Starting mDNS discovery...")
	startMDNSDiscovery()

	addScanLog("Loading cached devices...")
	refreshDevicesFromDB(networkID)
	updateAgentSystemStatusCache()

	go func() {
		<-mdnsDiscoveryDone
		addScanLog("mDNS discovery complete, enriching devices...")
		devices, _ := agentServices.DeviceService.FindByNetworkID(networkID)
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

	go runAgentScanLoop(networkID, stopDisplay)

	go func() {
		<-svc.Done
		if agentServices.ScanManager.IsRunning() {
			agentServices.ScanManager.StopScan()
		}
		stop()
	}()
}

func updateAgentSystemStatusCache() {
	if agentServices == nil || agentServices.SystemStatusService == nil {
		return
	}

	status, err := agentServices.SystemStatusService.GetLatest()
	if err != nil || status == nil {
		systemStatusMu.Lock()
		systemStatusIP = ""
		systemStatusGeo = ""
		systemStatusMu.Unlock()
		return
	}

	publicIP := ""
	if status.PublicIP != nil {
		publicIP = *status.PublicIP
	}

	geo := ""
	if status.Geolocation != nil {
		parts := []string{}
		if status.Geolocation.City != "" {
			parts = append(parts, status.Geolocation.City)
		}
		if status.Geolocation.Region != "" {
			parts = append(parts, status.Geolocation.Region)
		}
		if status.Geolocation.Country != "" {
			parts = append(parts, status.Geolocation.Country)
		}
		geo = strings.Join(parts, ", ")
	}

	systemStatusMu.Lock()
	systemStatusIP = publicIP
	systemStatusGeo = geo
	systemStatusMu.Unlock()
}

func addScanLog(format string, args ...interface{}) {
	scanLogsMutex.Lock()
	defer scanLogsMutex.Unlock()

	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("15:04:05")
	logEntry := fmt.Sprintf("[%s] %s", timestamp, msg)

	scanLogs = append(scanLogs, logEntry)
	if len(scanLogs) > 5 {
		scanLogs = scanLogs[len(scanLogs)-5:]
	}
	scanLastAction = msg
}

func runAgentScanLoop(networkID string, stop chan bool) {
	scanCount := 0

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

		select {
		case <-stop:
			return
		default:
			scanCount++
			scanStatus = "SCANNING"
			addScanLog("Starting scan #%d on %s", scanCount, network.CIDR)

			devices, err := agentServices.ScanManager.GetPingSweepService().ExecuteFastSweepScanCommand(network.CIDR)
			if err != nil {
				addScanLog("Scan error: %v", err)
			} else {
				addScanLog("Found %d devices", len(devices))

				for _, device := range devices {
					device.NetworkID = network.ID

					savedDevice, err := agentServices.DeviceService.CreateOrUpdate(&device)
					if err != nil {
						continue
					}

					updateScanDevice(savedDevice)

					if shouldEnrichDevice(savedDevice.IPv4) {
						go enrichDeviceInBackground(savedDevice)
					}

					if agentServices.DeviceService.EligibleForPortScan(savedDevice) {
						addScanLog("Port scanning %s...", savedDevice.IPv4)
						go func(d models.Device) {
							agentServices.ScanManager.GetPortScanService().Run(d)
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

			refreshDevicesFromDB(network.ID)

			waitTimer := time.NewTimer(config.AgentScanInterval)
			select {
			case <-stop:
				if !waitTimer.Stop() {
					<-waitTimer.C
				}
				return
			case <-waitTimer.C:
			}
		}
	}
}

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

func refreshDevicesFromDB(networkID string) {
	devices, err := agentServices.DeviceService.FindByNetworkID(networkID)
	if err != nil {
		return
	}

	for _, device := range devices {
		updateScanDevice(&device)
		if (device.Hostname == nil || *device.Hostname == "") ||
			(device.MAC != nil && *device.MAC != "" && (device.Vendor == nil || *device.Vendor == "")) {
			d := device
			if shouldEnrichDevice(d.IPv4) {
				go enrichDeviceInBackground(&d)
			}
		}
	}
}

func shouldEnrichDevice(ip string) bool {
	enrichedMutex.Lock()
	defer enrichedMutex.Unlock()
	if enrichedDevices[ip] {
		return false
	}
	enrichedDevices[ip] = true
	return true
}

func enrichDeviceInBackground(device *models.Device) {
	if device == nil || agentServices == nil {
		return
	}

	enrichSemaphore <- struct{}{}
	defer func() { <-enrichSemaphore }()

	freshDevice, err := agentServices.DeviceService.FindByID(device.ID)
	if err != nil || freshDevice == nil {
		return
	}

	updated := false

	if freshDevice.MAC != nil && *freshDevice.MAC != "" && (freshDevice.Vendor == nil || *freshDevice.Vendor == "") {
		vendor := agentServices.DeviceService.LookupVendor(*freshDevice.MAC)
		if vendor != "" {
			freshDevice.Vendor = &vendor
			updated = true
			addScanLog("Vendor: %s → %s", freshDevice.IPv4, vendor)
		}
	}

	if freshDevice.Hostname == nil || *freshDevice.Hostname == "" {
		mdnsHostnamesMu.RLock()
		hostname := mdnsHostnames[freshDevice.IPv4]
		mdnsHostnamesMu.RUnlock()

		if hostname == "" {
			hostname = lookupHostname(freshDevice.IPv4)
		}

		if hostname != "" {
			freshDevice.Hostname = &hostname
			updated = true
			addScanLog("Hostname: %s → %s", freshDevice.IPv4, hostname)
		}
	}

	if updated {
		if saved, err := agentServices.DeviceService.CreateOrUpdate(freshDevice); err == nil && saved != nil {
			updateScanDevice(saved)
		}
	}
}

var (
	mdnsHostnames       = make(map[string]string)
	mdnsHostnamesMu     sync.RWMutex
	mdnsDiscoveryDone   = make(chan struct{})
	mdnsResolvedNames   = make(map[string]bool)
	mdnsResolvedNamesMu sync.Mutex
	mdnsDiscoveryOnce   sync.Once
)

func startMDNSDiscovery() {
	go func() {
	interval := config.MDNSDiscoveryInterval
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

		runDiscovery := func() {
			var wg sync.WaitGroup
			for _, svc := range services {
				wg.Add(1)
				go func(s string) {
					defer wg.Done()
					discoverMDNSService(s)
				}(svc)
			}
			wg.Wait()
			mdnsDiscoveryOnce.Do(func() {
				close(mdnsDiscoveryDone)
			})
		}

		runDiscovery()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			runDiscovery()
		}
	}()
}

func discoverMDNSService(serviceType string) {
	if runtime.GOOS != "darwin" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.MDNSDiscoveryTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "dns-sd", "-Z", serviceType, "local")
	output, _ := cmd.CombinedOutput()

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "SRV") && strings.Contains(line, ".local.") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasSuffix(part, ".local.") {
					hostname := strings.TrimSuffix(part, ".local.")
					if hostname != "" && !strings.Contains(hostname, "_") && !strings.Contains(hostname, "\\") {
						mdnsResolvedNamesMu.Lock()
						if mdnsResolvedNames[hostname] {
							mdnsResolvedNamesMu.Unlock()
							continue
						}
						mdnsResolvedNames[hostname] = true
						mdnsResolvedNamesMu.Unlock()

						go resolveAndCacheHostname(hostname)
					}
				}
			}
		}
	}
}

func resolveAndCacheHostname(hostname string) {
	var ip string

	if runtime.GOOS == "darwin" {
		cmd := exec.Command("timeout", strconv.Itoa(int(config.MDNSResolveCommandTimeout.Seconds())), "dns-sd", "-G", "v4", hostname+".local")
		output, _ := cmd.CombinedOutput()

		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "STARTING") || strings.Contains(line, "Timestamp") || strings.Contains(line, "DATE:") {
				continue
			}
			parts := strings.Fields(line)
			for _, part := range parts {
				parsed := net.ParseIP(part)
				if parsed != nil && parsed.To4() != nil {
					ip = part
					break
				}
			}
			if ip != "" {
				break
			}
		}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), config.MDNSFallbackTimeout)
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

func lookupHostname(ip string) string {
	mdnsHostnamesMu.RLock()
	if hostname, ok := mdnsHostnames[ip]; ok {
		mdnsHostnamesMu.RUnlock()
		return hostname
	}
	mdnsHostnamesMu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), config.ReverseDNSTimeout)
	defer cancel()

	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err == nil && len(names) > 0 {
		hostname := strings.TrimSuffix(names[0], ".")
		hostname = strings.TrimSuffix(hostname, ".local")
		return hostname
	}

	return ""
}


func displayScanResults(ifaceName, networkCIDR, localIP, localMAC string, stop chan bool) {
	startTime := time.Now()
	frame := 0

	green := "\033[32m"
	brightGreen := "\033[92m"
	dim := "\033[2m"
	reset := "\033[0m"
	bold := "\033[1m"

	spinners := []string{"|", "/", "-", "\\"}

	fmt.Print("\033[?1049h")
	fmt.Print("\033[?25l")
	fmt.Print("\033[2J")

	defer func() {
		fmt.Print("\033[?25h")
		fmt.Print("\033[?1049l")
	}()

	for {
		select {
		case <-stop:
			return
		default:
			frame++
			spinner := spinners[frame%len(spinners)]

			fmt.Print("\033[H")

			elapsed := time.Since(startTime).Round(time.Second)

			clr := "\033[K"

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

			fmt.Printf("%s  [%s] SCANNING %s :: %s :: %s :: Elapsed: %s%s%s\n",
				green, spinner, ifaceName, networkCIDR, localIP, elapsed, reset, clr)
			fmt.Printf("%s  MAC: %s%s%s\n", dim, localMAC, reset, clr)
			systemStatusMu.RLock()
			publicIP := systemStatusIP
			geo := systemStatusGeo
			systemStatusMu.RUnlock()
			if publicIP == "" {
				publicIP = "N/A"
			}
			if geo == "" {
				geo = "N/A"
			}
			fmt.Printf("%s  Public IP: %s  Geo: %s%s%s\n", dim, publicIP, geo, reset, clr)
			fmt.Printf("%s\n", clr)

			fmt.Printf("%s  %-17s %-19s %-30s %-36s %s%s%s\n",
				brightGreen, "IP ADDRESS", "MAC ADDRESS", "VENDOR", "HOSTNAME", "STATUS", reset, clr)
			fmt.Printf("%s  ────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────%s%s\n", green, reset, clr)

			scanMutex.RLock()
			var sortedIPs []string
			for ip := range scanDevices {
				sortedIPs = append(sortedIPs, ip)
			}
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

				isOnline := time.Since(dev.LastSeen) <= config.AgentOnlineThreshold

				if !isOnline && time.Since(dev.LastSeen) > config.AgentOfflineHideAfter {
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

				var statusStr string
				var lineColor string
				if isOnline {
					statusStr = "● ONLINE"
					lineColor = green
				} else {
					statusStr = "○ IDLE"
					lineColor = "\033[2;32m"
				}

				fmt.Printf("%s  %-17s %-19s %-30s %-36s %s%s%s\n",
					lineColor, ip, mac, vendor, hostname, statusStr, reset, clr)

				if dev.Ports != "" {
					fmt.Printf("%s    └─ ports: %s%s%s\n", lineColor, dev.Ports, reset, clr)
				}
			}
			scanMutex.RUnlock()

			if onlineCount == 0 && offlineCount == 0 {
				fmt.Printf("%s  %s Scanning network...%s%s\n", dim, spinner, reset, clr)
			}

			fmt.Printf("%s  ────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────%s%s\n", green, reset, clr)

			fmt.Printf("%s  Devices: %d  |  Online: %d  |  Offline: %d%s%s\n",
				green, onlineCount+offlineCount, onlineCount, offlineCount, reset, clr)
			fmt.Printf("%s\n", clr)

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
			fmt.Print("\033[J")

			time.Sleep(config.AgentUIRefreshInterval)
		}
	}
}

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

func runAgentDetect(cmd *cobra.Command, args []string) {
	interfaces, err := net.Interfaces()
	if err != nil {
		errorLogger.Printf("Error getting network interfaces: %v", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println(" reconYa Network Interface Detection")
	fmt.Println(" ════════════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf(" %-20s %-18s %-20s %-15s\n", "INTERFACE", "IPv4", "MAC", "STATUS")
	fmt.Println(" ────────────────────────────────────────────────────────────────────")

	var hasInterfaces bool
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		status := "DOWN"
		if iface.Flags&net.FlagUp != 0 {
			status = "UP"
		}

		mac := iface.HardwareAddr.String()
		if mac == "" {
			mac = "N/A"
		}

		for _, addr := range addrs {
			ip, ipNet, err := net.ParseCIDR(addr.String())
			if err != nil || ip.To4() == nil {
				continue
			}

			if ip.IsLoopback() {
				continue
			}

			hasInterfaces = true

			ones, _ := ipNet.Mask.Size()
			networkIP := ip.Mask(ipNet.Mask)
			networkCIDR := fmt.Sprintf("%s/%d", networkIP.String(), ones)

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

func runAgent(cmd *cobra.Command, args []string) {
	if scanInterface != "" {
		runAgentScan(cmd, args)
	} else {
		cmd.Help()
	}
}

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

func detectPrimaryInterface() string {
	if runtime.GOOS == "darwin" {
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
		cmd := exec.Command("ip", "route", "show", "default")
		output, err := cmd.Output()
		if err == nil {
			parts := strings.Fields(string(output))
			for i, part := range parts {
				if part == "dev" && i+1 < len(parts) {
					return parts[i+1]
				}
			}
		}
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
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

			if isDockerNetwork(ip.String()) {
				continue
			}

			return iface.Name
		}
	}

	return ""
}
