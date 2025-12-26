package device

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"net"
	"reconya/db"
	"reconya/internal/config"
	"reconya/internal/fingerprint"
	"reconya/internal/network"
	"reconya/internal/oui"
	"reconya/internal/util"
	"reconya/models"
	"sort"
	"strings"
	"time"
)

type DeviceService struct {
	Config             *config.Config
	repository         db.DeviceRepository
	networkService     *network.NetworkService
	dbManager          *db.DBManager
	fingerprintService *fingerprint.FingerprintService
	ouiService         *oui.OUIService
}

func NewDeviceService(deviceRepo db.DeviceRepository, networkService *network.NetworkService, cfg *config.Config, dbManager *db.DBManager, ouiService *oui.OUIService) *DeviceService {
	return &DeviceService{
		Config:             cfg,
		repository:         deviceRepo,
		networkService:     networkService,
		dbManager:          dbManager,
		fingerprintService: fingerprint.NewFingerprintService(),
		ouiService:         ouiService,
	}
}

func (s *DeviceService) CreateOrUpdate(device *models.Device) (*models.Device, error) {
	currentTime := time.Now()
	device.LastSeenOnlineAt = &currentTime

	// If device doesn't have a network ID, we can't proceed
	// The scan manager should set the network ID before calling this method
	if device.NetworkID == "" {
		return nil, fmt.Errorf("device must have a network ID set")
	}

	// Get the network to check CIDR
	network, err := s.networkService.FindByID(device.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to find network: %v", err)
	}
	if network == nil {
		return nil, fmt.Errorf("network not found")
	}

	// Skip network and broadcast addresses
	if s.isNetworkOrBroadcastAddress(device.IPv4, network.CIDR) {
		log.Printf("Skipping network/broadcast address: %s", device.IPv4)
		return nil, fmt.Errorf("network or broadcast address not allowed: %s", device.IPv4)
	}

	// First try to find device by IP address
	existingDevice, err := s.FindByIPv4(device.IPv4)
	if err != nil && err != db.ErrNotFound {
		return nil, err
	}

	// If no device found by IP and we have a MAC address, try to find by MAC
	// This handles cases where a device changes IP but keeps the same MAC (DHCP reassignment)
	if existingDevice == nil && device.MAC != nil && *device.MAC != "" {
		existingByMAC, err := s.FindDeviceByMAC(*device.MAC)
		if err == nil && existingByMAC != nil {
			log.Printf("Found existing device by MAC %s, updating IP from %s to %s", 
				*device.MAC, existingByMAC.IPv4, device.IPv4)
			
			// Update the existing device's IP address and other fields
			existingByMAC.IPv4 = device.IPv4
			existingByMAC.Hostname = device.Hostname
			existingByMAC.Vendor = device.Vendor
			existingByMAC.NetworkID = device.NetworkID
			existingByMAC.LastSeenOnlineAt = &currentTime
			existingByMAC.Status = models.DeviceStatusOnline
			
			existingDevice = existingByMAC
			// Set the device ID to the existing device to ensure we update rather than create
			device.ID = existingDevice.ID
		}
	}

	s.setTimestamps(device, existingDevice, currentTime)

	// Set status if not already set
	if device.Status == "" {
		device.Status = models.DeviceStatusOnline
	}

	// Preserve name and comment if device already exists and incoming values are empty
	if existingDevice != nil {
		if device.Name == "" && existingDevice.Name != "" {
			device.Name = existingDevice.Name
		}
		if (device.Comment == nil || *device.Comment == "") && existingDevice.Comment != nil && *existingDevice.Comment != "" {
			device.Comment = existingDevice.Comment
		}
	}

	// Leave device name empty if not explicitly set

	// Use DB manager to serialize database access
	return s.dbManager.CreateOrUpdateDevice(s.repository, context.Background(), device)
}

func (s *DeviceService) setTimestamps(device, existingDevice *models.Device, currentTime time.Time) {
	if existingDevice == nil || existingDevice.CreatedAt.IsZero() {
		device.CreatedAt = currentTime
	} else {
		device.CreatedAt = existingDevice.CreatedAt
	}
	device.UpdatedAt = currentTime
}

