package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"reconya/db"
	"reconya/internal/config"
	"reconya/internal/device"
	"reconya/internal/eventlog"
	"reconya/internal/ipv6monitor"
	"reconya/internal/network"
	"reconya/internal/nicidentifier"
	"reconya/internal/oui"
	"reconya/internal/pingsweep"
	"reconya/internal/portscan"
	"reconya/internal/scan"
	"reconya/internal/settings"
	"reconya/internal/systemstatus"
	"reconya/internal/web"
	"reconya/middleware"
)

func runDeviceUpdater(service *device.DeviceService, done <-chan bool) {
	defer func() {
		if r := recover(); r != nil {
			errorLogger.Printf("Device updater panic recovered: %v", r)
			errorLogger.Printf("Device updater stack trace: %s", debug.Stack())
		}
		infoLogger.Println("Device updater service stopped")
	}()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	infoLogger.Println("Device updater started")
	for {
		select {
		case <-done:
			infoLogger.Println("Device updater received shutdown signal")
			return
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						errorLogger.Printf("UpdateDeviceStatuses panic: %v", r)
						errorLogger.Printf("UpdateDeviceStatuses stack: %s", debug.Stack())
					}
				}()
				err := service.UpdateDeviceStatuses()
				if err != nil {
					infoLogger.Printf("Failed to update device statuses: %v", err)
					time.Sleep(1 * time.Second)
				}
			}()
		}
	}
}

func runGeolocationCacheCleanup(repo *db.GeolocationRepository, done <-chan bool) {
	defer func() {
		if r := recover(); r != nil {
			errorLogger.Printf("Geolocation cache cleanup panic recovered: %v", r)
			errorLogger.Printf("Cache cleanup stack trace: %s", debug.Stack())
		}
		infoLogger.Println("Geolocation cache cleanup service stopped")
	}()

	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	infoLogger.Println("Geolocation cache cleanup service started")

	ctx := context.Background()
	if err := repo.CleanupExpired(ctx); err != nil {
		errorLogger.Printf("Initial geolocation cache cleanup failed: %v", err)
	}

	for {
		select {
		case <-done:
			infoLogger.Println("Geolocation cache cleanup received shutdown signal")
			return
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						errorLogger.Printf("Cache cleanup iteration panic: %v", r)
					}
				}()
				
				if err := repo.CleanupExpired(ctx); err != nil {
					errorLogger.Printf("Geolocation cache cleanup failed: %v", err)
				}
			}()
		}
	}
}

func runNetworkDetection(nicService *nicidentifier.NicIdentifierService, done <-chan bool) {
	defer func() {
		if r := recover(); r != nil {
			errorLogger.Printf("Network detection panic recovered: %v", r)
			errorLogger.Printf("Network detection stack trace: %s", debug.Stack())
		}
		infoLogger.Println("Network detection service stopped")
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	infoLogger.Println("Network detection service started")

	for {
		select {
		case <-done:
			infoLogger.Println("Network detection received shutdown signal")
			return
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						errorLogger.Printf("Network detection iteration panic: %v", r)
					}
				}()

				nicService.CheckForNewNetworks()
			}()
		}
	}
}

var (
	infoLogger  = log.New(os.Stdout, "", log.LstdFlags)
	errorLogger = log.New(os.Stderr, "", log.LstdFlags)
)

// Services holds all initialized services
type Services struct {
	Config              *config.Config
	DB                  *sql.DB
	RepoFactory         *db.RepositoryFactory
	NetworkService      *network.NetworkService
	DeviceService       *device.DeviceService
	EventLogService     *eventlog.EventLogService
	SystemStatusService *systemstatus.SystemStatusService
	SettingsService     *settings.SettingsService
	ScanManager         *scan.ScanManager
	NicService          *nicidentifier.NicIdentifierService
	GeolocationRepo     *db.GeolocationRepository
	Done                chan bool
}

