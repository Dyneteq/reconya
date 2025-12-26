package pingsweep

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"reconya/internal/config"
	"reconya/internal/device"
	"reconya/internal/eventlog"
	"reconya/internal/network"
	"reconya/internal/portscan"
	"reconya/internal/scanner"
	"reconya/models"
	"strings"
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
	log.Printf("Executing nmap command on network: %s", network)
	
	// Try multiple scan strategies for different environments
	devices, err := s.executeWithFallback(network)
	if err != nil {
		return nil, err
	}

	log.Printf("nmap command succeeded. Found %d devices", len(devices))

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

// executeWithFallback tries different scan strategies based on environment
func (s *PingSweepService) executeWithFallback(network string) ([]models.Device, error) {
	// Skip native scanner - it's too slow for large networks
	log.Printf("Skipping native Go scanner, using nmap directly")

	// Strategy 1: Try sudo with IP packets (works on most systems, gets MAC/vendor)
	devices, err := s.tryNmapCommand([]string{"sudo", "nmap", "-sn", "--send-ip", "-T4", "-n", "-oX", "-", network})
	if err == nil && len(devices) > 0 {
		log.Printf("Sudo IP scan successful, found %d devices", len(devices))
		return devices, nil
	}
	log.Printf("Sudo IP scan failed or found no devices: %v", err)

	// Strategy 3: Try IP packets without sudo (may still get some MAC info)
	devices, err = s.tryNmapCommand([]string{"nmap", "-sn", "--send-ip", "-T4", "-oX", "-", network})
	if err == nil && len(devices) > 0 {
		log.Printf("IP scan without sudo successful, found %d devices", len(devices))
		return devices, nil
	}
	log.Printf("IP scan without sudo failed or found no devices: %v", err)

	// Strategy 4: Try ARP scan with sudo (best for local networks but needs interface access)
	devices, err = s.tryNmapCommand([]string{"sudo", "nmap", "-sn", "-PR", "-T4", "-n", "-oX", "-", network})
	if err == nil && len(devices) > 0 {
		log.Printf("Sudo ARP scan successful, found %d devices", len(devices))
		return devices, nil
	}
	log.Printf("Sudo ARP scan failed or found no devices: %v", err)

	// Strategy 5: Try ARP scan without sudo
	devices, err = s.tryNmapCommand([]string{"nmap", "-sn", "-PR", "-T4", "-oX", "-", network})
	if err == nil && len(devices) > 0 {
		log.Printf("ARP scan without sudo successful, found %d devices", len(devices))
		return devices, nil
	}
	log.Printf("ARP scan without sudo failed or found no devices: %v", err)

	// Strategy 6: Last resort - TCP SYN scan on common ports (minimal info but finds hosts)
	devices, err = s.tryNmapCommand([]string{"nmap", "-sn", "-PS80,443,22,21,23,25,53,110,111,135,139,143,993,995", "-T4", "-oX", "-", network})
	if err == nil && len(devices) > 0 {
		log.Printf("TCP SYN probe scan successful, found %d devices", len(devices))
		return devices, nil
	}
	log.Printf("TCP SYN probe scan failed or found no devices: %v", err)

	return nil, fmt.Errorf("all scan strategies failed for network %s", network)
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

// tryNmapCommand executes a specific nmap command with automatic retry on timeout
func (s *PingSweepService) tryNmapCommand(args []string) ([]models.Device, error) {
	log.Printf("Trying nmap command: %s", strings.Join(args, " "))
	
	// First attempt with 20-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	output, err := cmd.CombinedOutput()
	
	// If timeout occurred and command doesn't already have -n flag, retry with -n
	if err != nil && ctx.Err() == context.DeadlineExceeded {
		log.Printf("nmap command timed out, checking if we can retry with -n flag")
		
		// Check if -n flag is already present
		hasNoResolve := false
		for _, arg := range args {
			if arg == "-n" {
				hasNoResolve = true
				break
			}
		}
		
		// If no -n flag present, retry with -n to skip DNS resolution
		if !hasNoResolve {
			log.Printf("Retrying nmap command with -n flag to skip DNS resolution")
			
			// Build new args with -n flag after the command name
			retryArgs := []string{args[0]} // command (nmap or sudo)
			if args[0] == "sudo" && len(args) > 1 {
				retryArgs = append(retryArgs, args[1]) // nmap
				retryArgs = append(retryArgs, "-n")    // add -n flag
				retryArgs = append(retryArgs, args[2:]...) // rest of args
			} else {
				retryArgs = append(retryArgs, "-n")    // add -n flag
				retryArgs = append(retryArgs, args[1:]...) // rest of args
			}
			
			log.Printf("Retry command: %s", strings.Join(retryArgs, " "))
			
			// Retry with 90-second timeout for Raspberry Pi compatibility
			retryCtx, retryCancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer retryCancel()
			
			retryCmd := exec.CommandContext(retryCtx, retryArgs[0], retryArgs[1:]...)
			output, err = retryCmd.CombinedOutput()
			
			if err != nil {
				if retryCtx.Err() == context.DeadlineExceeded {
					log.Printf("nmap retry also timed out after 90 seconds")
					return nil, fmt.Errorf("nmap command timed out even with -n flag")
				}
				log.Printf("nmap retry command failed: %v, output: %s", err, string(output))
				return nil, err
			}
		} else {
			log.Printf("nmap command already has -n flag and still timed out")
			return nil, fmt.Errorf("nmap command timed out after 20 seconds")
		}
	} else if err != nil {
		log.Printf("nmap command failed: %v, output: %s", err, string(output))
		return nil, err
	}

	if len(output) == 0 {
		return nil, fmt.Errorf("nmap returned empty output")
	}

	log.Printf("nmap command output length: %d bytes", len(output))
	
	devices := s.DeviceService.ParseFromNmapXML(string(output))
	return devices, nil
}

// tryGetHostname attempts to get hostname using additional methods
func (s *PingSweepService) tryGetHostname(ip string) string {
	// Try a dedicated nmap hostname scan first (most reliable)
	if hostname := s.tryNmapHostnameScan(ip); hostname != "" {
		return hostname
	}
	
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

// tryNmapHostnameScan does a quick nmap scan focused on hostname resolution
func (s *PingSweepService) tryNmapHostnameScan(ip string) string {
	// Quick hostname-focused scan
	cmd := exec.Command("nmap", "-sn", "-R", "--system-dns", "-oX", "-", ip)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	
	// Parse the XML output for hostname
	devices := s.DeviceService.ParseFromNmapXML(string(output))
	if len(devices) > 0 && devices[0].Hostname != nil && *devices[0].Hostname != "" {
		return *devices[0].Hostname
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
