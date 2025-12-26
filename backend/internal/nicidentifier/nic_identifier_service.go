package nicidentifier

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"reconya/internal/config"
	"reconya/internal/device"
	"reconya/internal/eventlog"
	"reconya/internal/network"
	"reconya/internal/systemstatus"
	"reconya/models"
)

type NicIdentifierService struct {
	NetworkService      *network.NetworkService
	SystemStatusService *systemstatus.SystemStatusService
	EventLogService     *eventlog.EventLogService
	DeviceService       *device.DeviceService
	Config              *config.Config
}

func NewNicIdentifierService(
	networkService *network.NetworkService,
	systemStatusService *systemstatus.SystemStatusService,
	eventLogService *eventlog.EventLogService,
	deviceService *device.DeviceService,
	config *config.Config) *NicIdentifierService {
	return &NicIdentifierService{
		NetworkService:      networkService,
		SystemStatusService: systemStatusService,
		EventLogService:     eventLogService,
		DeviceService:       deviceService,
		Config:              config,
	}
}

func (s *NicIdentifierService) Identify() {
	log.Printf("Attempting network identification")
	nic := s.getLocalNic()
	fmt.Printf("NIC: %v\n", nic)
	
	// Check for new networks and suggest creation
	s.CheckForNewNetworks()
	
	publicIP, err := s.getPublicIp()
	if err != nil {
		log.Printf("Failed to get public IP: %v", err)
		return
	}
	log.Printf("Public IP Address found: [%v]", publicIP)

	// Try to find an existing network for the primary NIC for system status
	var networkEntity *models.Network
	if nic.IPv4 != "" {
		// Calculate the /24 network for this IP
		ip := net.ParseIP(nic.IPv4)
		if ip != nil {
			ip4 := ip.To4()
			if ip4 != nil {
				// Calculate /24 network
				cidr := fmt.Sprintf("%d.%d.%d.0/24", ip4[0], ip4[1], ip4[2])
				log.Printf("Looking for existing network for primary NIC: %s", cidr)
				
				// Only look for existing network, don't create automatically
				existing, err := s.NetworkService.FindByCIDR(cidr)
				if err != nil {
					log.Printf("Error searching for network %s: %v", cidr, err)
				} else if existing != nil {
					log.Printf("Found existing network: %s", existing.CIDR)
					networkEntity = existing
				} else {
					log.Printf("No existing network found for %s - will be suggested via UI", cidr)
				}
			}
		}
	}

	localDevice := models.Device{
		Name:   nic.Name,
		IPv4:   nic.IPv4,
		Status: models.DeviceStatusOnline,
	}

	savedDevice, err := s.DeviceService.CreateOrUpdate(&localDevice)
	if err != nil {
		log.Printf("Failed to save or update local device: %v", err)
		return
	}

	systemStatus := models.SystemStatus{
		LocalDevice: *savedDevice,
		PublicIP:    &publicIP,
	}

	// Set NetworkID if we have a valid network entity
	if networkEntity != nil {
		systemStatus.NetworkID = networkEntity.ID
	}

	// Fetch geolocation for public IP
	if publicIP != "" {
		geo, err := s.SystemStatusService.FetchGeolocation(publicIP)
		if err == nil && geo != nil {
			systemStatus.Geolocation = geo
			log.Printf("Added geolocation for public IP %s: %s, %s", publicIP, geo.City, geo.Country)
		} else if err != nil {
			log.Printf("Failed to fetch geolocation for public IP %s: %v", publicIP, err)
		}
	}

	_, err = s.SystemStatusService.CreateOrUpdate(&systemStatus)
	if err != nil {
		log.Printf("Failed to create or update system status: %v", err)
		return
	}

	device := savedDevice.ID
	s.EventLogService.CreateOne(&models.EventLog{
		Type:     models.LocalIPFound,
		DeviceID: &device,
	})

	s.EventLogService.CreateOne(&models.EventLog{
		Type: models.LocalNetworkFound,
	})
}

func (s *NicIdentifierService) getLocalNic() models.NIC {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Println("Error getting network interfaces:", err)
		return models.NIC{}
	}

	var candidates []models.NIC
	var dockerInterfaces []models.NIC

	for _, iface := range interfaces {
		fmt.Printf("Checking interface: %s\n", iface.Name)
		if iface.Flags&net.FlagUp == 0 {
			fmt.Printf("Skipping %s: interface is down\n", iface.Name)
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			fmt.Printf("Skipping %s: interface is loopback\n", iface.Name)
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			fmt.Printf("Skipping %s: error getting addresses: %v\n", iface.Name, err)
			continue
		}

		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil || ip.To4() == nil {
				fmt.Printf("Skipping address %s on %s: not a valid IPv4\n", addr.String(), iface.Name)
				continue
			}

			if !ip.IsLoopback() {
				nic := models.NIC{Name: iface.Name, IPv4: ip.String()}
				
				// Check if this is a Docker or container network
				if s.isDockerOrContainerNetwork(ip.String()) {
					fmt.Printf("Found Docker/container interface: %s with IPv4: %s\n", iface.Name, ip.String())
					dockerInterfaces = append(dockerInterfaces, nic)
				} else {
					fmt.Printf("Found potential host interface: %s with IPv4: %s\n", iface.Name, ip.String())
					candidates = append(candidates, nic)
				}
			}
		}
	}

	// Prefer non-Docker interfaces
	if len(candidates) > 0 {
		// Prioritize common home/office networks
		for _, nic := range candidates {
			if s.isCommonPrivateNetwork(nic.IPv4) {
				fmt.Printf("Selected preferred interface: %s with IPv4: %s\n", nic.Name, nic.IPv4)
				return nic
			}
		}
		// If no common private networks, return first candidate
		fmt.Printf("Selected first non-Docker interface: %s with IPv4: %s\n", candidates[0].Name, candidates[0].IPv4)
		return candidates[0]
	}

	// Fallback to Docker interfaces if no others available
	if len(dockerInterfaces) > 0 {
		fmt.Printf("Using Docker interface as fallback: %s with IPv4: %s\n", dockerInterfaces[0].Name, dockerInterfaces[0].IPv4)
		return dockerInterfaces[0]
	}

	return models.NIC{}
}

