package fingerprint

import (
	"context"
	"encoding/xml"
	"log"
	"os/exec"
	"reconya/models"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type FingerprintService struct{}

func NewFingerprintService() *FingerprintService {
	return &FingerprintService{}
}

// AnalyzeDevice performs comprehensive device fingerprinting
func (f *FingerprintService) AnalyzeDevice(device *models.Device) {
	log.Printf("Starting device fingerprinting for %s", device.IPv4)
	
	// 1. Vendor-based device type detection
	deviceType := f.detectDeviceTypeFromVendor(device.Vendor)
	if deviceType != models.DeviceTypeUnknown {
		device.DeviceType = deviceType
		log.Printf("Device type detected from vendor: %s", deviceType)
	}
	
	// 2. Port-based service detection
	if portBasedType := f.detectDeviceTypeFromPorts(device.Ports); portBasedType != models.DeviceTypeUnknown {
		if device.DeviceType == models.DeviceTypeUnknown {
			device.DeviceType = portBasedType
		}
		log.Printf("Device type detected from ports: %s", portBasedType)
	}
	
	// 3. Hostname-based detection
	if hostnameType := f.detectDeviceTypeFromHostname(device.Hostname); hostnameType != models.DeviceTypeUnknown {
		if device.DeviceType == models.DeviceTypeUnknown {
			device.DeviceType = hostnameType
		}
		log.Printf("Device type detected from hostname: %s", hostnameType)
	}
	
	// 4. Web service-based detection
	if webType := f.detectDeviceTypeFromWebServices(device.WebServices); webType != models.DeviceTypeUnknown {
		if device.DeviceType == models.DeviceTypeUnknown {
			device.DeviceType = webType
		}
		log.Printf("Device type detected from web services: %s", webType)
	}
	
	// 5. Nmap OS detection (more intensive)
	if osInfo := f.performNmapOSDetection(device.IPv4); osInfo != nil {
		device.OS = osInfo
		log.Printf("OS detected: %s %s (confidence: %d%%)", osInfo.Name, osInfo.Version, osInfo.Confidence)
		
		// Refine device type based on OS
		if osBasedType := f.detectDeviceTypeFromOS(osInfo); osBasedType != models.DeviceTypeUnknown {
			if device.DeviceType == models.DeviceTypeUnknown {
				device.DeviceType = osBasedType
			}
		}
	}
	
	// Set default if still unknown
	if device.DeviceType == models.DeviceTypeUnknown {
		device.DeviceType = models.DeviceTypeWorkstation // Default fallback
	}
	
	log.Printf("Final device fingerprint - Type: %s, OS: %v", device.DeviceType, device.OS)
}

// detectDeviceTypeFromVendor identifies device type based on MAC vendor
func (f *FingerprintService) detectDeviceTypeFromVendor(vendor *string) models.DeviceType {
	if vendor == nil || *vendor == "" {
		return models.DeviceTypeUnknown
	}
	
	vendorLower := strings.ToLower(*vendor)
	
	// Network Infrastructure
	if strings.Contains(vendorLower, "cisco") || strings.Contains(vendorLower, "juniper") ||
		strings.Contains(vendorLower, "netgear") || strings.Contains(vendorLower, "linksys") ||
		strings.Contains(vendorLower, "d-link") || strings.Contains(vendorLower, "tp-link") {
		return models.DeviceTypeRouter
	}
	
	// NAS and Storage
	if strings.Contains(vendorLower, "synology") || strings.Contains(vendorLower, "qnap") ||
		strings.Contains(vendorLower, "drobo") || strings.Contains(vendorLower, "netapp") ||
		strings.Contains(vendorLower, "seagate") {
		return models.DeviceTypeNAS
	}
	
	// Printers
	if strings.Contains(vendorLower, "hp") || strings.Contains(vendorLower, "canon") ||
		strings.Contains(vendorLower, "epson") || strings.Contains(vendorLower, "brother") ||
		strings.Contains(vendorLower, "lexmark") || strings.Contains(vendorLower, "xerox") {
		return models.DeviceTypePrinter
	}
	
	// Security Cameras
	if strings.Contains(vendorLower, "hikvision") || strings.Contains(vendorLower, "dahua") ||
		strings.Contains(vendorLower, "axis") || strings.Contains(vendorLower, "vivotek") ||
		strings.Contains(vendorLower, "foscam") {
		return models.DeviceTypeCamera
	}
	
	// Mobile Devices
	if strings.Contains(vendorLower, "apple") || strings.Contains(vendorLower, "samsung") ||
		strings.Contains(vendorLower, "lg") || strings.Contains(vendorLower, "sony") ||
		strings.Contains(vendorLower, "huawei") || strings.Contains(vendorLower, "xiaomi") {
		return models.DeviceTypeMobile
	}
	
	// Servers/Enterprise
	if strings.Contains(vendorLower, "dell") || strings.Contains(vendorLower, "ibm") ||
		strings.Contains(vendorLower, "supermicro") || strings.Contains(vendorLower, "intel") {
		return models.DeviceTypeServer
	}
	
	return models.DeviceTypeUnknown
}

// detectDeviceTypeFromPorts analyzes open ports to determine device type
func (f *FingerprintService) detectDeviceTypeFromPorts(ports []models.Port) models.DeviceType {
	if len(ports) == 0 {
		return models.DeviceTypeUnknown
	}
	
	portNumbers := make(map[int]bool)
	services := make(map[string]bool)
	
	for _, port := range ports {
		if port.State == "open" {
			if portNum, err := strconv.Atoi(port.Number); err == nil {
				portNumbers[portNum] = true
			}
			services[strings.ToLower(port.Service)] = true
		}
	}
	
	// Network Infrastructure
	if portNumbers[161] && portNumbers[23] { // SNMP + Telnet
		return models.DeviceTypeRouter
	}
	
	// NAS/File Servers
	if (portNumbers[139] || portNumbers[445]) && (portNumbers[548] || portNumbers[2049]) {
		return models.DeviceTypeNAS
	}
	
	// Web Servers
	if (portNumbers[80] || portNumbers[443]) && (portNumbers[21] || portNumbers[22]) {
		return models.DeviceTypeServer
	}
	
	// Printers
	if portNumbers[515] || portNumbers[631] || portNumbers[9100] {
		return models.DeviceTypePrinter
	}
	
	// Security Cameras
	if portNumbers[554] || portNumbers[8080] || services["rtsp"] {
		return models.DeviceTypeCamera
	}
	
	// VoIP
	if portNumbers[5060] || portNumbers[5061] || services["sip"] {
		return models.DeviceTypeVoIP
	}
	
	// SSH-only devices (likely servers/workstations)
	if portNumbers[22] && len(portNumbers) <= 3 {
		return models.DeviceTypeServer
	}
	
	return models.DeviceTypeUnknown
}

// detectDeviceTypeFromHostname analyzes hostname patterns
func (f *FingerprintService) detectDeviceTypeFromHostname(hostname *string) models.DeviceType {
	if hostname == nil || *hostname == "" {
		return models.DeviceTypeUnknown
	}
	
	hostnameLower := strings.ToLower(*hostname)
	
	// NAS patterns
	if strings.Contains(hostnameLower, "nas") || strings.Contains(hostnameLower, "synology") ||
		strings.Contains(hostnameLower, "diskstation") {
		return models.DeviceTypeNAS
	}
	
	// Router patterns
	if strings.Contains(hostnameLower, "router") || strings.Contains(hostnameLower, "gateway") ||
		strings.Contains(hostnameLower, "ap-") || strings.Contains(hostnameLower, "access-point") {
		return models.DeviceTypeRouter
	}
	
	// Printer patterns
	if strings.Contains(hostnameLower, "printer") || strings.Contains(hostnameLower, "print") ||
		strings.HasPrefix(hostnameLower, "hp-") || strings.HasPrefix(hostnameLower, "canon-") {
		return models.DeviceTypePrinter
	}
	
	// Camera patterns
	if strings.Contains(hostnameLower, "camera") || strings.Contains(hostnameLower, "cam") ||
		strings.Contains(hostnameLower, "ipcam") {
		return models.DeviceTypeCamera
	}
	
	// Server patterns
	if strings.Contains(hostnameLower, "server") || strings.Contains(hostnameLower, "srv") ||
		strings.Contains(hostnameLower, "web") || strings.Contains(hostnameLower, "db") {
		return models.DeviceTypeServer
	}
	
	return models.DeviceTypeUnknown
}

// detectDeviceTypeFromWebServices analyzes web service signatures
func (f *FingerprintService) detectDeviceTypeFromWebServices(webServices []models.WebService) models.DeviceType {
	for _, ws := range webServices {
		titleLower := strings.ToLower(ws.Title)
		serverLower := strings.ToLower(ws.Server)
		
		// NAS signatures
		if strings.Contains(titleLower, "synology") || strings.Contains(titleLower, "diskstation") ||
			strings.Contains(titleLower, "qnap") || strings.Contains(titleLower, "nas") {
			return models.DeviceTypeNAS
		}
		
		// Router/AP signatures
		if strings.Contains(titleLower, "router") || strings.Contains(titleLower, "access point") ||
			strings.Contains(titleLower, "wireless") || strings.Contains(serverLower, "lighttpd") {
			return models.DeviceTypeRouter
		}
		
		// Printer signatures
		if strings.Contains(titleLower, "printer") || strings.Contains(titleLower, "print server") ||
			strings.Contains(titleLower, "cups") {
			return models.DeviceTypePrinter
		}
		
		// Camera signatures
		if strings.Contains(titleLower, "ip camera") || strings.Contains(titleLower, "webcam") ||
			strings.Contains(titleLower, "surveillance") {
			return models.DeviceTypeCamera
		}
	}
	
	return models.DeviceTypeUnknown
}

// detectDeviceTypeFromOS determines device type based on OS information
func (f *FingerprintService) detectDeviceTypeFromOS(os *models.DeviceOS) models.DeviceType {
	if os == nil || os.Name == "" {
		return models.DeviceTypeUnknown
	}
	
	osNameLower := strings.ToLower(os.Name)
	
	// Server OS
	if strings.Contains(osNameLower, "windows server") || strings.Contains(osNameLower, "ubuntu server") ||
		strings.Contains(osNameLower, "centos") || strings.Contains(osNameLower, "rhel") ||
		strings.Contains(osNameLower, "debian") && strings.Contains(osNameLower, "server") {
		return models.DeviceTypeServer
	}
	
	// Router/Embedded OS
	if strings.Contains(osNameLower, "linux") && (strings.Contains(osNameLower, "embedded") ||
		strings.Contains(osNameLower, "openwrt") || strings.Contains(osNameLower, "dd-wrt")) {
		return models.DeviceTypeRouter
	}
	
	// Mobile OS
	if strings.Contains(osNameLower, "ios") || strings.Contains(osNameLower, "android") {
		return models.DeviceTypeMobile
	}
	
	// Desktop OS
	if strings.Contains(osNameLower, "windows") && !strings.Contains(osNameLower, "server") ||
		strings.Contains(osNameLower, "mac os") || strings.Contains(osNameLower, "ubuntu") {
		return models.DeviceTypeWorkstation
	}
	
	return models.DeviceTypeUnknown
}

// performNmapOSDetection runs nmap OS detection
func (f *FingerprintService) performNmapOSDetection(ipv4 string) *models.DeviceOS {
	log.Printf("Performing nmap OS detection for %s", ipv4)
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	// Run nmap OS detection
	cmd := exec.CommandContext(ctx, "nmap", "-O", "-sT", "--osscan-guess", "-oX", "-", ipv4)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("nmap OS detection failed for %s: %v", ipv4, err)
		return nil
	}
	
	return f.parseNmapOSOutput(string(output))
}

