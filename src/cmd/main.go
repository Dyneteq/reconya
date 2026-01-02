package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/spf13/cobra"

	"reconya/db"
	"reconya/internal/config"
	"reconya/models"
	"reconya/internal/device"
	"reconya/internal/eventlog"
	"reconya/internal/ipv6monitor"
	"reconya/internal/network"
	"reconya/internal/nicidentifier"
	"reconya/internal/oui"
	"reconya/internal/pingsweep"
	"reconya/internal/portscan"
	"reconya/internal/scan"
	"reconya/internal/security"
	"reconya/internal/sensor"
	"reconya/internal/settings"
	"reconya/internal/systemstatus"
)

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
	SensorService       *sensor.SensorService
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
		infoLogger.Println("Continuing without OUI service - vendor lookup will be limited")
		ouiService = nil
	} else {
		stats := ouiService.GetStatistics()
		infoLogger.Printf("OUI service initialized successfully - %v entries loaded, last updated: %v",
			stats["total_entries"], stats["last_updated"])
	}

	sensorRepo := repoFactory.NewSensorRepository()

	networkService := network.NewNetworkService(networkRepo, cfg, dbManager)
	deviceService := device.NewDeviceService(deviceRepo, networkService, cfg, dbManager, ouiService)
	eventLogService := eventlog.NewEventLogService(eventLogRepo, deviceService, dbManager)

	// Initialize ARP history tracking and MITM detection
	arpHistoryRepo := repoFactory.NewARPHistoryRepository()
	mitmDetector := security.NewMITMDetector(arpHistoryRepo, eventLogService)
	deviceService.SetARPHistoryComponents(arpHistoryRepo, mitmDetector)
	systemStatusService := systemstatus.NewSystemStatusService(systemStatusRepo, geolocationRepo)
	settingsService := settings.NewSettingsService(settingsRepo)
	sensorService := sensor.NewSensorService(sensorRepo)
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
		SensorService:       sensorService,
		ScanManager:         scanManager,
		NicService:          nicService,
		GeolocationRepo:     geolocationRepo,
		Done:                done,
	}, nil
}

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