func (s *DeviceService) ParseFromNmap(bufferStream string) []models.Device {
	log.Println("Starting Nmap parse")
	var devices []models.Device
	lines := strings.Split(bufferStream, "\n")

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "Nmap scan report for") {
			device := models.Device{}
			parts := strings.Fields(line)
			if len(parts) >= 5 {
				ipAddress := strings.Trim(parts[len(parts)-1], "()")
				device.IPv4 = ipAddress

				if len(parts) == 6 {
					hostname := strings.Trim(parts[4], "()")
					if hostname != ipAddress {
						device.Hostname = &hostname
					} else {
						empty := ""
						device.Hostname = &empty
					}
				}

				if device.Hostname != nil {
					log.Printf("Found device - IP: %s, Hostname: %s", device.IPv4, *device.Hostname)
				} else {
					log.Printf("Found device - IP: %s, Hostname: <nil>", device.IPv4)
				}

			}

			if i+2 < len(lines) && strings.HasPrefix(lines[i+2], "MAC Address: ") {
				macParts := strings.Fields(lines[i+2])
				if len(macParts) >= 3 {
					mac := macParts[2]
					device.MAC = &mac
					log.Printf("Device MAC Address: %s", *device.MAC)
				}
			}

			devices = append(devices, device)
		}
	}

	log.Printf("Finished parsing Nmap output. Total devices found: %d", len(devices))
	return devices
}

func (s *DeviceService) ParseFromNmapXML(xmlOutput string) []models.Device {
	log.Println("Starting Nmap XML parse")
	var devices []models.Device
	var nmapXML models.NmapXML

	err := xml.Unmarshal([]byte(xmlOutput), &nmapXML)
	if err != nil {
		log.Printf("Error parsing Nmap XML output: %v", err)
		// Fallback to text parsing
		return s.ParseFromNmap(xmlOutput)
	}

	for _, host := range nmapXML.Hosts {
		device := models.Device{}
		var macAddress, vendor string

		// Extract IP address, MAC address, and vendor info
		for _, address := range host.Addresses {
			if address.AddrType == "ipv4" {
				device.IPv4 = address.Addr
			} else if address.AddrType == "mac" {
				macAddress = address.Addr
				vendor = address.Vendor
			}
		}

		// Skip if no IP address found
		if device.IPv4 == "" {
			continue
		}

		// Set MAC address if found
		if macAddress != "" {
			device.MAC = &macAddress
			log.Printf("Found MAC Address: %s for IP: %s", macAddress, device.IPv4)
		}

		// Set vendor info if found from Nmap
		if vendor != "" {
			device.Vendor = &vendor
			log.Printf("Found Vendor from Nmap: %s for IP: %s", vendor, device.IPv4)
		} else if macAddress != "" && s.ouiService != nil {
			// Fallback to OUI lookup if Nmap didn't provide vendor info
			if ouiVendor := s.ouiService.LookupVendor(macAddress); ouiVendor != "" {
				device.Vendor = &ouiVendor
				log.Printf("Found Vendor from OUI: %s for MAC: %s (IP: %s)", ouiVendor, macAddress, device.IPv4)
			}
		}

		// Extract hostname if available
		if len(host.Hostnames) > 0 && host.Hostnames[0].Name != "" {
			hostname := host.Hostnames[0].Name
			device.Hostname = &hostname
			log.Printf("Found Hostname: %s for IP: %s", hostname, device.IPv4)
		}

		log.Printf("Found device - IP: %s, MAC: %v, Vendor: %v, Hostname: %v",
			device.IPv4,
			func() string {
				if device.MAC != nil {
					return *device.MAC
				} else {
					return "<nil>"
				}
			}(),
			func() string {
				if device.Vendor != nil {
					return *device.Vendor
				} else {
					return "<nil>"
				}
			}(),
			func() string {
				if device.Hostname != nil {
					return *device.Hostname
				} else {
					return "<nil>"
				}
			}())

		devices = append(devices, device)
	}

	log.Printf("Finished parsing Nmap XML output. Total devices found: %d", len(devices))
	return devices
}

func (s *DeviceService) EligibleForPortScan(device *models.Device) bool {
	if device == nil {
		log.Println("Warning: Attempted to check port scan eligibility for a nil device")
		return false
	}

	now := time.Now()
	if device.PortScanEndedAt != nil && device.PortScanEndedAt.Add(30*time.Second).After(now) {
		return false
	}
	return true
}

func sortDevicesByIP(devices []models.Device) {
	sort.Slice(devices, func(i, j int) bool {
		ip1 := net.ParseIP(devices[i].IPv4)
		ip2 := net.ParseIP(devices[j].IPv4)
		return bytes.Compare(ip1, ip2) < 0
	})
}

