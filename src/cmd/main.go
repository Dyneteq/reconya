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
)

var (
	infoLogger  = log.New(os.Stdout, "", log.LstdFlags)
	errorLogger = log.New(os.Stderr, "", log.LstdFlags)
)

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

	networkService := network.NewNetworkService(networkRepo, cfg, dbManager)
	deviceService := device.NewDeviceService(deviceRepo, networkService, cfg, dbManager, ouiService)
	eventLogService := eventlog.NewEventLogService(eventLogRepo, deviceService, dbManager)
	systemStatusService := systemstatus.NewSystemStatusService(systemStatusRepo, geolocationRepo)
	settingsService := settings.NewSettingsService(settingsRepo)
	portScanService := portscan.NewPortScanService(deviceService, eventLogService)
	pingSweepService := pingsweep.NewPingSweepService(cfg, deviceService, eventLogService, networkService, portScanService)
	ipv6MonitorService := ipv6monitor.NewIPv6MonitorService(deviceService, networkService, infoLogger)
	nicService := nicidentifier.NewNicIdentifierService(networkService, systemStatusService, eventLogService, deviceService, cfg)

	scanManager := scan.NewScanManager(pingSweepService, networkService, ipv6MonitorService)

	done := make(chan bool)

	nicService.Identify()

	go runDeviceUpdater(deviceService, done)
	go runNetworkDetection(nicService, done)
	go runGeolocationCacheCleanup(geolocationRepo, done)
	go runPublicIPRefresh(nicService, done)

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

func runPublicIPRefresh(nicService *nicidentifier.NicIdentifierService, done <-chan bool) {
	ticker := time.NewTicker(config.PublicIPRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			nicService.RefreshPublicIPGeolocation()
			updateAgentSystemStatusCache()
		}
	}
}

func runDeviceUpdater(service *device.DeviceService, done <-chan bool) {
	defer func() {
		if r := recover(); r != nil {
			errorLogger.Printf("Device updater panic recovered: %v", r)
			errorLogger.Printf("Device updater stack trace: %s", debug.Stack())
		}
		infoLogger.Println("Device updater service stopped")
	}()

	ticker := time.NewTicker(config.DeviceUpdaterInterval)
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
					time.Sleep(config.DeviceUpdateRetryDelay)
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

	ticker := time.NewTicker(config.GeolocationCleanupInterval)
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

	ticker := time.NewTicker(config.NetworkDetectionInterval)
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

func runSuite(cmd *cobra.Command, args []string) {
	fmt.Println()
	fmt.Println(" reconYa Suite - Starting Web Server + Agent")
	fmt.Println(" ════════════════════════════════════════════════════════════════════")
	fmt.Println()

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

	fmt.Println(" [INIT] Initializing services...")
	svc, err := initServices()
	if err != nil {
		errorLogger.Printf("Failed to initialize services: %v", err)
		os.Exit(1)
	}

	agentServices = svc

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
		errorLogger.Printf("Error: no IPv4 address found on interface '%s'", primaryIface)
		os.Exit(1)
	}

	network, err := svc.NetworkService.FindOrCreate(networkCIDR)
	if err != nil {
		errorLogger.Printf("Error finding/creating network: %v", err)
		os.Exit(1)
	}

	fmt.Printf(" [WEB] Starting web server on port %s...\n", svc.Config.Port)
	fmt.Println()

	suiteServices = svc

	localMAC := iface.HardwareAddr.String()
	startAgentWithServices(svc, primaryIface, network.ID, networkCIDR, localIP, localMAC)

	go runWebWithServices(svc)

	<-svc.Done
	fmt.Print("\033[2J\033[H")
	fmt.Println("\nScan stopped. Goodbye!")
}

var rootCmd = &cobra.Command{
	Use:   "reconya",
	Short: "reconYa - Network reconnaissance and monitoring tool",
	Long:  `reconYa is a network reconnaissance and monitoring tool that helps you discover and track devices on your network.`,
	Run:   runSuite,
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent mode commands",
	Long:  `Agent mode provides headless network monitoring and reconnaissance utilities.`,
	Run:   runAgent,
}

var suiteCmd = &cobra.Command{
	Use:   "suite",
	Short: "Start both web server and agent",
	Long:  `Start the reconYa suite which includes both the web server and the network agent running in parallel.`,
	Run:   runSuite,
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	agentCmd.Flags().StringVarP(&scanInterface, "interface", "i", "", "Network interface to scan (e.g., en0, eth0)")

	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(suiteCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		errorLogger.Printf("Error: %v", err)
		os.Exit(1)
	}
}
