package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

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

func main() {
	signal.Ignore(syscall.SIGTERM, syscall.SIGQUIT)

	defer func() {
		if r := recover(); r != nil {
			errorLogger.Printf("FATAL PANIC in main(): %v", r)
			errorLogger.Printf("Stack trace: %s", debug.Stack())
			errorLogger.Printf("RESTARTING BACKEND IN 1 SECOND...")
			time.Sleep(1 * time.Second)
			main()
		}
	}()

	infoLogger.Printf("Starting reconYa backend - Process ID: %d", os.Getpid())
	infoLogger.Printf("Runtime: %s/%s, Go version: %s", runtime.GOOS, runtime.GOARCH, runtime.Version())
	infoLogger.Printf("🛡️ Backend is protected against external termination")

	cfg, err := config.LoadConfig()
	if err != nil {
		infoLogger.Printf("Failed to load configuration: %v", err)
		infoLogger.Printf("CRITICAL ERROR - RESTARTING IN 2 SECONDS...")
		time.Sleep(2 * time.Second)
		main()
		return
	}

	var repoFactory *db.RepositoryFactory
	var sqliteDB *sql.DB

	infoLogger.Println("Using SQLite database")
	sqliteDB, err = db.ConnectToSQLite(cfg.SQLitePath)
	if err != nil {
		infoLogger.Printf("Failed to connect to SQLite: %v", err)
		infoLogger.Printf("DATABASE ERROR - RESTARTING IN 3 SECONDS...")
		time.Sleep(3 * time.Second)
		main()
		return
	}

	if err := db.InitializeSchema(sqliteDB); err != nil {
		infoLogger.Printf("Failed to initialize database schema: %v", err)
		infoLogger.Printf("SCHEMA ERROR - RESTARTING IN 3 SECONDS...")
		time.Sleep(3 * time.Second)
		main()
		return
	}

	infoLogger.Println("Resetting port scan cooldowns for development...")
	if err := db.ResetPortScanCooldowns(sqliteDB); err != nil {
		infoLogger.Printf("Warning: Failed to reset port scan cooldowns: %v", err)
	}

	repoFactory = db.NewRepositoryFactory(sqliteDB, cfg.DatabaseName)

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

	sessionSecret := "your-secret-key-here-replace-in-production"
	webHandler := web.NewWebHandler(deviceService, eventLogService, networkService, systemStatusService, scanManager, geolocationRepo, settingsService, nicService, cfg, sessionSecret)
	router := webHandler.SetupRoutes()
	loggedRouter := middleware.LoggingMiddleware(router)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: loggedRouter,
	}

	infoLogger.Println("Backend initialization completed successfully")

	serverReady := make(chan bool, 1)

	go func() {
		infoLogger.Printf("Server is starting on port %s...", cfg.Port)

		ln, err := net.Listen("tcp", ":"+cfg.Port)
		if err != nil {
			infoLogger.Printf("Port %s is not available: %v", cfg.Port, err)
			select {
			case serverReady <- false:
			default:
			}
			return
		}
		ln.Close()

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			infoLogger.Printf("Server ListenAndServe error: %v", err)
			close(done)
			select {
			case serverReady <- false:
			default:
			}
			infoLogger.Printf("SERVER ERROR - RESTARTING IN 2 SECONDS...")
			time.Sleep(2 * time.Second)
			main()
			return
		}
		infoLogger.Println("Server ListenAndServe has exited normally")
	}()

	go func() {
		time.Sleep(500 * time.Millisecond)
		resp, err := http.Get("http://localhost:" + cfg.Port + "/")
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
			infoLogger.Printf("✅ reconYa backend is ready and accepting connections on port %s", cfg.Port)
			infoLogger.Println("🚀 Backend startup completed successfully")
			infoLogger.Printf("[INFO] Server started successfully on port %s", cfg.Port)
			infoLogger.Println("[READY] reconYa backend is ready to serve requests")
		} else {
			infoLogger.Println("❌ Backend startup failed")
		}
	case <-time.After(10 * time.Second):
		infoLogger.Println("⚠️ Backend startup timeout - server may still be initializing")
	}

	waitForShutdown(server, done)
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