// sortDevicePointersByIP sorts a slice of device pointers by IP address
func sortDevicePointersByIP(devices []*models.Device) {
	sort.Slice(devices, func(i, j int) bool {
		ip1 := net.ParseIP(devices[i].IPv4)
		ip2 := net.ParseIP(devices[j].IPv4)
		
		if ip1 == nil || ip2 == nil {
			return devices[i].IPv4 < devices[j].IPv4
		}
		
		return bytes.Compare(ip1, ip2) < 0
	})
}

func (s *DeviceService) FindAll() ([]*models.Device, error) {
	ctx := context.Background()
	devices, err := s.repository.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	// Sort devices by IP address directly on pointers
	sortDevicePointersByIP(devices)

	return devices, nil
}

func (s *DeviceService) FindByID(deviceID string) (*models.Device, error) {
	ctx := context.Background()
	device, err := s.repository.FindByID(ctx, deviceID)
	if err == db.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		log.Printf("Error finding device with ID %s: %v", deviceID, err)
		return nil, err
	}
	return device, nil
}

func (s *DeviceService) Delete(deviceID string) error {
	ctx := context.Background()
	
	// Check if device exists before attempting deletion
	device, err := s.repository.FindByID(ctx, deviceID)
	if err == db.ErrNotFound {
		return fmt.Errorf("device not found")
	}
	if err != nil {
		log.Printf("Error finding device with ID %s for deletion: %v", deviceID, err)
		return err
	}
	
	// Delete the device (this will cascade to ports and web services)
	err = s.repository.DeleteByID(ctx, deviceID)
	if err != nil {
		log.Printf("Error deleting device with ID %s: %v", deviceID, err)
		return err
	}
	
	log.Printf("Successfully deleted device %s (%s)", device.IPv4, deviceID)
	return nil
}

// DeleteByNetworkID deletes all devices belonging to a specific network
func (s *DeviceService) DeleteByNetworkID(networkID string) error {
	ctx := context.Background()
	
	// Find all devices for this network
	devices, err := s.FindByNetworkID(networkID)
	if err != nil {
		return fmt.Errorf("failed to find devices for network %s: %v", networkID, err)
	}
	
	log.Printf("Deleting %d devices from network %s", len(devices), networkID)
	
	// Delete each device
	var errors []string
	deletedCount := 0
	
	for _, device := range devices {
		err := s.repository.DeleteByID(ctx, device.ID)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to delete device %s (%s): %v", device.IPv4, device.ID, err))
			continue
		}
		deletedCount++
		log.Printf("Deleted device %s (%s)", device.IPv4, device.ID)
	}
	
	if len(errors) > 0 {
		log.Printf("Deleted %d devices with %d errors", deletedCount, len(errors))
		for _, errMsg := range errors {
			log.Printf("Error: %s", errMsg)
		}
		return fmt.Errorf("deleted %d devices but encountered %d errors", deletedCount, len(errors))
	}
	
	log.Printf("Successfully deleted all %d devices from network %s", deletedCount, networkID)
	return nil
}

func (s *DeviceService) FindByIPv4(ipv4 string) (*models.Device, error) {
	ctx := context.Background()
	device, err := s.repository.FindByIP(ctx, ipv4)
	if err == db.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		log.Printf("Error finding device with IPv4 %s: %v", ipv4, err)
		return nil, err
	}
	return device, nil
}

func (s *DeviceService) FindByNetworkID(networkID string) ([]models.Device, error) {
	ctx := context.Background()
	devices, err := s.repository.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	// Filter devices by network ID
	var filteredDevices []models.Device
	for _, d := range devices {
		if d.NetworkID == networkID {
			filteredDevices = append(filteredDevices, *d)
		}
	}

	// Sort devices by IP address
	sortDevicesByIP(filteredDevices)

	return filteredDevices, nil
}