// initServices initializes all services needed by the application
func initServices() (*Services, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	infoLogger.Println("Using SQLite database")
	sqliteDB, err := db.ConnectToSQLite(cfg.SQLitePath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SQLite: %w", err)
	}

	if err := db.InitializeSchema(sqliteDB); err != nil {
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	infoLogger.Println("Resetting port scan cooldowns for development...")
	if err := db.ResetPortScanCooldowns(sqliteDB); err != nil {
		infoLogger.Printf("Warning: Failed to reset port scan cooldowns: %v", err)
	}

	repoFactory := db.NewRepositoryFactory(sqliteDB, cfg.DatabaseName)

	networkRepo := repoFactory.NewNetworkRepository()
	deviceRepo := repoFactory.NewDeviceRepository()
	eventLogRepo := repoFactory.NewEventLogRepository()
	systemStatusRepo := repoFactory.NewSystemStatusRepository()
	geolocationRepo := repoFactory.NewGeolocationRepository()
	settingsRepo := repoFactory.NewSettingsRepository()

	dbManager := db.NewDBManager()

	ouiDataPath := filepath.Join(filepath.Dir(cfg.SQLitePath), "oui")
	ouiService := oui.NewOUIService(ouiDataPath)
	infoLogger.Println("Initializing OUI service...")
	if err := ouiService.Initialize(); err != nil {
		infoLogger.Printf("Warning: Failed to initialize OUI service: %v", err)
		infoLogger.Println("Continuing without OUI service - vendor lookup will rely on Nmap only")
		ouiService = nil
	} else {
		stats := ouiService.GetStatistics()
		infoLogger.Printf("OUI service initialized successfully - %v entries loaded, last updated: %v",
			stats["total_entries"], stats["last_updated"])
	}

	networkService := network.NewNetworkService(networkRepo, cfg, dbManager)
	deviceService := device.NewDeviceService(deviceRepo, networkService, cfg, dbManager, ouiService)
	eventLogService := eventlog.NewEventLogService(eventLogRepo, deviceService, dbManager)
	systemStatusService := systemstatus.NewSystemStatusService(systemStatusRepo, geolocationRepo)
	settingsService := settings.NewSettingsService(settingsRepo)
	portScanService := portscan.NewPortScanService(deviceService, eventLogService)
	pingSweepService := pingsweep.NewPingSweepService(cfg, deviceService, eventLogService, networkService, portScanService)

	ipv6MonitorService := ipv6monitor.NewIPv6MonitorService(deviceService, networkService, infoLogger)

	scanManager := scan.NewScanManager(pingSweepService, networkService, ipv6MonitorService)

	nicService := nicidentifier.NewNicIdentifierService(networkService, systemStatusService, eventLogService, deviceService, cfg)

	done := make(chan bool)

	nicService.Identify()

	go runDeviceUpdater(deviceService, done)
	go runNetworkDetection(nicService, done)
	go runGeolocationCacheCleanup(geolocationRepo, done)

	return &Services{
		Config:              cfg,
		DB:                  sqliteDB,
		RepoFactory:         repoFactory,
		NetworkService:      networkService,
		DeviceService:       deviceService,
		EventLogService:     eventLogService,
		SystemStatusService: systemStatusService,
		SettingsService:     settingsService,
		ScanManager:         scanManager,
		NicService:          nicService,
		GeolocationRepo:     geolocationRepo,
		Done:                done,
	}, nil
}

// runWeb starts the web server
func runWeb(cmd *cobra.Command, args []string) {
	signal.Ignore(syscall.SIGTERM, syscall.SIGQUIT)

	defer func() {
		if r := recover(); r != nil {
			errorLogger.Printf("FATAL PANIC in runWeb(): %v", r)
			errorLogger.Printf("Stack trace: %s", debug.Stack())
			errorLogger.Printf("RESTARTING BACKEND IN 1 SECOND...")
			time.Sleep(1 * time.Second)
			runWeb(cmd, args)
		}
	}()

	infoLogger.Printf("Starting reconYa backend (web mode) - Process ID: %d", os.Getpid())
	infoLogger.Printf("Runtime: %s/%s, Go version: %s", runtime.GOOS, runtime.GOARCH, runtime.Version())

	svc, err := initServices()
	if err != nil {
		errorLogger.Printf("Failed to initialize services: %v", err)
		errorLogger.Printf("CRITICAL ERROR - RESTARTING IN 2 SECONDS...")
		time.Sleep(2 * time.Second)
		runWeb(cmd, args)
		return
	}

	sessionSecret := "your-secret-key-here-replace-in-production"
	webHandler := web.NewWebHandler(svc.DeviceService, svc.EventLogService, svc.NetworkService, svc.SystemStatusService, svc.ScanManager, svc.GeolocationRepo, svc.SettingsService, svc.NicService, svc.Config, sessionSecret)
	router := webHandler.SetupRoutes()
	loggedRouter := middleware.LoggingMiddleware(router)

	server := &http.Server{
		Addr:    ":" + svc.Config.Port,
		Handler: loggedRouter,
	}

	infoLogger.Println("Backend initialization completed successfully")

	serverReady := make(chan bool, 1)

	go func() {
		infoLogger.Printf("Server is starting on port %s...", svc.Config.Port)

		ln, err := net.Listen("tcp", ":"+svc.Config.Port)
		if err != nil {
			infoLogger.Printf("Port %s is not available: %v", svc.Config.Port, err)
			select {
			case serverReady <- false:
			default:
			}
			return
		}
		ln.Close()

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			infoLogger.Printf("Server ListenAndServe error: %v", err)
			close(svc.Done)
			select {
			case serverReady <- false:
			default:
			}
			infoLogger.Printf("SERVER ERROR - RESTARTING IN 2 SECONDS...")
			time.Sleep(2 * time.Second)
			runWeb(cmd, args)
			return
		}
		infoLogger.Println("Server ListenAndServe has exited normally")
	}()

	go func() {
		time.Sleep(500 * time.Millisecond)
		resp, err := http.Get("http://localhost:" + svc.Config.Port + "/")
		if err == nil {
			resp.Body.Close()
			select {
			case serverReady <- true:
			default:
			}
		} else {
			infoLogger.Printf("Server health check failed: %v", err)
		}
	}()

	select {
	case ready := <-serverReady:
		if ready {
			infoLogger.Printf("reconYa backend is ready and accepting connections on port %s", svc.Config.Port)
			infoLogger.Println("Backend startup completed successfully")
			infoLogger.Printf("[INFO] Server started successfully on port %s", svc.Config.Port)
			infoLogger.Println("[READY] reconYa backend is ready to serve requests")
		} else {
			infoLogger.Println("Backend startup failed")
		}
	case <-time.After(10 * time.Second):
		infoLogger.Println("Backend startup timeout - server may still be initializing")
	}

	waitForShutdown(server, svc.Done)
}

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

