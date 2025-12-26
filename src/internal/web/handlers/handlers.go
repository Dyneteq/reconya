package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"reconya/db"
	"reconya/internal/config"
	"reconya/internal/device"
	"reconya/internal/eventlog"
	"reconya/internal/network"
	"reconya/internal/nicidentifier"
	"reconya/internal/scan"
	"reconya/internal/sensor"
	"reconya/internal/settings"
	"reconya/internal/systemstatus"
	"reconya/models"

	"github.com/gorilla/sessions"
)

// Templates will be loaded from filesystem for now
// TODO: Embed templates in production build

// findPath searches for a path in multiple locations and returns the first one found
func findPath(name string) string {
	paths := []string{
		name,          // Current directory (running from src)
		"src/" + name, // From project root
		"../" + name,  // Parent directory
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return name // Return default if not found
}

type WebHandler struct {
	deviceService         *device.DeviceService
	eventLogService       *eventlog.EventLogService
	networkService        *network.NetworkService
	systemStatusService   *systemstatus.SystemStatusService
	scanManager           *scan.ScanManager
	geolocationRepository *db.GeolocationRepository
	settingsService       *settings.SettingsService
	sensorService         *sensor.SensorService
	nicIdentifierService  *nicidentifier.NicIdentifierService
	templates             *template.Template
	sessionStore          *sessions.CookieStore
	config                *config.Config
}

type PageData struct {
	Page             string
	User             *models.User
	Error            string
	Username         string
	Devices          []*models.Device
	EventLogs        []*models.EventLog
	SystemStatusData *SystemStatusTemplateData // Use the new struct for system status
	NetworkMap       *NetworkMapData
	Networks         []models.Network
	ScanState        *scan.ScanState
}

type NetworkMapData struct {
	BaseIP      string
	IPRange     []int
	Devices     map[string]*models.Device
	NetworkInfo *NetworkInfo
}

type NetworkInfo struct {
	OnlineDevices  int
	IdleDevices    int
	OfflineDevices int
}

// SystemStatusTemplateData holds system status data
type SystemStatusTemplateData struct {
	SystemStatus *models.SystemStatus
	NetworkCIDR  string
	NetworkInfo  *NetworkInfo
	DevicesCount int
	ScanState    *scan.ScanState
}

func NewWebHandler(
	deviceService *device.DeviceService,
	eventLogService *eventlog.EventLogService,
	networkService *network.NetworkService,
	systemStatusService *systemstatus.SystemStatusService,
	scanManager *scan.ScanManager,
	geolocationRepository *db.GeolocationRepository,
	settingsService *settings.SettingsService,
	sensorService *sensor.SensorService,
	nicIdentifierService *nicidentifier.NicIdentifierService,
	config *config.Config,
	sessionSecret string,
) *WebHandler {
	// Initialize template functions
	funcMap := template.FuncMap{
		"formatTime": func(t time.Time) string {
			if t.IsZero() {
				return "Never"
			}
			return t.Format("2006-01-02 15:04:05")
		},
		"formatTimeAgo": func(t time.Time) string {
			if t.IsZero() {
				return "Never"
			}
			duration := time.Since(t)
			switch {
			case duration < time.Minute:
				return fmt.Sprintf("%ds ago", int(duration.Seconds()))
			case duration < time.Hour:
				return fmt.Sprintf("%dm ago", int(duration.Minutes()))
			case duration < 24*time.Hour:
				return fmt.Sprintf("%dh ago", int(duration.Hours()))
			default:
				return fmt.Sprintf("%dd ago", int(duration.Hours()/24))
			}
		},
		"formatFileSize": func(bytes interface{}) string {
			var size float64
			switch v := bytes.(type) {
			case int:
				size = float64(v)
			case int64:
				size = float64(v)
			case float64:
				size = v
			default:
				return "N/A"
			}

			if size == 0 {
				return "N/A"
			}

			kb := size / 1024
			if kb < 1024 {
				return fmt.Sprintf("%.1f KB", kb)
			}
			mb := kb / 1024
			return fmt.Sprintf("%.1f MB", mb)
		},
		"upper": func(s string) string {
			return strings.ToUpper(s)
		},
		"deref": func(ptr interface{}) interface{} {
			if ptr == nil {
				return "-"
			}
			switch v := ptr.(type) {
			case *string:
				if v == nil {
					return "-"
				}
				return *v
			case *time.Time:
				if v == nil {
					return time.Time{}
				}
				return *v
			default:
				return ptr
			}
		},
		"formatEventType": func(eventType string) string {
			return strings.ReplaceAll(strings.Title(strings.ReplaceAll(eventType, "_", " ")), "_", " ")
		},
		"slice": func(items interface{}, start, end int) interface{} {
			switch v := items.(type) {
			case []*models.Port:
				if start >= len(v) {
					return []*models.Port{}
				}
				if end > len(v) {
					end = len(v)
				}
				return v[start:end]
			}
			return items
		},
		"eq": func(a, b interface{}) bool {
			return a == b
		},
		"len": func(items interface{}) int {
			switch v := items.(type) {
			case []*models.Device:
				return len(v)
			case []*models.Port:
				return len(v)
			case []*models.WebService:
				return len(v)
			case []*models.EventLog:
				return len(v)
			}
			return 0
		},
		"or": func(args ...interface{}) interface{} {
			for _, arg := range args {
				if arg != nil && arg != "" {
					return arg
				}
			}
			if len(args) > 0 {
				return args[len(args)-1]
			}
			return nil
		},
		"where": func(slice interface{}, field, value string) interface{} {
			switch v := slice.(type) {
			case []*models.Device:
				var result []*models.Device
				for _, item := range v {
					var fieldValue string
					switch field {
					case "Status":
						fieldValue = string(item.Status)
					case "IPv4":
						fieldValue = item.IPv4
					}
					if fieldValue == value {
						result = append(result, item)
					}
				}
				return result
			}
			return slice
		},
		"split": func(s, sep string) []string {
			return strings.Split(s, sep)
		},
		"last": func(slice []string) string {
			if len(slice) == 0 {
				return ""
			}
			return slice[len(slice)-1]
		},
		"add": func(a, b interface{}) interface{} {
			switch av := a.(type) {
			case int:
				if bv, ok := b.(int); ok {
					return av + bv
				}
				if bv, ok := b.(float64); ok {
					return float64(av) + bv
				}
			case float64:
				if bv, ok := b.(float64); ok {
					return av + bv
				}
				if bv, ok := b.(int); ok {
					return av + float64(bv)
				}
			}
			return a
		},
		"mul": func(a, b interface{}) interface{} {
			switch av := a.(type) {
			case int:
				if bv, ok := b.(int); ok {
					return av * bv
				}
				if bv, ok := b.(float64); ok {
					return float64(av) * bv
				}
			case float64:
				if bv, ok := b.(float64); ok {
					return av * bv
				}
				if bv, ok := b.(int); ok {
					return av * float64(bv)
				}
			}
			return a
		},
		"div": func(a, b interface{}) interface{} {
			switch av := a.(type) {
			case int:
				if bv, ok := b.(int); ok {
					if bv == 0 {
						return 0
					}
					return av / bv
				}
				if bv, ok := b.(float64); ok {
					if bv == 0 {
						return 0.0
					}
					return float64(av) / bv
				}
			case float64:
				if bv, ok := b.(float64); ok {
					if bv == 0 {
						return 0.0
					}
					return av / bv
				}
				if bv, ok := b.(int); ok {
					if bv == 0 {
						return 0.0
					}
					return av / float64(bv)
				}
			}
			return a
		},
		"sub": func(a, b interface{}) interface{} {
			switch av := a.(type) {
			case int:
				if bv, ok := b.(int); ok {
					return av - bv
				}
				if bv, ok := b.(float64); ok {
					return float64(av) - bv
				}
			case float64:
				if bv, ok := b.(float64); ok {
					return av - bv
				}
				if bv, ok := b.(int); ok {
					return av - float64(bv)
				}
			}
			return a
		},
		"cos": func(angle float64) float64 {
			return math.Cos(angle)
		},
		"sin": func(angle float64) float64 {
			return math.Sin(angle)
		},
		"replace": func(s, old, new string) string {
			return strings.ReplaceAll(s, old, new)
		},
		"contains": func(s, substr string) bool {
			return strings.Contains(s, substr)
		},
		"string": func(v interface{}) string {
			switch val := v.(type) {
			case models.DeviceType:
				return string(val)
			case models.DeviceStatus:
				return string(val)
			case string:
				return val
			default:
				return fmt.Sprintf("%v", val)
			}
		},
	}

	// Parse templates from filesystem
	tmpl := template.New("").Funcs(funcMap)

	// Find templates directory (works from project root or src directory)
	templatesDir := findPath("templates")

	// Parse templates with unique names to avoid conflicts
	baseFiles, err := filepath.Glob(filepath.Join(templatesDir, "layouts", "*.html"))
	if err != nil {
		panic(fmt.Sprintf("Failed to glob base templates: %v", err))
	}

	componentFiles, err := filepath.Glob(filepath.Join(templatesDir, "components", "*.html"))
	if err != nil {
		panic(fmt.Sprintf("Failed to glob component templates: %v", err))
	}

	indexFile := filepath.Join(templatesDir, "index.html")

	files := append(baseFiles, componentFiles...)
	files = append(files, indexFile)
	log.Printf("Found template files: %v", files)

	if len(files) == 0 {
		panic("No template files found")
	}

	tmpl, err = tmpl.ParseFiles(files...)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse templates: %v", err))
	}

	// Log template names for debugging
	for _, t := range tmpl.Templates() {
		log.Printf("Loaded template: %s", t.Name())
	}

	// Debug: Try to find login.html specifically
	loginTmpl := tmpl.Lookup("login.html")
	if loginTmpl != nil {
		log.Printf("Found login.html template: %s", loginTmpl.Name())
	} else {
		log.Printf("ERROR: login.html template not found!")
	}

	store := sessions.NewCookieStore([]byte(sessionSecret))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 30, // 30 days
		HttpOnly: true,
		Secure:   false, // Set to false for HTTP (localhost development)
		SameSite: http.SameSiteStrictMode,
	}

	return &WebHandler{
		deviceService:         deviceService,
		eventLogService:       eventLogService,
		networkService:        networkService,
		systemStatusService:   systemStatusService,
		scanManager:           scanManager,
		geolocationRepository: geolocationRepository,
		settingsService:       settingsService,
		sensorService:         sensorService,
		nicIdentifierService:  nicIdentifierService,
		templates:             tmpl,
		sessionStore:          store,
		config:                config,
	}
}