func (s *DeviceService) FindAllForNetwork(cidr string) ([]models.Device, error) {
	var deviceValues []models.Device

	network, err := s.networkService.FindByCIDR(cidr)
	if err != nil {
		return nil, err
	}

	if network == nil {
		return deviceValues, nil
	}
	// Get all devices first
	ctx := context.Background()
	allDevices, err := s.repository.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	// Filter devices by network ID
	for _, d := range allDevices {
		// Make sure we're comparing non-empty values
		if d.NetworkID != "" && network.ID != "" && d.NetworkID == network.ID {
			deviceValues = append(deviceValues, *d)
		} else if d.NetworkID == "" && network.ID != "" {
			// If device has no network ID but belongs to the current network
			// The device might be in this network but the ID wasn't saved
			// This is a workaround for existing data
			d.NetworkID = network.ID

			// Use retry logic for updating the device
			_, err := util.RetryOnLockWithResult(func() (*models.Device, error) {
				return s.repository.CreateOrUpdate(context.Background(), d)
			})

			if err != nil {
				log.Printf("Error updating device network ID: %v", err)
			} else {
				deviceValues = append(deviceValues, *d)
			}
		} else {
			log.Printf("Skipping device %s (network ID mismatch)", d.IPv4)
		}
	}

	sortDevicesByIP(deviceValues)
	return deviceValues, nil
}

// FindOnlineDevicesForNetwork returns only devices that have been actually discovered online
func (s *DeviceService) FindOnlineDevicesForNetwork(cidr string) ([]models.Device, error) {
	network, err := s.networkService.FindByCIDR(cidr)
	if err != nil {
		return nil, err
	}

	if network == nil {
		return []models.Device{}, nil
	}

	// Get all devices first
	ctx := context.Background()
	allDevices, err := s.repository.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	var deviceValues []models.Device

	// Filter devices by network ID AND only include devices that have been seen online
	for _, d := range allDevices {
		// Skip devices that have never been seen online
		if d.LastSeenOnlineAt == nil {
			continue
		}

		// Show online and idle devices - only skip offline devices
		if d.Status == models.DeviceStatusOffline {
			continue
		}

		// Skip network and broadcast addresses
		if s.isNetworkOrBroadcastAddress(d.IPv4, cidr) {
			continue
		}

		// Check network membership
		shouldInclude := false
		if d.NetworkID != "" && network.ID != "" && d.NetworkID == network.ID {
			shouldInclude = true
		} else if d.NetworkID == "" && network.ID != "" {
			// If device has no network ID but belongs to the current network
			d.NetworkID = network.ID

			// Use retry logic for updating the device
			_, err := util.RetryOnLockWithResult(func() (*models.Device, error) {
				return s.repository.CreateOrUpdate(context.Background(), d)
			})

			if err != nil {
				log.Printf("Error updating device network ID: %v", err)
			}
			shouldInclude = true
		}

		if shouldInclude {
			deviceValues = append(deviceValues, *d)
		}
	}

	sortDevicesByIP(deviceValues)
	log.Printf("Filtered to %d active devices (online/idle)", len(deviceValues))
	return deviceValues, nil
}

// isNetworkOrBroadcastAddress checks if an IP is a network or broadcast address
func (s *DeviceService) isNetworkOrBroadcastAddress(ipStr, cidrStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return true // Invalid IP, exclude it
	}

	_, network, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return false // Can't parse CIDR, include the IP
	}

	// Check if it's the network address
	if ip.Equal(network.IP) {
		return true
	}

	// Check if it's the broadcast address
	// For IPv4, calculate the broadcast address
	if ip.To4() != nil {
		mask := network.Mask
		broadcast := make(net.IP, len(network.IP))
		for i := range network.IP {
			broadcast[i] = network.IP[i] | ^mask[i]
		}
		if ip.Equal(broadcast) {
			return true
		}
	}

	return false
}

func (s *DeviceService) UpdateDeviceStatuses() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use DB manager to serialize database access
	// Device status transitions: online -> idle after 1 minute, idle/online -> offline after 3 minutes
	return s.dbManager.UpdateDeviceStatuses(s.repository, ctx, 3*time.Minute)
}

