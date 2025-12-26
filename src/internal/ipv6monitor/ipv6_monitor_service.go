package ipv6monitor

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"reconya/internal/device"
	"reconya/internal/network"
	"reconya/models"
)

type IPv6MonitorService struct {
	deviceService  *device.DeviceService
	networkService *network.NetworkService
	logger         *log.Logger
	
	// Monitoring state
	isRunning      bool
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.RWMutex
	
	// Configuration
	monitorInterfaces []string
	hostPrefixes     []string
	linkLocalEnabled bool
	multicastEnabled bool
	
	// Channels for async processing
	deviceChan chan IPv6Device
	wg         sync.WaitGroup
}

type IPv6Device struct {
	LinkLocal      string    `json:"link_local"`
	UniqueLocal    string    `json:"unique_local"`
	Global         string    `json:"global"`
	MAC            string    `json:"mac"`
	Interface      string    `json:"interface"`
	Hostname       string    `json:"hostname"`
	Timestamp      time.Time `json:"timestamp"`
	Source         string    `json:"source"` // "ndp", "interface", "multicast"
}

type IPv6Address struct {
	Address   string `json:"address"`
	Type      string `json:"type"`      // "link-local", "unique-local", "global"
	Interface string `json:"interface"`
	MAC       string `json:"mac"`
}

func NewIPv6MonitorService(deviceService *device.DeviceService, networkService *network.NetworkService, logger *log.Logger) *IPv6MonitorService {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &IPv6MonitorService{
		deviceService:     deviceService,
		networkService:    networkService,
		logger:           logger,
		ctx:              ctx,
		cancel:           cancel,
		monitorInterfaces: []string{}, // Will be auto-detected
		hostPrefixes:     []string{},  // Will be auto-detected
		linkLocalEnabled: true,
		multicastEnabled: true,
		deviceChan:       make(chan IPv6Device, 100),
	}
}

func (s *IPv6MonitorService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.isRunning {
		return fmt.Errorf("IPv6 monitor service is already running")
	}
	
	s.logger.Println("Starting IPv6 passive monitoring service...")
	
	// Auto-detect network interfaces and prefixes
	if err := s.detectNetworkConfiguration(); err != nil {
		return fmt.Errorf("failed to detect network configuration: %w", err)
	}
	
	s.isRunning = true
	
	// Start device processing worker
	s.wg.Add(1)
	go s.deviceProcessor()
	
	// Start monitoring routines
	s.wg.Add(1)
	go s.monitorNDPTable()
	
	s.wg.Add(1)
	go s.monitorNetworkInterfaces()
	
	if s.multicastEnabled {
		s.wg.Add(1)
		go s.monitorMulticastTraffic()
	}
	
	s.logger.Printf("IPv6 passive monitoring started for interfaces: %v", s.monitorInterfaces)
	s.logger.Printf("Monitoring IPv6 prefixes: %v", s.hostPrefixes)
	
	return nil
}

func (s *IPv6MonitorService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.isRunning {
		return nil
	}
	
	s.logger.Println("Stopping IPv6 passive monitoring service...")
	
	s.cancel()
	s.isRunning = false
	
	// Close channel and wait for workers to finish
	close(s.deviceChan)
	s.wg.Wait()
	
	s.logger.Println("IPv6 passive monitoring service stopped")
	return nil
}

func (s *IPv6MonitorService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

func (s *IPv6MonitorService) detectNetworkConfiguration() error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to get network interfaces: %w", err)
	}
	
	s.monitorInterfaces = []string{}
	s.hostPrefixes = []string{}
	
	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		
		hasIPv6 := false
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				if ipNet.IP.To4() == nil && ipNet.IP.To16() != nil {
					hasIPv6 = true
					
					// Extract network prefix for global addresses
					if !ipNet.IP.IsLinkLocalUnicast() && !ipNet.IP.IsLoopback() {
						prefix := fmt.Sprintf("%s/%d", ipNet.IP.String(), 64) // Assume /64 for monitoring
						s.hostPrefixes = append(s.hostPrefixes, prefix)
					}
				}
			}
		}
		
		if hasIPv6 {
			s.monitorInterfaces = append(s.monitorInterfaces, iface.Name)
		}
	}
	
	return nil
}

func (s *IPv6MonitorService) monitorNDPTable() {
	defer s.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.scanNDPTable()
		}
	}
}