// Page handlers are in handlers_pages.go

// Device API handlers are in handlers_devices.go

func (h *WebHandler) APISystemStatus(w http.ResponseWriter, r *http.Request) {
	log.Println("APISystemStatus called")
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get system status from service
	status, err := h.systemStatusService.GetLatest()
	if err != nil {
		log.Printf("Error getting system status: %v", err)
		// If service fails, create mock data for now
		status = &models.SystemStatus{
			NetworkID: "N/A",
			PublicIP:  nil,
		}
	} else if status == nil {
		log.Printf("No system status found in database, using fallback")
		// If no system status exists yet, create mock data
		status = &models.SystemStatus{
			NetworkID: "N/A",
			PublicIP:  nil,
		}
	} else {
		log.Printf("SystemStatus found: NetworkID=%s", status.NetworkID)
	}

	// Get current or selected network to determine which network to show
	currentNetwork := h.scanManager.GetSelectedOrCurrentNetwork()
	scanState := h.scanManager.GetState()
	var devices []*models.Device
	var networkCIDR string = "N/A"

	if currentNetwork != nil {
		log.Printf("APISystemStatus: currentNetwork is not nil, ID: %s", currentNetwork.ID)
		// Show devices from the currently selected/scanning network
		devicesSlice, err := h.deviceService.FindByNetworkID(currentNetwork.ID)
		if err != nil {
			log.Printf("Error getting devices for system status %s: %v", currentNetwork.ID, err)
			devices = []*models.Device{}
		} else {
			// Convert []models.Device to []*models.Device
			devices = make([]*models.Device, len(devicesSlice))
			for i := range devicesSlice {
				devices[i] = &devicesSlice[i]
			}
		}
		networkCIDR = currentNetwork.CIDR
	} else {
		log.Println("APISystemStatus: currentNetwork is nil, falling back to all devices")
		// If no network is selected, show all devices
		devices, err = h.deviceService.FindAll()
		if err != nil {
			log.Printf("Error getting all devices for system status: %v", err)
			devices = []*models.Device{}
		}
	}

	networkMapData := h.buildNetworkMap(devices)

	data := SystemStatusTemplateData{
		SystemStatus: status,
		NetworkCIDR:  networkCIDR,
		NetworkInfo:  networkMapData.NetworkInfo,
		DevicesCount: len(devices),
		ScanState:    &scanState,
	}

	log.Printf("APISystemStatus: returning data: %+v", data)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding system status JSON: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *WebHandler) APIEventLogs(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get recent event logs
	eventLogSlice, err := h.eventLogService.GetAll(20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to pointer slice
	eventLogs := make([]*models.EventLog, len(eventLogSlice))
	for i := range eventLogSlice {
		eventLogs[i] = &eventLogSlice[i]
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"logs": eventLogs,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *WebHandler) APIEventLogsTable(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get more event logs for the table view (100 instead of 20)
	eventLogSlice, err := h.eventLogService.GetAll(100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to pointer slice
	eventLogs := make([]*models.EventLog, len(eventLogSlice))
	for i := range eventLogSlice {
		eventLogs[i] = &eventLogSlice[i]
	}

	data := struct {
		EventLogs []*models.EventLog `json:"eventLogs"`
	}{
		EventLogs: eventLogs,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *WebHandler) APINetworkMap(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	// Find the first network that has devices to display in the map
	networks, err := h.networkService.FindAll()
	if err != nil {
		log.Printf("Error getting networks for network map: %v", err)
		networks = []models.Network{}
	}

	var devicesSlice []models.Device
	var selectedNetwork *models.Network

	// Find first network with devices
	for i := range networks {
		netDevices, err := h.deviceService.FindByNetworkID(networks[i].ID)
		if err == nil && len(netDevices) > 0 {
			devicesSlice = netDevices
			selectedNetwork = &networks[i]
			break
		}
	}

	devices := make([]*models.Device, len(devicesSlice))
	for i := range devicesSlice {
		devices[i] = &devicesSlice[i]
	}

	networkMap := h.buildNetworkMapForNetwork(devices, selectedNetwork)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(networkMap); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Helper methods
func (h *WebHandler) getUserFromSession(session *sessions.Session) *models.User {
	if userID, ok := session.Values["user_id"].(string); ok {
		return &models.User{
			Username: userID,
		}
	}
	return nil
}

func (h *WebHandler) authenticate(username, password string) bool {
	// Simple authentication - replace with your logic
	return username == "admin" && password == "password"
}

func (h *WebHandler) buildNetworkMap(devices []*models.Device) *NetworkMapData {
	// Build device map by IP
	deviceMap := make(map[string]*models.Device)
	online, idle, offline := 0, 0, 0

	for _, device := range devices {
		deviceMap[device.IPv4] = device
		switch device.Status {
		case models.DeviceStatusOnline:
			online++
		case models.DeviceStatusIdle:
			idle++
		default:
			offline++
		}
	}

	// Parse network CIDR from current scan network
	var baseIP string
	var ipRange []int
	currentNetwork := h.scanManager.GetCurrentNetwork()
	if currentNetwork != nil {
		baseIP, ipRange = h.parseNetworkCIDR(currentNetwork.CIDR)
	} else {
		// Fallback if no network is selected
		baseIP = "192.168.1"
		ipRange = make([]int, 254)
		for i := range ipRange {
			ipRange[i] = i + 1
		}
	}

	return &NetworkMapData{
		BaseIP:  baseIP,
		IPRange: ipRange,
		Devices: deviceMap,
		NetworkInfo: &NetworkInfo{
			OnlineDevices:  online + idle, // Count both online and idle as "online" for dashboard
			IdleDevices:    idle,
			OfflineDevices: offline,
		},
	}
}

func (h *WebHandler) buildNetworkMapForNetwork(devices []*models.Device, network *models.Network) *NetworkMapData {
	// Build device map by IP
	deviceMap := make(map[string]*models.Device)
	online, idle, offline := 0, 0, 0

	for _, device := range devices {
		deviceMap[device.IPv4] = device
		switch device.Status {
		case models.DeviceStatusOnline:
			online++
		case models.DeviceStatusIdle:
			idle++
		default:
			offline++
		}
	}

	// Parse network CIDR from the provided network
	var baseIP string
	var ipRange []int
	if network != nil && network.CIDR != "" {
		baseIP, ipRange = h.parseNetworkCIDR(network.CIDR)
	} else {
		// Fallback if no network provided
		baseIP = "192.168.1"
		ipRange = make([]int, 254)
		for i := range ipRange {
			ipRange[i] = i + 1
		}
	}

	return &NetworkMapData{
		BaseIP:  baseIP,
		IPRange: ipRange,
		Devices: deviceMap,
		NetworkInfo: &NetworkInfo{
			OnlineDevices:  online + idle,
			IdleDevices:    idle,
			OfflineDevices: offline,
		},
	}
}

// parseNetworkCIDR parses a CIDR string and returns base IP and host range
func (h *WebHandler) parseNetworkCIDR(cidr string) (string, []int) {
	// Default fallback
	defaultBaseIP := "192.168.1"
	defaultRange := make([]int, 254)
	for i := 1; i <= 254; i++ {
		defaultRange[i-1] = i
	}

	if cidr == "" {
		return defaultBaseIP, defaultRange
	}

	// Parse CIDR
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		log.Printf("Error parsing CIDR %s: %v", cidr, err)
		return defaultBaseIP, defaultRange
	}

	// Get network address
	networkIP := ipNet.IP

	// Calculate subnet mask bits
	ones, bits := ipNet.Mask.Size()
	if bits != 32 {
		log.Printf("Invalid network mask in CIDR %s", cidr)
		return defaultBaseIP, defaultRange
	}

	// Calculate number of host addresses
	hostBits := bits - ones
	totalHosts := 1 << hostBits // 2^hostBits

	// Subtract network and broadcast addresses
	usableHosts := totalHosts - 2
	if usableHosts <= 0 {
		usableHosts = 1
	}

	// Generate base IP (network portion)
	parts := strings.Split(networkIP.String(), ".")
	if len(parts) < 3 {
		return defaultBaseIP, defaultRange
	}

	// For /23 networks (like 192.168.10.0/23), we need to handle the range properly
	// For /24 networks, it's simpler
	var baseIP string
	var ipRange []int

	if ones >= 24 {
		// /24 or smaller subnet - use the first 3 octets as base
		baseIP = strings.Join(parts[:3], ".")
		// Generate host range for the last octet
		maxHosts := usableHosts
		if maxHosts > 254 {
			maxHosts = 254
		}
		ipRange = make([]int, maxHosts)
		for i := 1; i <= maxHosts; i++ {
			ipRange[i-1] = i
		}
	} else {
		// Larger subnet (like /23) - more complex range calculation
		baseIP = strings.Join(parts[:3], ".")

		// For /23, we have 512 addresses total, 510 usable
		// This spans two /24 networks (e.g., 192.168.10.0-192.168.11.255)
		maxHosts := usableHosts
		if maxHosts > 510 {
			maxHosts = 510
		}

		// Generate a reasonable range for visualization (limit to avoid UI issues)
		visualHosts := maxHosts
		if visualHosts > 254 {
			visualHosts = 254
		}

		ipRange = make([]int, visualHosts)
		for i := 1; i <= visualHosts; i++ {
			ipRange[i-1] = i
		}
	}

	return baseIP, ipRange
}

func (h *WebHandler) APITargets(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Same as APIDevices - targets are devices
	devices, err := h.deviceService.FindAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get user's screenshot setting
	screenshotsEnabled := h.settingsService.AreScreenshotsEnabled(fmt.Sprintf("%d", user.ID))

	viewMode := r.URL.Query().Get("view")
	data := struct {
		Devices            []*models.Device `json:"devices"`
		ViewMode           string           `json:"viewMode"`
		ScreenshotsEnabled bool             `json:"screenshotsEnabled"`
	}{
		Devices:            devices,
		ViewMode:           viewMode,
		ScreenshotsEnabled: screenshotsEnabled,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *WebHandler) APITrafficCore(w http.ResponseWriter, r *http.Request) {
	devices, err := h.deviceService.FindAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Devices []*models.Device `json:"devices"`
	}{
		Devices: devices,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Network API handlers are in handlers_networks.go

// Scan handlers are in handlers_scan.go

// APIAbout returns the about page content for the SPA
func (h *WebHandler) APIAbout(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Read version from package.json
	version := h.getVersionFromPackageJSON()

	response := map[string]interface{}{
		"version": version,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("About component JSON encoding error: %v", err)
		http.Error(w, fmt.Sprintf("JSON encoding error: %v", err), http.StatusInternalServerError)
	}
}

// APISettings returns the settings page
func (h *WebHandler) APISettings(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get user settings
	settings, err := h.settingsService.GetUserSettings(fmt.Sprintf("%d", user.ID))
	if err != nil {
		log.Printf("Error getting user settings: %v", err)
		http.Error(w, "Failed to load settings", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"settings": settings,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding settings JSON: %v", err)
		http.Error(w, "Failed to encode settings", http.StatusInternalServerError)
		return
	}
}

// APISettingsScreenshots handles screenshot settings updates
func (h *WebHandler) APISettingsScreenshots(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Unauthorized",
		})
		return
	}

	// Parse the enabled parameter - checkbox sends value when checked, nothing when unchecked
	enabledStr := r.FormValue("screenshots_enabled")
	enabled := enabledStr == "true" || enabledStr == "on"

	log.Printf("Screenshot settings update: enabled=%s, parsed=%v", enabledStr, enabled)

	// Update settings
	updates := map[string]interface{}{
		"screenshots_enabled": enabled,
	}

	_, err := h.settingsService.UpdateUserSettings(fmt.Sprintf("%d", user.ID), updates)
	if err != nil {
		log.Printf("Error updating screenshot settings: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to update settings",
		})
		return
	}

	log.Printf("Updated screenshot settings for user %d: enabled=%v", user.ID, enabled)

	// Return JSON success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Settings updated successfully",
	})
}

// Network detection handlers are in handlers_networks.go

// getVersionFromPackageJSON reads the version from package.json
func (h *WebHandler) getVersionFromPackageJSON() string {
	// Try to read package.json from the project root
	packageJSONPath := filepath.Join("..", "package.json")

	// If that doesn't work, try relative to the binary location
	if _, err := os.Stat(packageJSONPath); os.IsNotExist(err) {
		packageJSONPath = "package.json"
	}

	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		log.Printf("Error reading package.json: %v", err)
		return "unknown"
	}

	var packageInfo struct {
		Version string `json:"version"`
	}

	if err := json.Unmarshal(data, &packageInfo); err != nil {
		log.Printf("Error parsing package.json: %v", err)
		return "unknown"
	}

	return packageInfo.Version
}

// Sensor handlers are in handlers_sensors.go