// PerformDeviceFingerprinting analyzes device characteristics to determine type and OS
func (s *DeviceService) UpdateDevice(deviceID string, name *string, comment *string) (*models.Device, error) {
	ctx := context.Background()

	device, err := s.repository.FindByID(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	if name != nil {
		device.Name = *name
	}
	if comment != nil {
		device.Comment = comment
	}

	updatedDevice, err := s.repository.CreateOrUpdate(ctx, device)
	if err != nil {
		return nil, fmt.Errorf("failed to update device: %v", err)
	}

	return updatedDevice, nil
}

func (s *DeviceService) PerformDeviceFingerprinting(device *models.Device) {
	log.Printf("Starting device fingerprinting for %s", device.IPv4)
	s.fingerprintService.AnalyzeDevice(device)
}

// CleanupAllDeviceNames clears the names of all devices in the database
// IPv6-specific methods
func (s *DeviceService) FindDeviceByIPv6(ipv6Address string) (*models.Device, error) {
	devices, err := s.FindAll()
	if err != nil {
		return nil, err
	}
	
	for _, device := range devices {
		if device.IPv6LinkLocal != nil && *device.IPv6LinkLocal == ipv6Address {
			return device, nil
		}
		if device.IPv6UniqueLocal != nil && *device.IPv6UniqueLocal == ipv6Address {
			return device, nil
		}
		if device.IPv6Global != nil && *device.IPv6Global == ipv6Address {
			return device, nil
		}
		
		// Check additional IPv6 addresses
		for _, addr := range device.IPv6Addresses {
			if addr == ipv6Address {
				return device, nil
			}
		}
	}
	
	return nil, fmt.Errorf("device not found with IPv6 address: %s", ipv6Address)
}

func (s *DeviceService) FindDeviceByMAC(macAddress string) (*models.Device, error) {
	devices, err := s.FindAll()
	if err != nil {
		return nil, err
	}
	
	for _, device := range devices {
		if device.MAC != nil && *device.MAC == macAddress {
			return device, nil
		}
	}
	
	return nil, fmt.Errorf("device not found with MAC address: %s", macAddress)
}

func (s *DeviceService) UpdateDeviceIPv6Addresses(deviceID string, ipv6Addresses map[string]string) error {
	device, err := s.FindByID(deviceID)
	if err != nil {
		return err
	}
	
	// Update IPv6 addresses
	if linkLocal, ok := ipv6Addresses["link_local"]; ok && linkLocal != "" {
		device.IPv6LinkLocal = &linkLocal
	}
	if uniqueLocal, ok := ipv6Addresses["unique_local"]; ok && uniqueLocal != "" {
		device.IPv6UniqueLocal = &uniqueLocal
	}
	if global, ok := ipv6Addresses["global"]; ok && global != "" {
		device.IPv6Global = &global
	}
	
	// Update additional addresses
	if additional, ok := ipv6Addresses["additional"]; ok && additional != "" {
		device.AddIPv6Address(additional)
	}
	
	device.UpdatedAt = time.Now()
	
	// Update device in database
	_, err = s.repository.CreateOrUpdate(context.Background(), device)
	return err
}

func (s *DeviceService) GetDevicesByIPv6Prefix(prefix string) ([]models.Device, error) {
	devices, err := s.FindAll()
	if err != nil {
		return nil, err
	}
	
	var result []models.Device
	for _, device := range devices {
		if device.HasIPv6() {
			allAddresses := device.GetAllIPv6Addresses()
			for _, addr := range allAddresses {
				if strings.HasPrefix(addr, prefix) {
					result = append(result, *device)
					break
				}
			}
		}
	}
	
	return result, nil
}

func (s *DeviceService) CreateDevice(device *models.Device) error {
	// Generate ID if not set
	if device.ID == "" {
		device.ID = generateDeviceID()
	}
	
	// Set timestamps
	now := time.Now()
	device.CreatedAt = now
	device.UpdatedAt = now
	
	// Create device in database
	_, err := s.repository.CreateOrUpdate(context.Background(), device)
	return err
}

func (s *DeviceService) UpdateDeviceRecord(device *models.Device) error {
	device.UpdatedAt = time.Now()
	_, err := s.repository.CreateOrUpdate(context.Background(), device)
	return err
}

func generateDeviceID() string {
	return fmt.Sprintf("device_%d", time.Now().UnixNano())
}

func (s *DeviceService) CleanupNetworkBroadcastDevices() error {
	ctx := context.Background()
	
	// Get all devices
	devices, err := s.repository.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch devices: %v", err)
	}
	
	// Get all networks to check CIDRs
	networks, err := s.networkService.FindAll()
	if err != nil {
		return fmt.Errorf("failed to fetch networks: %v", err)
	}
	
	var deletedCount int
	for _, device := range devices {
		if device.NetworkID == "" {
			continue
		}
		
		// Find the network for this device
		var network *models.Network
		for _, n := range networks {
			if n.ID == device.NetworkID {
				network = &n
				break
			}
		}
		
		if network == nil {
			continue
		}
		
		// Check if this device is a network/broadcast address
		if s.isNetworkOrBroadcastAddress(device.IPv4, network.CIDR) {
			log.Printf("Cleaning up network/broadcast device: %s", device.IPv4)
			if err := s.repository.DeleteByID(ctx, device.ID); err != nil {
				log.Printf("Failed to delete device %s: %v", device.IPv4, err)
			} else {
				deletedCount++
			}
		}
	}
	
	log.Printf("Cleaned up %d network/broadcast address devices", deletedCount)
	return nil
}