// isDockerOrContainerNetwork checks if an IP belongs to common container networks
func (s *NicIdentifierService) isDockerOrContainerNetwork(ip string) bool {
	// Common Docker and container network ranges
	dockerRanges := []string{
		"172.17.0.0/16",    // Default Docker bridge
		"172.18.0.0/16",    // Docker custom networks
		"172.19.0.0/16",
		"172.20.0.0/16",
		"172.21.0.0/16",
		"172.22.0.0/16",
		"172.23.0.0/16",
		"172.24.0.0/16",
		"172.25.0.0/16",
		"172.26.0.0/16",
		"172.27.0.0/16",
		"172.28.0.0/16",
		"172.29.0.0/16",
		"172.30.0.0/16",
		"172.31.0.0/16",
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

// isCommonPrivateNetwork checks if an IP belongs to common home/office networks
func (s *NicIdentifierService) isCommonPrivateNetwork(ip string) bool {
	// Common home/office network ranges
	commonRanges := []string{
		"192.168.0.0/16",   // Most common home networks
		"10.0.0.0/8",       // Corporate networks
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	for _, cidr := range commonRanges {
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

func (s *NicIdentifierService) getPublicIp() (string, error) {
	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(ip), nil
}

// CheckForNewNetworks detects new networks from active NICs and suggests creation
func (s *NicIdentifierService) CheckForNewNetworks() {
	log.Printf("Checking for new networks...")
	
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Error getting network interfaces for network detection: %v", err)
		return
	}

	var detectedNetworks []string
	
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			
			ip := ipNet.IP.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}

			// Skip Docker/container networks
			if s.isDockerOrContainerNetwork(ip.String()) {
				continue
			}

			// Calculate network CIDR
			networkCIDR := ipNet.String()
			detectedNetworks = append(detectedNetworks, networkCIDR)
			
			log.Printf("Detected active network: %s on interface %s", networkCIDR, iface.Name)
		}
	}
	
	// Check if detected networks exist in database
	for _, networkCIDR := range detectedNetworks {
		s.checkAndSuggestNetwork(networkCIDR)
	}
}

// checkAndSuggestNetwork checks if a network exists and creates suggestion if not
func (s *NicIdentifierService) checkAndSuggestNetwork(networkCIDR string) {
	// Parse the network to get the base network address
	_, ipNet, err := net.ParseCIDR(networkCIDR)
	if err != nil {
		log.Printf("Error parsing network CIDR %s: %v", networkCIDR, err)
		return
	}
	
	// Get the network address (not the host IP)
	networkAddr := ipNet.IP.String()
	ones, _ := ipNet.Mask.Size()
	baseNetworkCIDR := fmt.Sprintf("%s/%d", networkAddr, ones)
	
	log.Printf("Checking if network %s exists (derived from %s)", baseNetworkCIDR, networkCIDR)
	
	// Check if this network already exists
	existing, err := s.NetworkService.FindByCIDR(baseNetworkCIDR)
	if err != nil {
		log.Printf("Error checking existing network %s: %v", baseNetworkCIDR, err)
		return
	}
	
	if existing != nil {
		log.Printf("Network %s already exists, skipping suggestion", baseNetworkCIDR)
		return
	}
	
	// Network doesn't exist - log suggestion event
	log.Printf("New network detected: %s", baseNetworkCIDR)
	s.EventLogService.CreateOne(&models.EventLog{
		Type:        models.NewNetworkDetected,
		Description: fmt.Sprintf("New network %s detected. Consider creating it for scanning.", baseNetworkCIDR),
	})
}

// GetDetectedNetworks returns a list of detected networks that don't exist in the database
func (s *NicIdentifierService) GetDetectedNetworks() []DetectedNetwork {
	var detected []DetectedNetwork
	
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Error getting network interfaces: %v", err)
		return detected
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			
			ip := ipNet.IP.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}

			// Skip Docker/container networks
			if s.isDockerOrContainerNetwork(ip.String()) {
				continue
			}

			// Calculate base network CIDR (network address, not host IP)
			ones, _ := ipNet.Mask.Size()
			// Apply mask to get network address
			networkIP := ipNet.IP.Mask(ipNet.Mask)
			baseNetworkCIDR := fmt.Sprintf("%s/%d", networkIP.String(), ones)
			
			// Check if this network exists
			existing, err := s.NetworkService.FindByCIDR(baseNetworkCIDR)
			if err != nil || existing != nil {
				continue
			}
			
			// Add to detected networks
			detected = append(detected, DetectedNetwork{
				CIDR:      baseNetworkCIDR,
				Interface: iface.Name,
				IP:        ip.String(),
			})
		}
	}
	
	return detected
}

// DetectedNetwork represents a detected network that doesn't exist in the database
type DetectedNetwork struct {
	CIDR      string `json:"cidr"`
	Interface string `json:"interface"`
	IP        string `json:"ip"`
}