// parseNmapOSOutput parses nmap XML output for OS information
func (f *FingerprintService) parseNmapOSOutput(xmlOutput string) *models.DeviceOS {
	var nmapXML struct {
		XMLName xml.Name `xml:"nmaprun"`
		Hosts   []struct {
			OS []struct {
				OSMatches []struct {
					Name     string `xml:"name,attr"`
					Accuracy string `xml:"accuracy,attr"`
					OSClass  []struct {
						Vendor   string `xml:"vendor,attr"`
						OSFamily string `xml:"osfamily,attr"`
						OSGen    string `xml:"osgen,attr"`
					} `xml:"osclass"`
				} `xml:"osmatch"`
			} `xml:"os"`
		} `xml:"host"`
	}
	
	err := xml.Unmarshal([]byte(xmlOutput), &nmapXML)
	if err != nil {
		log.Printf("Error parsing nmap OS XML: %v", err)
		return nil
	}
	
	for _, host := range nmapXML.Hosts {
		for _, os := range host.OS {
			if len(os.OSMatches) > 0 {
				match := os.OSMatches[0] // Take the best match
				
				confidence, _ := strconv.Atoi(match.Accuracy)
				
				osInfo := &models.DeviceOS{
					Name:       match.Name,
					Confidence: confidence,
				}
				
				// Extract version and family from name
				if version := f.extractVersionFromOSName(match.Name); version != "" {
					osInfo.Version = version
				}
				
				if len(match.OSClass) > 0 {
					osInfo.Family = match.OSClass[0].OSFamily
				}
				
				return osInfo
			}
		}
	}
	
	return nil
}

// extractVersionFromOSName tries to extract version number from OS name
func (f *FingerprintService) extractVersionFromOSName(osName string) string {
	// Common version patterns
	patterns := []string{
		`\b(\d+\.\d+(?:\.\d+)?)\b`,     // 10.15.7, 8.1, etc.
		`\b(Windows \d+)\b`,            // Windows 10, Windows 11
		`\b(Ubuntu \d+\.\d+)\b`,        // Ubuntu 20.04
		`\b(CentOS \d+)\b`,             // CentOS 7
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(osName); len(matches) > 1 {
			return matches[1]
		}
	}
	
	return ""
}