func (s *IPv6MonitorService) scanNDPTable() {
	var cmd *exec.Cmd
	
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("ip", "-6", "neigh", "show")
	case "darwin":
		cmd = exec.Command("ndp", "-an")
	default:
		s.logger.Printf("IPv6 NDP monitoring not supported on %s", runtime.GOOS)
		return
	}
	
	output, err := cmd.Output()
	if err != nil {
		s.logger.Printf("Failed to get NDP table: %v", err)
		return
	}
	
	devices := s.parseNDPOutput(string(output))
	for _, device := range devices {
		select {
		case s.deviceChan <- device:
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *IPv6MonitorService) parseNDPOutput(output string) []IPv6Device {
	var devices []IPv6Device
	lines := strings.Split(output, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		device := s.parseNDPLine(line)
		if device != nil {
			devices = append(devices, *device)
		}
	}
	
	return devices
}

func (s *IPv6MonitorService) parseNDPLine(line string) *IPv6Device {
	var device IPv6Device
	device.Timestamp = time.Now()
	device.Source = "ndp"
	
	switch runtime.GOOS {
	case "linux":
		// Linux format: "2001:db8::1 dev eth0 lladdr 00:11:22:33:44:55 REACHABLE"
		re := regexp.MustCompile(`^([0-9a-fA-F:]+)\s+dev\s+(\w+)\s+lladdr\s+([0-9a-fA-F:]+)\s+\w+`)
		matches := re.FindStringSubmatch(line)
		if len(matches) == 4 {
			device.Interface = matches[2]
			device.MAC = strings.ToUpper(matches[3])
			
			// Categorize IPv6 address
			if strings.HasPrefix(matches[1], "fe80:") {
				device.LinkLocal = matches[1]
			} else if strings.HasPrefix(matches[1], "fc00:") || strings.HasPrefix(matches[1], "fd00:") {
				device.UniqueLocal = matches[1]
			} else {
				device.Global = matches[1]
			}
		}
		
	case "darwin":
		// macOS format: "2001:db8::1 00:11:22:33:44:55 eth0"
		re := regexp.MustCompile(`^([0-9a-fA-F:]+)\s+([0-9a-fA-F:]+)\s+(\w+)`)
		matches := re.FindStringSubmatch(line)
		if len(matches) == 4 {
			device.Interface = matches[3]
			device.MAC = strings.ToUpper(matches[2])
			
			// Categorize IPv6 address
			if strings.HasPrefix(matches[1], "fe80:") {
				device.LinkLocal = matches[1]
			} else if strings.HasPrefix(matches[1], "fc00:") || strings.HasPrefix(matches[1], "fd00:") {
				device.UniqueLocal = matches[1]
			} else {
				device.Global = matches[1]
			}
		}
	}
	
	// Only return device if we have valid data
	if device.MAC != "" && (device.LinkLocal != "" || device.UniqueLocal != "" || device.Global != "") {
		return &device
	}
	
	return nil
}

func (s *IPv6MonitorService) monitorNetworkInterfaces() {
	defer s.wg.Done()
	
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.scanNetworkInterfaces()
		}
	}
}

func (s *IPv6MonitorService) scanNetworkInterfaces() {
	interfaces, err := net.Interfaces()
	if err != nil {
		s.logger.Printf("Failed to get network interfaces: %v", err)
		return
	}
	
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		
		var device IPv6Device
		device.Interface = iface.Name
		device.Timestamp = time.Now()
		device.Source = "interface"
		device.MAC = iface.HardwareAddr.String()
		
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				if ipNet.IP.To4() == nil && ipNet.IP.To16() != nil {
					// Skip loopback
					if ipNet.IP.IsLoopback() {
						continue
					}
					
					// Categorize IPv6 address
					if ipNet.IP.IsLinkLocalUnicast() {
						device.LinkLocal = ipNet.IP.String()
					} else if isUniqueLocal(ipNet.IP) {
						device.UniqueLocal = ipNet.IP.String()
					} else {
						device.Global = ipNet.IP.String()
					}
				}
			}
		}
		
		// Only process if we have IPv6 addresses
		if device.LinkLocal != "" || device.UniqueLocal != "" || device.Global != "" {
			select {
			case s.deviceChan <- device:
			case <-s.ctx.Done():
				return
			}
		}
	}
}

func (s *IPv6MonitorService) monitorMulticastTraffic() {
	defer s.wg.Done()
	
	// This would require raw socket access and is more complex
	// For now, we'll implement a placeholder that could be extended
	s.logger.Println("IPv6 multicast monitoring placeholder - would require raw socket implementation")
	
	// Keep the goroutine alive until context is done
	<-s.ctx.Done()
}