func (s *DeviceService) CleanupAllDeviceNames() error {
	ctx := context.Background()
	
	// Get all devices
	devices, err := s.repository.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch devices: %v", err)
	}
	
	log.Printf("Starting device name cleanup for %d devices", len(devices))
	
	// Update each device to clear the name
	var errors []string
	for _, device := range devices {
		// Clear the device name
		device.Name = ""
		
		// Update the device
		_, err := s.repository.CreateOrUpdate(ctx, device)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to update device %s: %v", device.IPv4, err))
			continue
		}
		
		log.Printf("Cleared name for device %s", device.IPv4)
	}
	
	if len(errors) > 0 {
		log.Printf("Device name cleanup completed with %d errors", len(errors))
		for _, errMsg := range errors {
			log.Printf("Error: %s", errMsg)
		}
		return fmt.Errorf("cleanup completed with %d errors", len(errors))
	}
	
	log.Printf("Device name cleanup completed successfully for %d devices", len(devices))
	return nil
}

// CleanupDuplicateDevices finds and removes duplicate devices with the same MAC address
// Keeps the most recently updated device and preserves user-set names and comments
func (s *DeviceService) CleanupDuplicateDevices() error {
	ctx := context.Background()
	
	// Get all devices
	devices, err := s.repository.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch devices: %v", err)
	}
	
	log.Printf("Starting duplicate device cleanup for %d devices", len(devices))
	
	// Group devices by MAC address (skip devices without MAC)
	macGroups := make(map[string][]*models.Device)
	for _, device := range devices {
		if device.MAC != nil && *device.MAC != "" {
			macGroups[*device.MAC] = append(macGroups[*device.MAC], device)
		}
	}
	
	var deletedCount int
	var errors []string
	
	// Process each MAC group
	for mac, deviceGroup := range macGroups {
		if len(deviceGroup) <= 1 {
			continue // No duplicates
		}
		
		log.Printf("Found %d devices with MAC %s", len(deviceGroup), mac)
		
		// Sort by UpdatedAt to find the most recent
		sort.Slice(deviceGroup, func(i, j int) bool {
			return deviceGroup[i].UpdatedAt.After(deviceGroup[j].UpdatedAt)
		})
		
		keeper := deviceGroup[0] // Most recently updated
		duplicates := deviceGroup[1:]
		
		// Preserve user-set data from duplicates
		for _, duplicate := range duplicates {
			if duplicate.Name != "" && keeper.Name == "" {
				keeper.Name = duplicate.Name
			}
			if duplicate.Comment != nil && *duplicate.Comment != "" && 
			   (keeper.Comment == nil || *keeper.Comment == "") {
				keeper.Comment = duplicate.Comment
			}
		}
		
		// Update the keeper with preserved data
		_, err := s.repository.CreateOrUpdate(ctx, keeper)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to update keeper device %s: %v", keeper.IPv4, err))
			continue
		}
		
		// Delete duplicates
		for _, duplicate := range duplicates {
			log.Printf("Removing duplicate device %s (MAC: %s, keeping %s)", 
				duplicate.IPv4, mac, keeper.IPv4)
			
			err := s.repository.DeleteByID(ctx, duplicate.ID)
			if err != nil {
				errors = append(errors, fmt.Sprintf("Failed to delete duplicate device %s: %v", duplicate.IPv4, err))
				continue
			}
			deletedCount++
		}
	}
	
	if len(errors) > 0 {
		log.Printf("Duplicate cleanup completed with %d duplicates removed and %d errors", deletedCount, len(errors))
		for _, errMsg := range errors {
			log.Printf("Error: %s", errMsg)
		}
		return fmt.Errorf("cleanup completed with %d errors", len(errors))
	}
	
	log.Printf("Duplicate device cleanup completed successfully, removed %d duplicates", deletedCount)
	return nil
}
