package pingsweep

import (
	"fmt"
	"log"
	"reconya/internal/config"
	"reconya/internal/device"
	"reconya/internal/eventlog"
	"reconya/internal/network"
	"reconya/internal/portscan"
	"reconya/internal/scanner"
	"reconya/models"
	"sync"
)

type PingSweepService struct {
	Config          *config.Config
	DeviceService   *device.DeviceService
	EventLogService *eventlog.EventLogService
	NetworkService  *network.NetworkService
	PortScanService *portscan.PortScanService
	portScanQueue   chan models.Device
	portScanWorkers sync.WaitGroup
}

func NewPingSweepService(
	cfg *config.Config,
	deviceService *device.DeviceService,
	eventLogService *eventlog.EventLogService,
	networkService *network.NetworkService,
	portScanService *portscan.PortScanService) *PingSweepService {

	service := &PingSweepService{
		Config:          cfg,
		DeviceService:   deviceService,
		EventLogService: eventLogService,
		NetworkService:  networkService,
		PortScanService: portScanService,
		portScanQueue:   make(chan models.Device, 100), // Buffer for 100 devices
	}

	// Start 3 port scan workers
	service.startPortScanWorkers(3)

	return service
}
// Run method is deprecated - use the scan manager to control scanning
// This method is kept for compatibility but should not be called directly
func (s *PingSweepService) Run() {
	log.Println("PingSweepService.Run() is deprecated - scanning is now controlled by scan manager")
}

func (s *PingSweepService) ExecuteSweepScanCommand(network string) ([]models.Device, error) {
	log.Printf("Executing network scan on: %s", network)

	devices, err := s.executeWithFallback(network)
	if err != nil {
		return nil, err
	}

	log.Printf("Network scan succeeded. Found %d devices", len(devices))

	return devices, nil
}

// executeWithFallback performs network scan using native Go scanner
func (s *PingSweepService) executeWithFallback(network string) ([]models.Device, error) {
	// Use native Go scanner (no external dependencies, no privileges required)
	devices, err := s.tryNativeScanner(network)
	if err != nil {
		return nil, fmt.Errorf("network scan failed for %s: %v", network, err)
	}

	return devices, nil
}

// tryNativeScanner uses the native Go scanner for network discovery
func (s *PingSweepService) tryNativeScanner(network string) ([]models.Device, error) {
	log.Printf("Trying native Go scanner on network: %s", network)

	nativeScanner := scanner.NewNativeScanner()
	devices, err := nativeScanner.ScanNetwork(network)
	if err != nil {
		return nil, err
	}

	return devices, nil
}

// startPortScanWorkers starts background workers for port scanning
func (s *PingSweepService) startPortScanWorkers(numWorkers int) {
	log.Printf("Starting %d port scan workers", numWorkers)

	for i := 0; i < numWorkers; i++ {
		s.portScanWorkers.Add(1)
		go s.portScanWorker(i)
	}
}

// portScanWorker continuously processes devices from the port scan queue
func (s *PingSweepService) portScanWorker(workerID int) {
	defer s.portScanWorkers.Done()

	log.Printf("Port scan worker %d started", workerID)

	for device := range s.portScanQueue {
		log.Printf("Worker %d: Starting port scan for device %s", workerID, device.IPv4)
		s.PortScanService.Run(device)
		log.Printf("Worker %d: Completed port scan for device %s", workerID, device.IPv4)
	}

	log.Printf("Port scan worker %d stopped", workerID)
}
