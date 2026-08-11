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
	"time"
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

// ExecuteSweepScanForRanges sweeps each active range of a network in turn,
// merging the results into one device list keyed by IP. Ranges are scanned
// sequentially rather than concurrently: the native scanner's ARP cache is a
// single package-global snapshot that gets reset at the start of every
// ScanNetwork call, so parallel range scans would race and corrupt each
// other's ARP resolution.
func (s *PingSweepService) ExecuteSweepScanForRanges(ranges []models.NetworkRange) ([]models.Device, error) {
	var perRange [][]models.Device

	for _, r := range ranges {
		devices, err := s.executeWithFallback(r.CIDR)
		if err != nil {
			log.Printf("Range scan failed for %s: %v", r.CIDR, err)
			continue
		}
		perRange = append(perRange, devices)

		if s.NetworkService != nil && r.ID != "" {
			if err := s.NetworkService.MarkRangeScanned(r.ID, time.Now()); err != nil {
				log.Printf("Failed to record last_scanned_at for range %s: %v", r.CIDR, err)
			}
		}
	}

	result := mergeDevicesByIP(perRange...)

	log.Printf("Multi-range scan succeeded across %d range(s). Found %d unique devices", len(ranges), len(result))

	return result, nil
}

// mergeDevicesByIP flattens multiple ranges' scan results into one list,
// keyed by IP so a host visible from more than one range (overlapping
// ranges, or a broadcast/gateway address shared across subnets) is only
// reported once. Later batches win on conflict.
func mergeDevicesByIP(batches ...[]models.Device) []models.Device {
	merged := make(map[string]models.Device)
	for _, batch := range batches {
		for _, d := range batch {
			merged[d.IPv4] = d
		}
	}

	result := make([]models.Device, 0, len(merged))
	for _, d := range merged {
		result = append(result, d)
	}
	return result
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
	if s.Config != nil && s.Config.VendorLookupEnabled {
		nativeScanner.EnableOnlineVendorLookup()
	}
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