// runSuite starts both the web server and agent in parallel
func runSuite(cmd *cobra.Command, args []string) {
	fmt.Println()
	fmt.Println(" reconYa Suite - Starting Web Server + Agent")
	fmt.Println(" ════════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Detect primary interface for agent
	primaryIface := detectPrimaryInterface()
	if primaryIface == "" {
		fmt.Println(" Warning: Could not detect primary network interface for agent.")
		fmt.Println(" Starting web server only. Use 'reconya agent -i <interface>' separately.")
		fmt.Println()
		runWeb(cmd, args)
		return
	}

	fmt.Printf(" Primary interface: %s\n", primaryIface)
	fmt.Println()

	// Initialize services once for both web and agent
	fmt.Println(" [INIT] Initializing services...")
	svc, err := initServices()
	if err != nil {
		errorLogger.Printf("Failed to initialize services: %v", err)
		os.Exit(1)
	}

	// Store services for agent use
	agentServices = svc

	// Find network for agent scanning
	iface, err := net.InterfaceByName(primaryIface)
	if err != nil {
		errorLogger.Printf("Error: interface '%s' not found: %v", primaryIface, err)
		os.Exit(1)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		errorLogger.Printf("Error getting addresses: %v", err)
		os.Exit(1)
	}

	var networkCIDR string
	for _, addr := range addrs {
		ip, ipNet, err := net.ParseCIDR(addr.String())
		if err != nil || ip.To4() == nil || ip.IsLoopback() {
			continue
		}
		ones, _ := ipNet.Mask.Size()
		networkIP := ip.Mask(ipNet.Mask)
		networkCIDR = fmt.Sprintf("%s/%d", networkIP.String(), ones)
		break
	}

	if networkCIDR == "" {
		errorLogger.Printf("Error: no IPv4 address found on interface '%s'", primaryIface)
		os.Exit(1)
	}

	// Find or create the network
	network, err := svc.NetworkService.FindOrCreate(networkCIDR)
	if err != nil {
		errorLogger.Printf("Error finding/creating network: %v", err)
		os.Exit(1)
	}

	// Create or update local sensor
	localSensor := createLocalSensor(svc, primaryIface, networkCIDR, iface.HardwareAddr.String())
	if localSensor != nil {
		fmt.Printf(" [SENSOR] Local agent registered: %s\n", localSensor.ID[:8])
	}

	fmt.Printf(" [AGENT] Scanning network: %s\n", networkCIDR)
	fmt.Printf(" [WEB] Starting web server on port %s...\n", svc.Config.Port)
	fmt.Println()

	// Start agent scanning in background (headless - no TUI)
	go runSuiteAgentLoop(network.ID, svc, localSensor)

	// Set shared services for web server to use
	suiteServices = svc

	// Run web server in foreground (this handles its own signal handling)
	runWeb(cmd, args)
}

// createLocalSensor creates or updates the local agent sensor
func createLocalSensor(svc *Services, ifaceName, networkCIDR, mac string) *models.Sensor {
	ctx := context.Background()

	// Check if local sensor already exists
	sensors, err := svc.SensorService.GetAllSensors(ctx)
	if err != nil {
		errorLogger.Printf("Error getting sensors: %v", err)
		return nil
	}

	var localSensor *models.Sensor
	for _, s := range sensors {
		if s.Name == "Local Agent" {
			localSensor = s
			break
		}
	}

	// Create if not exists
	if localSensor == nil {
		localSensor, err = svc.SensorService.CreateSensor(ctx, "Local Agent")
		if err != nil {
			errorLogger.Printf("Error creating local sensor: %v", err)
			return nil
		}
	}

	// Get hostname
	hostname, _ := os.Hostname()

	// Get local IP from interface
	var localIP string
	if iface, err := net.InterfaceByName(ifaceName); err == nil {
		if addrs, err := iface.Addrs(); err == nil {
			for _, addr := range addrs {
				if ip, _, err := net.ParseCIDR(addr.String()); err == nil && ip.To4() != nil {
					localIP = ip.String()
					break
				}
			}
		}
	}

	// Register/update the sensor
	req := sensor.RegisterRequest{
		Hostname:    hostname,
		IP:          localIP,
		MAC:         mac,
		Interface:   ifaceName,
		NetworkCIDR: networkCIDR,
	}
	localSensor, err = svc.SensorService.Register(ctx, localSensor.Token, req)
	if err != nil {
		errorLogger.Printf("Error registering local sensor: %v", err)
		return nil
	}

	return localSensor
}

// runSuiteAgentLoop runs agent scanning without TUI for suite mode
func runSuiteAgentLoop(networkID string, svc *Services, localSensor *models.Sensor) {
	scanCount := 0
	ctx := context.Background()

	network, err := svc.NetworkService.FindByID(networkID)
	if err != nil || network == nil {
		errorLogger.Printf("Error: network not found")
		return
	}

	// Start with scan status as idle - wait for user to trigger scan
	if localSensor != nil {
		svc.SensorService.UpdateScanStatus(ctx, localSensor.ID, models.SensorScanStatusIdle)
	}

	infoLogger.Printf("[AGENT] Ready and waiting for scan command...")

	for {
		// Check if we should be scanning
		shouldScan := false
		if localSensor != nil {
			currentSensor, err := svc.SensorService.GetSensorByID(ctx, localSensor.ID)
			if err == nil && currentSensor != nil {
				shouldScan = currentSensor.ScanStatus == models.SensorScanStatusRunning
			}
			// Update last seen
			svc.SensorService.Register(ctx, localSensor.Token, sensor.RegisterRequest{})
		}

		if !shouldScan {
			// Not scanning - wait and check again
			time.Sleep(2 * time.Second)
			continue
		}

		scanCount++
		infoLogger.Printf("[AGENT] Starting scan #%d on %s", scanCount, network.CIDR)

		// Use existing PingSweepService
		devices, err := svc.ScanManager.GetPingSweepService().ExecuteSweepScanCommand(network.CIDR)
		if err != nil {
			infoLogger.Printf("[AGENT] Scan error: %v", err)
		} else {
			infoLogger.Printf("[AGENT] Found %d devices", len(devices))

			// Process each device
			for _, device := range devices {
				device.NetworkID = network.ID
				savedDevice, err := svc.DeviceService.CreateOrUpdate(&device)
				if err != nil {
					continue
				}

				// Trigger port scan if eligible
				if svc.DeviceService.EligibleForPortScan(savedDevice) {
					go svc.ScanManager.GetPortScanService().Run(*savedDevice)
				}
			}
		}

		infoLogger.Printf("[AGENT] Scan #%d complete", scanCount)

		// Wait 30 seconds between scans
		time.Sleep(30 * time.Second)
	}
}

// Command definitions
var rootCmd = &cobra.Command{
	Use:   "reconya",
	Short: "reconYa - Network reconnaissance and monitoring tool",
	Long:  `reconYa is a network reconnaissance and monitoring tool that helps you discover and track devices on your network.`,
	Run:   runSuite, // Default to suite mode (web + agent)
}

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the web interface",
	Long:  `Start the reconYa web interface for network monitoring and management.`,
	Run:   runWeb,
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent mode commands",
	Long: `Agent mode provides headless network monitoring and reconnaissance utilities.

Examples:
  reconya agent detect           # List available network interfaces
  reconya agent primary          # Start scanning on primary interface (auto-detect)
  reconya agent -i en0           # Start scanning on interface en0
  reconya agent --interface eth0 # Start scanning on interface eth0`,
	Run: runAgent,
}

var detectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Detect available network interfaces",
	Long:  `Detect and display all available network interfaces with their IP addresses and MAC addresses.`,
	Run:   runAgentDetect,
}

var primaryCmd = &cobra.Command{
	Use:   "primary",
	Short: "Start scanning on primary interface",
	Long:  `Automatically detect the primary network interface and start scanning.`,
	Run:   runAgentPrimary,
}

var suiteCmd = &cobra.Command{
	Use:   "suite",
	Short: "Start both web server and agent",
	Long:  `Start the reconYa suite which includes both the web server and the network agent running in parallel.`,
	Run:   runSuite,
}

func init() {
	// Disable auto-generated completion command
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Add -i/--interface flag to agent command
	agentCmd.Flags().StringVarP(&scanInterface, "interface", "i", "", "Network interface to scan (e.g., en0, eth0)")
	agentCmd.Flags().StringVar(&serverURL, "server", "", "Web server URL to register with (e.g., http://localhost:3000)")
	agentCmd.Flags().StringVar(&sensorToken, "token", "", "Sensor token for registration")

	// Add same flags to primary command
	primaryCmd.Flags().StringVar(&serverURL, "server", "", "Web server URL to register with (e.g., http://localhost:3000)")
	primaryCmd.Flags().StringVar(&sensorToken, "token", "", "Sensor token for registration")

	rootCmd.AddCommand(webCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(suiteCmd)
	agentCmd.AddCommand(detectCmd)
	agentCmd.AddCommand(primaryCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		errorLogger.Printf("Error: %v", err)
		os.Exit(1)
	}
}