var (
	scanInterface string
	scanDevices   = make(map[string]*scanDevice)
	scanMutex     sync.RWMutex
	scanRunning   bool
)

// runAgentScan starts the network scan on the specified interface
func runAgentScan(cmd *cobra.Command, args []string) {
	if scanInterface == "" {
		fmt.Println("Error: interface required. Use -i <interface>")
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

	// Set up signal handling for clean exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start the display goroutine
	stopDisplay := make(chan bool)
	go displayScanResults(scanInterface, networkCIDR, localIP, iface.HardwareAddr.String(), stopDisplay)

	// Start the scanning goroutine
	scanRunning = true
	go runContinuousScan(networkCIDR, stopDisplay)

	// Wait for interrupt
	<-sigChan
	scanRunning = false
	stopDisplay <- true
	time.Sleep(100 * time.Millisecond)

	// Clear screen and show final message
	fmt.Print("\033[2J\033[H")
	fmt.Println("\nScan stopped. Goodbye!")
}

// runContinuousScan continuously scans the network
func runContinuousScan(networkCIDR string, stop chan bool) {
	for scanRunning {
		select {
		case <-stop:
			return
		default:
			scanNetworkQuick(networkCIDR)
			// Wait between scans
			for i := 0; i < 50 && scanRunning; i++ {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

// scanNetworkQuick performs a quick ping scan
func scanNetworkQuick(networkCIDR string) {
	// Try nmap first (faster and gets MAC addresses)
	cmd := exec.Command("nmap", "-sn", "-T4", "--send-ip", "-oG", "-", networkCIDR)
	output, err := cmd.Output()
	if err != nil {
		// Fallback to simple ping sweep
		scanWithPing(networkCIDR)
		return
	}

	// Parse nmap grepable output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Host:") && strings.Contains(line, "Status: Up") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				ip := parts[1]

				// Get hostname if present
				hostname := ""
				if strings.Contains(line, "(") && strings.Contains(line, ")") {
					start := strings.Index(line, "(")
					end := strings.Index(line, ")")
					if start < end {
						hostname = line[start+1 : end]
					}
				}

				updateDevice(ip, "", "", hostname)
			}
		}
	}

	// Try to get MAC addresses with arp
	getARPInfo()
}

// scanWithPing performs a basic ping sweep
func scanWithPing(networkCIDR string) {
	_, ipNet, err := net.ParseCIDR(networkCIDR)
	if err != nil {
		return
	}

	// Generate IPs
	var ips []string
	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); incIP(ip) {
		ips = append(ips, ip.String())
	}

	// Ping each IP concurrently
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 50)

	for _, ip := range ips {
		wg.Add(1)
		go func(targetIP string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if pingHost(targetIP) {
				updateDevice(targetIP, "", "", "")
			}
		}(ip)
	}
	wg.Wait()
}