func (s *IPv6MonitorService) deviceProcessor() {
	defer s.wg.Done()
	
	for {
		select {
		case device, ok := <-s.deviceChan:
			if !ok {
				return
			}
			s.processIPv6Device(device)
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *IPv6MonitorService) processIPv6Device(ipv6Device IPv6Device) {
	// Try to resolve hostname
	if ipv6Device.Hostname == "" {
		if ipv6Device.Global != "" {
			if names, err := net.LookupAddr(ipv6Device.Global); err == nil && len(names) > 0 {
				ipv6Device.Hostname = names[0]
			}
		}
	}
	
	// Find or create device
	var existingDevice *models.Device
	var err error
	
	// Try to find by MAC address first
	if ipv6Device.MAC != "" {
		existingDevice, err = s.deviceService.FindDeviceByMAC(ipv6Device.MAC)
		if err != nil {
			s.logger.Printf("Error finding device by MAC: %v", err)
		}
	}
	
	// If not found by MAC, try by IPv6 addresses
	if existingDevice == nil {
		if ipv6Device.Global != "" {
			existingDevice, err = s.deviceService.FindDeviceByIPv6(ipv6Device.Global)
		} else if ipv6Device.UniqueLocal != "" {
			existingDevice, err = s.deviceService.FindDeviceByIPv6(ipv6Device.UniqueLocal)
		} else if ipv6Device.LinkLocal != "" {
			existingDevice, err = s.deviceService.FindDeviceByIPv6(ipv6Device.LinkLocal)
		}
	}
	
	if existingDevice != nil {
		// Update existing device with IPv6 information
		s.updateDeviceIPv6(existingDevice, ipv6Device)
	} else {
		// Create new device for IPv6-only device
		s.createIPv6Device(ipv6Device)
	}
}

func (s *IPv6MonitorService) updateDeviceIPv6(device *models.Device, ipv6Device IPv6Device) {
	updated := false
	
	// Update IPv6 addresses
	if ipv6Device.LinkLocal != "" && device.IPv6LinkLocal == nil {
		device.IPv6LinkLocal = &ipv6Device.LinkLocal
		updated = true
	}
	
	if ipv6Device.UniqueLocal != "" && device.IPv6UniqueLocal == nil {
		device.IPv6UniqueLocal = &ipv6Device.UniqueLocal
		updated = true
	}
	
	if ipv6Device.Global != "" && device.IPv6Global == nil {
		device.IPv6Global = &ipv6Device.Global
		updated = true
	}
	
	// Update status to online
	if device.Status != models.DeviceStatusOnline {
		device.Status = models.DeviceStatusOnline
		now := time.Now()
		device.LastSeenOnlineAt = &now
		updated = true
	}
	
	// Update hostname if we have one
	if ipv6Device.Hostname != "" && device.Hostname == nil {
		device.Hostname = &ipv6Device.Hostname
		updated = true
	}
	
	if updated {
		device.UpdatedAt = time.Now()
		if err := s.deviceService.UpdateDeviceRecord(device); err != nil {
			s.logger.Printf("Failed to update device with IPv6 info: %v", err)
		} else {
			s.logger.Printf("Updated device %s with IPv6 addresses", device.Name)
		}
	}
}

func (s *IPv6MonitorService) createIPv6Device(ipv6Device IPv6Device) {
	// Skip creating IPv6-only devices for now to avoid 0.0.0.0 entries
	// Instead, log that we found an IPv6-only device
	s.logger.Printf("Found IPv6-only device (not creating): MAC=%s, IPv6=%s", 
		ipv6Device.MAC, 
		func() string {
			if ipv6Device.Global != "" {
				return ipv6Device.Global
			}
			if ipv6Device.UniqueLocal != "" {
				return ipv6Device.UniqueLocal
			}
			return ipv6Device.LinkLocal
		}())
	
	// TODO: Implement proper dual-stack support to handle IPv6-only devices
	// For now, we only add IPv6 addresses to existing IPv4 devices
}

// Helper functions
func isUniqueLocal(ip net.IP) bool {
	// Unique Local addresses: fc00::/7 (fc00:: to fdff::)
	return ip[0] == 0xfc || ip[0] == 0xfd
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func generateDeviceName(ipv6Device IPv6Device) string {
	if ipv6Device.Hostname != "" {
		return ipv6Device.Hostname
	}
	
	if ipv6Device.Global != "" {
		return fmt.Sprintf("IPv6-%s", ipv6Device.Global[0:8])
	}
	
	if ipv6Device.UniqueLocal != "" {
		return fmt.Sprintf("IPv6-ULA-%s", ipv6Device.UniqueLocal[0:8])
	}
	
	if ipv6Device.LinkLocal != "" {
		return fmt.Sprintf("IPv6-LL-%s", ipv6Device.LinkLocal[0:8])
	}
	
	return fmt.Sprintf("IPv6-Device-%s", ipv6Device.MAC)
}

// GetMonitoringStatus returns the current monitoring status
func (s *IPv6MonitorService) GetMonitoringStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return map[string]interface{}{
		"running":            s.isRunning,
		"monitored_interfaces": s.monitorInterfaces,
		"host_prefixes":      s.hostPrefixes,
		"link_local_enabled": s.linkLocalEnabled,
		"multicast_enabled":  s.multicastEnabled,
		"queue_size":         len(s.deviceChan),
	}
}