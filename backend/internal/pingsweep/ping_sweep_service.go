package pingsweep

import (
	"fmt"
	"log"
	"os/exec"
  "math/rand"
	"reconya-ai/internal/config"
	"reconya-ai/internal/device"
	"reconya-ai/internal/eventlog"
	"reconya-ai/internal/network"
	"reconya-ai/internal/portscan"
	"reconya-ai/internal/util"
	"reconya-ai/models"
	"strings"
	"time"
)

type PingSweepService struct {
	Config          *config.Config
	DeviceService   *device.DeviceService
	EventLogService *eventlog.EventLogService
	NetworkService  *network.NetworkService
	PortScanService *portscan.PortScanService
}

func NewPingSweepService(
	cfg *config.Config,
	deviceService *device.DeviceService,
	eventLogService *eventlog.EventLogService,
	networkService *network.NetworkService,
	portScanService *portscan.PortScanService) *PingSweepService {
	return &PingSweepService{
		Config:          cfg,
		DeviceService:   deviceService,
		EventLogService: eventLogService,
		NetworkService:  networkService,
		PortScanService: portScanService,
	}
}
func (s *PingSweepService) Run() {
	log.Println("Starting new ping sweep scan...")

	// Use retry logic for creating initial ping sweep event log
	err := util.RetryOnLock(func() error {
		return s.EventLogService.CreateOne(&models.EventLog{
			Type: models.PingSweep,
		})
	})
	
	if err != nil {
		log.Printf("Error creating ping sweep event log: %v", err)
	}
	devices, err := s.ExecuteSweepScanCommand(s.Config.NetworkCIDR)
	if err != nil {
		log.Printf("Error executing sweep scan: %v\n", err)
		return
	}

	// Create a list of devices that need port scanning
	var devicesToPingScan []models.Device

	// First update all devices - ONE AT A TIME
	for _, device := range devices {
		// Use retry logic for updating device
		updatedDevice, err := util.RetryOnLockWithResult(func() (*models.Device, error) {
			return s.DeviceService.CreateOrUpdate(&device)
		})
		
		if err != nil {
			log.Printf("Error updating device %s after retries: %v", device.IPv4, err)
			continue
		}

		deviceIDStr := device.ID
		// Use retry logic for creating event log
		eventLogErr := util.RetryOnLock(func() error {
			return s.EventLogService.CreateOne(&models.EventLog{
				Type:     models.DeviceOnline,
				DeviceID: &deviceIDStr,
			})
		})
		
		if eventLogErr != nil {
			log.Printf("Error creating device online event log: %v", eventLogErr)
		}

		// Add to the list if eligible for port scan - but limit to max 3 devices per scan
		if s.DeviceService.EligibleForPortScan(updatedDevice) {
			devicesToPingScan = append(devicesToPingScan, *updatedDevice)
		}
	}
  dest := make([]models.Device, len(devicesToPingScan))
  perm := rand.Perm(len(devicesToPingScan))
  for i, v := range perm {
    dest[v] = devicesToPingScan[i]
  }

	// Run port scans ONE AT A TIME, not concurrently
	if len(dest) > 0 {
		log.Printf("Running port scans for 3 of %d devices, one at a time", len(dest))
    for _, deviceToScan := range dest[:3] {
			// Run the port scan synchronously
			s.PortScanService.Run(deviceToScan)
			
			// Add a small delay between port scans to reduce database contention
			time.Sleep(500 * time.Millisecond)
		}
	}

	log.Printf("Ping sweep scan completed. Found %d devices.", len(devices))
	log.Printf("Ping sweep scan completed. Found %d devices.", len(devices))
}

func (s *PingSweepService) ExecuteSweepScanCommand(network string) ([]models.Device, error) {
	log.Printf("Executing nmap command on network: %s", network)
	// Enhanced scan to gather MAC addresses, hostnames, and vendor info
	// -sn: ping scan, --send-ip: use IP packets, -T4: faster timing, -oX: XML output
	// -R: resolve hostnames, --dns-servers: use specific DNS servers for resolution
	// --system-dns: use system DNS resolution
	// sudo required on macOS for proper MAC address and vendor detection
	cmd := exec.Command("/usr/bin/sudo", "nmap", "-sn", "--send-ip", "-T4", "-R", "--system-dns", "-oX", "-", network)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("nmap command failed: %s\n", string(output))
		return nil, fmt.Errorf("error executing nmap: %w", err)
	}

	log.Printf("nmap command succeeded. Output length: %d bytes", len(output))

	devices := s.DeviceService.ParseFromNmapXML(string(output))
	
	// If we didn't get hostnames from nmap, try to enhance with additional methods
	for i, device := range devices {
		if device.Hostname == nil || *device.Hostname == "" {
			// Try to get hostname using additional methods
			if hostname := s.tryGetHostname(device.IPv4); hostname != "" {
				devices[i].Hostname = &hostname
				log.Printf("Enhanced hostname detection found: %s for IP: %s", hostname, device.IPv4)
			}
		}
	}
	
	return devices, nil
}

// tryGetHostname attempts to get hostname using additional methods
func (s *PingSweepService) tryGetHostname(ip string) string {
	// Try DNS reverse lookup with timeout
	if hostname := s.tryDNSReverseLookup(ip); hostname != "" {
		return hostname
	}
	
	// Try alternative DNS lookup methods
	if hostname := s.tryDigReverseLookup(ip); hostname != "" {
		return hostname
	}
	
	return ""
}

// tryDNSReverseLookup attempts reverse DNS lookup with timeout
func (s *PingSweepService) tryDNSReverseLookup(ip string) string {
	// Try using nslookup with a short timeout
	cmd := exec.Command("timeout", "2", "nslookup", ip)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	
	// Parse hostname from nslookup output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "name =") {
			parts := strings.Split(line, "name =")
			if len(parts) > 1 {
				hostname := strings.TrimSpace(parts[1])
				hostname = strings.TrimSuffix(hostname, ".") // Remove trailing dot
				if hostname != "" && !strings.Contains(hostname, "NXDOMAIN") {
					return hostname
				}
			}
		}
	}
	
	return ""
}

// tryDigReverseLookup attempts reverse DNS lookup using dig
func (s *PingSweepService) tryDigReverseLookup(ip string) string {
	// Try using dig with a short timeout for reverse lookup
	cmd := exec.Command("timeout", "2", "dig", "+short", "-x", ip)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	
	hostname := strings.TrimSpace(string(output))
	hostname = strings.TrimSuffix(hostname, ".") // Remove trailing dot
	
	if hostname != "" && !strings.Contains(hostname, "NXDOMAIN") && hostname != ip {
		return hostname
	}
	
	return ""
}