// incIP increments an IP address
func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// pingHost pings a host and returns true if online
func pingHost(ip string) bool {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "1", "-w", "500", ip)
	} else {
		cmd = exec.Command("ping", "-c", "1", "-W", "1", ip)
	}
	err := cmd.Run()
	return err == nil
}

// getARPInfo gets MAC addresses from ARP table
func getARPInfo() {
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("arp", "-an")
	} else {
		cmd = exec.Command("arp", "-n")
	}

	output, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// Parse ARP output
		// macOS format: ? (192.168.1.1) at aa:bb:cc:dd:ee:ff on en0
		// Linux format: 192.168.1.1 ether aa:bb:cc:dd:ee:ff

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		var ip, mac string
		if runtime.GOOS == "darwin" {
			// macOS format
			if strings.HasPrefix(fields[1], "(") && strings.HasSuffix(fields[1], ")") {
				ip = strings.Trim(fields[1], "()")
				if len(fields) >= 4 && fields[3] != "(incomplete)" {
					mac = fields[3]
				}
			}
		} else {
			// Linux format
			ip = fields[0]
			if len(fields) >= 3 {
				mac = fields[2]
			}
		}

		if ip != "" && mac != "" && mac != "(incomplete)" {
			scanMutex.Lock()
			if dev, ok := scanDevices[ip]; ok {
				dev.MAC = strings.ToUpper(mac)
				dev.Vendor = lookupVendor(mac)
			}
			scanMutex.Unlock()
		}
	}
}

