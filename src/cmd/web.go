package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"reconya/internal/web/handlers"
	"reconya/middleware"
)

// Shared services for suite mode
var suiteServices *Services

// runWeb starts the web server (initializes its own services)
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

	// Use pre-initialized services if available (suite mode), otherwise init
	var svc *Services
	var err error
	if suiteServices != nil {
		svc = suiteServices
		infoLogger.Println("Using pre-initialized services (suite mode)")
	} else {
		svc, err = initServices()
		if err != nil {
			errorLogger.Printf("Failed to initialize services: %v", err)
			errorLogger.Printf("CRITICAL ERROR - RESTARTING IN 2 SECONDS...")
			time.Sleep(2 * time.Second)
			runWeb(cmd, args)
			return
		}
	}

	runWebWithServices(svc)
}

// runWebWithServices starts the web server with provided services
func runWebWithServices(svc *Services) {
	sessionSecret := "your-secret-key-here-replace-in-production"
	webHandler := handlers.NewWebHandler(svc.DeviceService, svc.EventLogService, svc.NetworkService, svc.SystemStatusService, svc.ScanManager, svc.GeolocationRepo, svc.SettingsService, svc.NicService, svc.Config, sessionSecret)
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