// lookupVendor looks up the vendor from MAC address
func lookupVendor(mac string) string {
	// Simple OUI lookup for common vendors
	mac = strings.ToUpper(strings.ReplaceAll(mac, ":", ""))
	if len(mac) < 6 {
		return ""
	}
	oui := mac[:6]

	vendors := map[string]string{
		"000C29": "VMware",
		"005056": "VMware",
		"001C42": "Parallels",
		"080027": "VirtualBox",
		"00155D": "Microsoft",
		"0A0027": "VirtualBox",
		"DC4427": "Ubiquiti",
		"18E829": "Ubiquiti",
		"B4FBE4": "Ubiquiti",
		"74ACB9": "Ubiquiti",
		"788A20": "Ubiquiti",
		"D021F9": "Ubiquiti",
		"F09FC2": "Ubiquiti",
		"24A43C": "Ubiquiti",
		"802AA8": "Ubiquiti",
		"FCECDA": "Ubiquiti",
		"AC8BA9": "Apple",
		"F0D4F2": "Apple",
		"DC2B2A": "Apple",
		"A4B197": "Apple",
		"00CD08": "Apple",
		"98D6D6": "Apple",
		"F4F15A": "Apple",
		"B8E856": "Apple",
		"3CE0B7": "Samsung",
		"50A4D0": "Samsung",
		"D02544": "Samsung",
		"A00798": "Samsung",
		"2C54CF": "LG",
		"34FCEF": "LG",
		"B4E62D": "Dell",
		"A4BADB": "Dell",
		"14FEB5": "Dell",
		"B0A7B9": "Dell",
		"54BF64": "Dell",
		"5CF9DD": "Dell",
		"3497F6": "Intel",
		"A4C494": "Intel",
		"00D861": "Intel",
		"485B39": "HP",
		"1CC1DE": "HP",
		"00215A": "HP",
		"1062E5": "HP",
		"6C3BE5": "HP",
		"8CAEA7": "HP",
		"B05ADA": "Raspberry Pi",
		"DC26B5": "Raspberry Pi",
		"E45F01": "Raspberry Pi",
		"B827EB": "Raspberry Pi",
		"D83ADD": "Raspberry Pi",
	}

	if vendor, ok := vendors[oui]; ok {
		return vendor
	}
	return ""
}

// updateDevice updates or adds a device to the scan list
func updateDevice(ip, mac, vendor, hostname string) {
	scanMutex.Lock()
	defer scanMutex.Unlock()

	if dev, ok := scanDevices[ip]; ok {
		dev.LastSeen = time.Now()
		dev.Status = "ONLINE"
		if mac != "" && dev.MAC == "" {
			dev.MAC = strings.ToUpper(mac)
		}
		if vendor != "" && dev.Vendor == "" {
			dev.Vendor = vendor
		}
		if hostname != "" && dev.Hostname == "" {
			dev.Hostname = hostname
		}
	} else {
		scanDevices[ip] = &scanDevice{
			IP:       ip,
			MAC:      strings.ToUpper(mac),
			Vendor:   vendor,
			Hostname: hostname,
			Status:   "ONLINE",
			LastSeen: time.Now(),
		}
	}
}

// displayScanResults displays the scan results in airodump-ng style
func displayScanResults(ifaceName, networkCIDR, localIP, localMAC string, stop chan bool) {
	startTime := time.Now()

	for {
		select {
		case <-stop:
			return
		default:
			// Clear screen and move cursor to top
			fmt.Print("\033[2J\033[H")

			elapsed := time.Since(startTime).Round(time.Second)

			// Header
			fmt.Println()
			fmt.Printf(" reconYa Agent Scanner  [%s]  %s  Elapsed: %s\n", ifaceName, networkCIDR, elapsed)
			fmt.Println()
			fmt.Println(" ═══════════════════════════════════════════════════════════════════════════════════")
			fmt.Println()
			fmt.Printf(" Local: %s (%s)\n", localIP, localMAC)
			fmt.Println()
			fmt.Printf(" %-18s %-19s %-15s %-20s %s\n", "IP ADDRESS", "MAC ADDRESS", "VENDOR", "HOSTNAME", "STATUS")
			fmt.Println(" ─────────────────────────────────────────────────────────────────────────────────────")

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

			for _, ip := range sortedIPs {
				dev := scanDevices[ip]

				// Update status based on last seen
				status := dev.Status
				if time.Since(dev.LastSeen) > 30*time.Second {
					status = "OFFLINE"
				}

				mac := dev.MAC
				if mac == "" {
					mac = "??:??:??:??:??:??"
				}

				vendor := dev.Vendor
				if len(vendor) > 14 {
					vendor = vendor[:14]
				}

				hostname := dev.Hostname
				if len(hostname) > 19 {
					hostname = hostname[:19]
				}

				// Color the status
				statusStr := status
				if status == "ONLINE" {
					statusStr = "\033[32mONLINE\033[0m"  // Green
				} else {
					statusStr = "\033[31mOFFLINE\033[0m" // Red
				}

				fmt.Printf(" %-18s %-19s %-15s %-20s %s\n", ip, mac, vendor, hostname, statusStr)
			}
			scanMutex.RUnlock()

			fmt.Println()
			fmt.Println(" ─────────────────────────────────────────────────────────────────────────────────────")

			scanMutex.RLock()
			deviceCount := len(scanDevices)
			scanMutex.RUnlock()

			fmt.Printf(" Devices found: %d    Press Ctrl+C to stop\n", deviceCount)
			fmt.Println()

			time.Sleep(1 * time.Second)
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
	fmt.Println(" Use 'reconya web' to start the web interface and begin scanning.")
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

// rootCmd is the base command
var rootCmd = &cobra.Command{
	Use:   "reconya",
	Short: "reconYa - Network reconnaissance and monitoring tool",
	Long:  `reconYa is a network reconnaissance and monitoring tool that helps you discover and track devices on your network.`,
	Run:   runWeb, // Default to web mode when no subcommand is provided
}

// webCmd starts the web interface
var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the web interface",
	Long:  `Start the reconYa web interface for network monitoring and management.`,
	Run:   runWeb,
}

// agentCmd is the parent command for agent operations
var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent mode commands",
	Long: `Agent mode provides headless network monitoring and reconnaissance utilities.

Examples:
  reconya agent detect           # List available network interfaces
  reconya agent -i en0           # Start scanning on interface en0
  reconya agent --interface eth0 # Start scanning on interface eth0`,
	Run: runAgent,
}

// detectCmd detects network interfaces
var detectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Detect available network interfaces",
	Long:  `Detect and display all available network interfaces with their IP addresses and MAC addresses.`,
	Run:   runAgentDetect,
}

func init() {
	// Add -i/--interface flag to agent command
	agentCmd.Flags().StringVarP(&scanInterface, "interface", "i", "", "Network interface to scan (e.g., en0, eth0)")

	rootCmd.AddCommand(webCmd)
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(detectCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		errorLogger.Printf("Error: %v", err)
		os.Exit(1)
	}
}


func waitForShutdown(server *http.Server, done chan bool) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	infoLogger.Printf("Runtime info - OS: %s, Arch: %s, Go version: %s", runtime.GOOS, runtime.GOARCH, runtime.Version())
	infoLogger.Printf("Process ID: %d", os.Getpid())

	infoLogger.Println("Waiting for interrupt signal (Ctrl+C) to shutdown...")
	infoLogger.Println("Server is running and ready to accept connections...")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		select {
		case sig := <-stop:
			infoLogger.Printf("Received shutdown signal: %v", sig)

			close(done)

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()

			infoLogger.Println("Shutting down the server...")
			if err := server.Shutdown(shutdownCtx); err != nil {
				errorLogger.Printf("Server Shutdown error: %v", err)
				errorLogger.Println("Forcing shutdown...")
				os.Exit(1)
			}
			infoLogger.Println("[SUCCESS] Services stopped")
			return
		case <-ticker.C:
			infoLogger.Println("Server heartbeat: Still running...")
			select {
			case <-ctx.Done():
				infoLogger.Println("Context cancelled, shutting down...")
				return
			default:
			}
		}
	}
}
