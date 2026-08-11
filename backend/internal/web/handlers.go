package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"reconya/db"
	"reconya/internal/alert"
	"reconya/internal/config"
	"reconya/internal/device"
	"reconya/internal/eventlog"
	"reconya/internal/network"
	"reconya/internal/nicidentifier"
	"reconya/internal/portscan"
	"reconya/internal/scan"
	"reconya/internal/settings"
	"reconya/internal/systemstatus"
	"reconya/models"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

type WebHandler struct {
	deviceService         *device.DeviceService
	eventLogService       *eventlog.EventLogService
	networkService        *network.NetworkService
	systemStatusService   *systemstatus.SystemStatusService
	scanManager           *scan.ScanManager
	geolocationRepository *db.GeolocationRepository
	settingsService       *settings.SettingsService
	nicIdentifierService  *nicidentifier.NicIdentifierService
	alertService          *alert.AlertService
	portScanService       *portscan.PortScanService
	templates             *template.Template
	templateFS            fs.FS
	staticFS              fs.FS
	version               string
	sessionStore          *sessions.CookieStore
	config                *config.Config
}

// PageData is everything index.html reads. The console fetches its data over
// the JSON API, so the shell only needs to know which page it is rendering and
// who is logged in; login.html adds the two error fields.
type PageData struct {
	Page     string
	User     *models.User
	Error    string
	Username string
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

func NewWebHandler(
	deviceService *device.DeviceService,
	eventLogService *eventlog.EventLogService,
	networkService *network.NetworkService,
	systemStatusService *systemstatus.SystemStatusService,
	scanManager *scan.ScanManager,
	geolocationRepository *db.GeolocationRepository,
	settingsService *settings.SettingsService,
	nicIdentifierService *nicidentifier.NicIdentifierService,
	alertService *alert.AlertService,
	portScanService *portscan.PortScanService,
	config *config.Config,
	sessionSecret string,
	templateFS fs.FS,
	staticFS fs.FS,
	version string,
) *WebHandler {
	funcMap := templateFuncMap()

	// Parse templates from embedded filesystem
	tmpl := template.New("").Funcs(funcMap)

	allFiles := []string{"index.html"}

	standaloneFiles, err := fs.Glob(templateFS, "standalone/*.html")
	if err != nil {
		panic(fmt.Sprintf("Failed to glob standalone templates: %v", err))
	}
	allFiles = append(allFiles, standaloneFiles...)

	log.Printf("Found embedded template files: %v", allFiles)

	if len(allFiles) == 0 {
		panic("No embedded template files found")
	}

	tmpl, err = tmpl.ParseFS(templateFS, allFiles...)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse embedded templates: %v", err))
	}

	for _, t := range tmpl.Templates() {
		log.Printf("Loaded template: %s", t.Name())
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
		nicIdentifierService:  nicIdentifierService,
		alertService:          alertService,
		portScanService:       portScanService,
		templates:             tmpl,
		templateFS:            templateFS,
		staticFS:              staticFS,
		version:               version,
		sessionStore:          store,
		config:                config,
	}
}

// Page Handlers
// ServePage serves the main application page with the specified page context
func (h *WebHandler) ServePage(pageName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := h.sessionStore.Get(r, "reconya-session")
		user := h.getUserFromSession(session)
		if user == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Map root path to dashboard
		if pageName == "" {
			pageName = "dashboard"
		}

		data := PageData{
			Page: pageName,
			User: user,
		}

		if err := h.templates.ExecuteTemplate(w, "index.html", data); err != nil {
			log.Printf("%s template execution error: %v", pageName, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// Legacy handlers for backward compatibility
func (h *WebHandler) Index(w http.ResponseWriter, r *http.Request) {
	h.ServePage("dashboard")(w, r)
}

func (h *WebHandler) Devices(w http.ResponseWriter, r *http.Request) {
	h.ServePage("devices")(w, r)
}

func (h *WebHandler) Networks(w http.ResponseWriter, r *http.Request) {
	h.ServePage("networks")(w, r)
}

func (h *WebHandler) Logs(w http.ResponseWriter, r *http.Request) {
	h.ServePage("logs")(w, r)
}

func (h *WebHandler) Alerts(w http.ResponseWriter, r *http.Request) {
	h.ServePage("alerts")(w, r)
}

func (h *WebHandler) Settings(w http.ResponseWriter, r *http.Request) {
	h.ServePage("settings")(w, r)
}

func (h *WebHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// Use standalone login template to avoid conflicts
		loginTmpl, err := template.ParseFS(h.templateFS, "standalone/login.html")
		if err != nil {
			log.Printf("Failed to parse standalone login template: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := struct {
			Page     string
			Error    string
			Username string
		}{
			Page:     "login",
			Error:    "",
			Username: "",
		}
		if err := loginTmpl.Execute(w, data); err != nil {
			log.Printf("Template execution error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Handle POST login
	username := r.FormValue("username")
	password := r.FormValue("password")

	// Simple authentication (replace with your auth logic)
	if h.authenticate(username, password) {
		session, _ := h.sessionStore.Get(r, "reconya-session")
		session.Values["user_id"] = username
		session.Values["username"] = username
		session.Save(r, w)

		// Redirect to home page after successful login
		http.Redirect(w, r, "/", http.StatusSeeOther)
	} else {
		data := struct {
			Page     string
			Error    string
			Username string
		}{
			Page:     "login",
			Error:    "Invalid username or password",
			Username: username,
		}
		if err := h.templates.ExecuteTemplate(w, "login.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (h *WebHandler) Logout(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	session.Values = make(map[interface{}]interface{})
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *WebHandler) APIDeviceModal(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	deviceID := vars["id"]

	device, err := h.deviceService.FindByID(deviceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if device == nil {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}

	// Get user's screenshot setting
	screenshotsEnabled := h.settingsService.AreScreenshotsEnabled(fmt.Sprintf("%d", user.ID))

	// Debug logging for IPv6 fields
	log.Printf("Device %s IPv6 data: LinkLocal=%v, UniqueLocal=%v, Global=%v, Addresses=%v",
		device.ID, device.IPv6LinkLocal, device.IPv6UniqueLocal, device.IPv6Global, device.IPv6Addresses)

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"device":             device,
		"screenshotsEnabled": screenshotsEnabled,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *WebHandler) APIUpdateDevice(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	deviceID := vars["id"]

	// Parse JSON body. IsFavorite/Ignored/Addressing are pointers so a field
	// absent from the request (nil) is distinguishable from an explicit
	// false/"" value, unlike the empty-string-means-unset convention used
	// for Name/Comment below.
	var data struct {
		Name       string  `json:"name"`
		Comment    string  `json:"comment"`
		IsFavorite *bool   `json:"is_favorite"`
		Ignored    *bool   `json:"ignored"`
		Addressing *string `json:"addressing"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("Updating device %s: name='%s', comment='%s'", deviceID, data.Name, data.Comment)

	var namePtr, commentPtr *string
	if data.Name != "" {
		namePtr = &data.Name
	}
	if data.Comment != "" {
		commentPtr = &data.Comment
	}

	var addressingPtr *models.Addressing
	if data.Addressing != nil {
		switch models.Addressing(*data.Addressing) {
		case models.AddressingUnknown, models.AddressingStatic, models.AddressingDHCP:
			addressing := models.Addressing(*data.Addressing)
			addressingPtr = &addressing
		default:
			http.Error(w, "Invalid addressing value: must be \"\", \"static\", or \"dhcp\"", http.StatusBadRequest)
			return
		}
	}

	device, err := h.deviceService.UpdateDevice(deviceID, namePtr, commentPtr, data.IsFavorite, data.Ignored, addressingPtr)
	if err != nil {
		log.Printf("Failed to update device %s: %v", deviceID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully updated device %s", deviceID)

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]any{
		"success": true,
		"device":  device,
		"message": "Device updated successfully",
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *WebHandler) APIDeleteDevice(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	deviceID := vars["id"]

	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get device info before deletion for logging
	device, err := h.deviceService.FindByID(deviceID)
	if err != nil {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}

	if device == nil {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}

	// Delete the device
	err = h.deviceService.Delete(deviceID)
	if err != nil {
		log.Printf("Failed to delete device %s: %v", deviceID, err)
		http.Error(w, fmt.Sprintf("Failed to delete device: %v", err), http.StatusInternalServerError)
		return
	}

	// Log the event
	h.eventLogService.Log(models.DeviceDeleted, fmt.Sprintf("Device %s deleted", device.IPv4), "")

	log.Printf("Successfully deleted device %s (%s)", device.IPv4, deviceID)

	// Return empty response to remove the table row
	w.WriteHeader(http.StatusOK)
}

func (h *WebHandler) APIPingInternet(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	if user := h.getUserFromSession(session); user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	conn, err := net.DialTimeout("tcp", "1.1.1.1:80", 2*time.Second)
	up := err == nil
	if up {
		conn.Close()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"up": up})
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
	return username == h.config.Username && password == h.config.Password
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

func (h *WebHandler) APIDeviceList(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Unauthorized",
			"success": false,
		})
		return
	}

	devices, err := h.deviceService.FindAll()
	if err != nil {
		log.Printf("Failed to get devices: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to retrieve devices",
			"success": false,
		})
		return
	}

	// Scope to the active network on the server. Device rows accumulate across
	// every network the host has ever been attached to, and those stay useful
	// when roaming — but sending all of them and filtering in the browser leaks
	// other networks' inventory into every response.
	if activeNetwork := h.scanManager.GetSelectedOrCurrentNetwork(); activeNetwork != nil {
		scoped := make([]*models.Device, 0, len(devices))
		for _, device := range devices {
			if device.NetworkID == activeNetwork.ID {
				scoped = append(scoped, device)
			}
		}
		devices = scoped
	}

	// Get user's screenshot setting
	screenshotsEnabled := h.settingsService.AreScreenshotsEnabled(fmt.Sprintf("%d", user.ID))

	// Build network ID to CIDR map for display
	networkMap := map[string]string{}
	if networks, err := h.networkService.FindAll(); err == nil {
		for _, n := range networks {
			networkMap[n.ID] = n.CIDR
		}
	}

	// Determine the active network for client-side filtering
	activeNetworkID := ""
	scanState := h.scanManager.GetState()
	if scanState.CurrentNetwork != nil {
		activeNetworkID = scanState.CurrentNetwork.ID
	} else if scanState.SelectedNetwork != nil {
		activeNetworkID = scanState.SelectedNetwork.ID
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"devices":            devices,
		"screenshotsEnabled": screenshotsEnabled,
		"networks":           networkMap,
		"activeNetworkId":    activeNetworkID,
		"success":            true,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode JSON response in APIDeviceList: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to encode response",
			"success": false,
		})
	}
}

// APICleanupDeviceNames clears all device names
func (h *WebHandler) APICleanupNetworkBroadcastDevices(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err := h.deviceService.CleanupNetworkBroadcastDevices()
	if err != nil {
		log.Printf("Error cleaning up network/broadcast devices: %v", err)
		http.Error(w, "Failed to cleanup network/broadcast devices", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Network/broadcast devices cleaned up successfully"))
}

func (h *WebHandler) APICleanupDeviceNames(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := h.deviceService.CleanupAllDeviceNames()
	if err != nil {
		log.Printf("Device name cleanup failed: %v", err)
		http.Error(w, fmt.Sprintf("Cleanup failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "success", "message": "All device names have been cleared successfully"}`))
}

// Network API handlers
func (h *WebHandler) APINetworks(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	log.Printf("APINetworks: Fetching networks for display")
	// Get all networks from service
	networksSlice, err := h.networkService.FindAll()
	if err != nil {
		log.Printf("APINetworks: Error getting networks: %v", err)
		networksSlice = []models.Network{} // Ensure it's an empty slice, not nil
	}

	log.Printf("APINetworks: Retrieved %d networks from service", len(networksSlice))

	// Convert to pointer slice for template
	networks := make([]*models.Network, len(networksSlice))
	for i := range networksSlice {
		// Get device count for each network
		deviceCount, _ := h.networkService.GetDeviceCount(networksSlice[i].ID)
		networksSlice[i].DeviceCount = deviceCount
		networks[i] = &networksSlice[i]
	}

	// Get scan state for network selection highlighting
	scanState := h.scanManager.GetState()

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"networks":  networks,
		"scanState": scanState,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding networks JSON: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// parseNetworkRangeForm reads the repeated cidr/cidr_label fields submitted by
// the /networks page's repeatable range-row form. Blank CIDR rows (from an
// empty trailing "add range" row) are dropped; labels are paired with cidrs
// by position and default to "" if the arrays are uneven.
func parseNetworkRangeForm(r *http.Request) (cidrs []string, labels []string) {
	rawCidrs := r.Form["cidr"]
	rawLabels := r.Form["cidr_label"]

	for i, c := range rawCidrs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		cidrs = append(cidrs, c)
		label := ""
		if i < len(rawLabels) {
			label = strings.TrimSpace(rawLabels[i])
		}
		labels = append(labels, label)
	}

	return cidrs, labels
}

// parseStaticRangesForm reads the repeated static_range fields submitted by
// the network edit dialog's static-ranges textarea (one CIDR per line, blank
// lines dropped). These annotate device addressing, not scan targets.
func parseStaticRangesForm(r *http.Request) []string {
	var ranges []string
	for _, cidr := range r.Form["static_range"] {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		ranges = append(ranges, cidr)
	}
	return ranges
}

func (h *WebHandler) APICreateNetwork(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	cidrs, labels := parseNetworkRangeForm(r)
	description := strings.TrimSpace(r.FormValue("description"))
	staticRanges := parseStaticRangesForm(r)

	log.Printf("APICreateNetwork: Received request - name=%s, cidrs=%v, description=%s", name, cidrs, description)

	if err := models.ValidateNetworkRanges(cidrs); err != nil {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if err := models.ValidateStaticRanges(staticRanges); err != nil {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Create network
	log.Printf("APICreateNetwork: Calling networkService.Create")
	network, err := h.networkService.Create(name, cidrs, labels, description, staticRanges)
	if err != nil {
		log.Printf("APICreateNetwork: Error creating network: %v", err)
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to create network: %v", err),
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	log.Printf("APICreateNetwork: Network created successfully: ID=%s, CIDR=%s", network.ID, network.CIDR)

	// Log the event
	h.eventLogService.Log(models.NetworkCreated, fmt.Sprintf("Network %s (%s) created", network.CIDR, network.Name), "")

	// Return JSON success response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Network created successfully",
		"network": network,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *WebHandler) APIUpdateNetwork(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	networkID := vars["id"]

	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	cidrs, labels := parseNetworkRangeForm(r)
	description := strings.TrimSpace(r.FormValue("description"))
	staticRanges := parseStaticRangesForm(r)

	if err := models.ValidateNetworkRanges(cidrs); err != nil {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if err := models.ValidateStaticRanges(staticRanges); err != nil {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Update network
	network, err := h.networkService.Update(networkID, name, cidrs, labels, description, staticRanges)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to update network: %v", err),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Log the event
	h.eventLogService.Log(models.NetworkUpdated, fmt.Sprintf("Network %s (%s) updated", network.CIDR, network.Name), "")

	// Return JSON success response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Network updated successfully",
		"network": network,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *WebHandler) APIDeleteNetwork(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	networkID := vars["id"]

	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if a scan is currently running on this network
	if h.scanManager.IsRunning() {
		currentNetwork := h.scanManager.GetCurrentNetwork()
		if currentNetwork != nil && currentNetwork.ID == networkID {
			w.Header().Set("Content-Type", "application/json")
			response := map[string]interface{}{
				"success": false,
				"error":   "Cannot delete network: a scan is currently running on this network. Please stop the scan first.",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Get network info before deletion for logging
	network, err := h.networkService.FindByID(networkID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   "Network not found",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Check if network has devices before deletion
	deviceCount, err := h.networkService.GetDeviceCount(networkID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to check network devices: %v", err),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if deviceCount > 0 {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Cannot delete network: %d devices are still using this network. Please remove or reassign devices first.", deviceCount),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Delete network
	err = h.networkService.Delete(networkID)
	if err != nil {
		// Check if this is a foreign key constraint error
		errorMsg := fmt.Sprintf("Failed to delete network: %v", err)
		if strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
			errorMsg = "Cannot delete network: devices are still using this network. Please remove or reassign devices first."
		}
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   errorMsg,
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Log the event
	if network != nil {
		h.eventLogService.Log(models.NetworkDeleted, fmt.Sprintf("Network %s (%s) deleted", network.CIDR, network.Name), "")
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Network deleted successfully",
	}
	json.NewEncoder(w).Encode(response)
}

// APIToggleNetworkRange flips a range's active flag, including or excluding
// it from future scans. The range keeps its scan history either way.
func (h *WebHandler) APIToggleNetworkRange(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	networkID := vars["id"]
	rangeID := vars["rangeId"]

	network, err := h.networkService.FindByID(networkID)
	if err != nil || network == nil {
		http.Error(w, "Network not found", http.StatusNotFound)
		return
	}

	var target *models.NetworkRange
	for i := range network.Ranges {
		if network.Ranges[i].ID == rangeID {
			target = &network.Ranges[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "Range not found", http.StatusNotFound)
		return
	}

	if err := h.networkService.SetRangeActive(rangeID, !target.Active); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to update range: %v", err),
		})
		return
	}

	updated, err := h.networkService.FindByID(networkID)
	if err != nil || updated == nil {
		http.Error(w, "Network not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"network": updated,
	})
}

// APIScanStatus returns the current scan status
func (h *WebHandler) APIScanStatus(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	scanState := h.scanManager.GetState()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scanState)
}

// APIScanStart starts scanning a network
func (h *WebHandler) APIScanStart(w http.ResponseWriter, r *http.Request) {
	log.Printf("APIScanStart: Request received, method=%s", r.Method)

	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		log.Printf("APIScanStart: Unauthorized access attempt")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	networkID := r.FormValue("network-selector")
	log.Printf("APIScanStart: Network ID from form: '%s'", networkID)

	if networkID == "" {
		log.Printf("APIScanStart: No network ID provided")
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   "Please select a network to scan",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	err := h.scanManager.StartScan(networkID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
		if scanErr, ok := err.(*scan.ScanError); ok {
			switch scanErr.Type {
			case scan.AlreadyRunning:
				w.WriteHeader(http.StatusConflict)
			case scan.NetworkNotFound:
				w.WriteHeader(http.StatusNotFound)
			default:
				w.WriteHeader(http.StatusBadRequest)
			}
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Note: Scan started event is logged by scan_manager.go to avoid duplicates

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Scan started successfully",
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// APIScanStop stops the current scan
func (h *WebHandler) APIScanStop(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err := h.scanManager.StopScan()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
		if scanErr, ok := err.(*scan.ScanError); ok {
			switch scanErr.Type {
			case scan.NotRunning:
				w.WriteHeader(http.StatusConflict)
			default:
				w.WriteHeader(http.StatusBadRequest)
			}
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Log the event
	h.eventLogService.Log(models.ScanStopped, "Network scan stopped", "")

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Scan stopped successfully",
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// APIScanControl returns the scan control component
func (h *WebHandler) APIScanControl(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Set cache control headers to prevent browser caching
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "application/json")

	// Get networks and scan state
	networksSlice, err := h.networkService.FindAll()
	if err != nil {
		log.Printf("Error getting networks for scan control: %v", err)
		networksSlice = []models.Network{}
	}

	scanState := h.scanManager.GetState()

	response := map[string]interface{}{
		"networks":  networksSlice,
		"scanState": scanState,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding scan control JSON: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// APIScanSelectNetwork sets the selected network (without starting scan)
// APIDashboardMetrics returns JSON data for dashboard metrics
func (h *WebHandler) APIDashboardMetrics(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get current or selected network to determine which network to show
	currentNetwork := h.scanManager.GetSelectedOrCurrentNetwork()
	var devices []*models.Device
	var networkCIDR string = "N/A"
	var err error

	if currentNetwork != nil {
		// Show devices from the currently selected/scanning network
		devicesSlice, err := h.deviceService.FindByNetworkID(currentNetwork.ID)
		if err != nil {
			log.Printf("Error getting devices for dashboard metrics %s: %v", currentNetwork.ID, err)
			devices = []*models.Device{}
		} else {
			// Convert []models.Device to []*models.Device
			devices = make([]*models.Device, len(devicesSlice))
			for i := range devicesSlice {
				devices[i] = &devicesSlice[i]
			}
		}
		networkCIDR = currentNetwork.GetDisplayName()
	} else {
		// If no network is selected, show all devices
		devices, err = h.deviceService.FindAll()
		if err != nil {
			log.Printf("Error getting all devices for dashboard metrics: %v", err)
			devices = []*models.Device{}
		}
	}

	networkMapData := h.buildNetworkMap(devices)

	// Get system status for public IP and location
	status, err := h.systemStatusService.GetLatest()
	var publicIP string
	var location string = ""
	if err == nil && status != nil && status.PublicIP != nil {
		publicIP = *status.PublicIP
		log.Printf("DEBUG: Got public IP: %s", publicIP)

		// If geolocation is missing, try to fetch it now
		if status.Geolocation == nil && h.config.GeolocationEnabled {
			log.Printf("DEBUG: Geolocation is nil, attempting to fetch for IP %s", publicIP)
			geo, geoErr := h.systemStatusService.FetchGeolocation(publicIP)
			if geoErr == nil && geo != nil {
				log.Printf("DEBUG: Successfully fetched geolocation, updating SystemStatus")
				status.Geolocation = geo
				// Update the system status with geolocation
				_, updateErr := h.systemStatusService.CreateOrUpdate(status)
				if updateErr != nil {
					log.Printf("ERROR: Failed to update SystemStatus with geolocation: %v", updateErr)
				}
			} else {
				log.Printf("DEBUG: Failed to fetch geolocation: %v", geoErr)
			}
		}

		// Build location string from geolocation data
		if status.Geolocation != nil {
			geo := status.Geolocation
			log.Printf("DEBUG: Geolocation found - City: %s, Region: %s, Country: %s", geo.City, geo.Region, geo.Country)
			if geo.City != "" && geo.Country != "" {
				location = geo.City + ", " + geo.Country
			} else if geo.Country != "" {
				location = geo.Country
			} else if geo.Region != "" {
				location = geo.Region
			}
			log.Printf("DEBUG: Final location string: %s", location)
		} else {
			log.Printf("DEBUG: Geolocation is still nil for public IP %s", publicIP)
		}
	} else {
		log.Printf("DEBUG: SystemStatus error or nil - err: %v, status: %v", err, status)
	}

	// Calculate network saturation
	var saturation float64 = 0.0
	if currentNetwork != nil && len(networkMapData.IPRange) > 0 {
		// Total possible addresses in the range
		totalAddresses := len(networkMapData.IPRange)
		// Devices found in the range
		devicesInRange := len(devices)
		// Calculate saturation percentage
		if totalAddresses > 0 {
			saturation = (float64(devicesInRange) / float64(totalAddresses)) * 100
		}
	}

	metrics := map[string]interface{}{
		"networkRange":   networkCIDR,
		"publicIP":       publicIP,
		"location":       location,
		"devicesFound":   len(devices),
		"devicesOnline":  networkMapData.NetworkInfo.OnlineDevices,
		"devicesOffline": networkMapData.NetworkInfo.OfflineDevices,
		"saturation":     saturation,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (h *WebHandler) APIScanSelectNetwork(w http.ResponseWriter, r *http.Request) {
	log.Println("APIScanSelectNetwork called")
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	networkID := r.FormValue("network-id")
	if networkID == "" {
		http.Error(w, "Network ID is required", http.StatusBadRequest)
		return
	}

	log.Printf("Setting selected network to: %s", networkID)
	err := h.scanManager.SetSelectedNetwork(networkID)
	if err != nil {
		if scanErr, ok := err.(*scan.ScanError); ok {
			switch scanErr.Type {
			case scan.NetworkNotFound:
				http.Error(w, scanErr.Message, http.StatusNotFound)
			default:
				http.Error(w, scanErr.Message, http.StatusBadRequest)
			}
		} else {
			http.Error(w, fmt.Sprintf("Failed to select network: %v", err), http.StatusInternalServerError)
		}
		return
	}

	log.Println("APIScanSelectNetwork completed successfully")
	w.Header().Set("HX-Trigger", "network-selected")
	w.WriteHeader(http.StatusOK)
}

// APIAbout returns the about page content for the SPA
func (h *WebHandler) APIAbout(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Read version from VERSION file
	version := h.getVersion()

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

// APIDetectedNetworks returns detected networks that don't exist in the database
func (h *WebHandler) APIDetectedNetworks(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	detectedNetworks := h.nicIdentifierService.GetDetectedNetworks()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(detectedNetworks); err != nil {
		http.Error(w, "Failed to encode detected networks", http.StatusInternalServerError)
	}
}

// APINetworkSuggestion creates a network from a suggestion
func (h *WebHandler) APINetworkSuggestion(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cidr := strings.TrimSpace(r.FormValue("cidr"))
	if cidr == "" {
		http.Error(w, "CIDR is required", http.StatusBadRequest)
		return
	}

	// Validate CIDR format
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		http.Error(w, "Invalid CIDR format", http.StatusBadRequest)
		return
	}

	// Create network with auto-generated name
	name := fmt.Sprintf("Network %s", cidr)
	description := "Auto-detected network"

	network, err := h.networkService.Create(name, []string{cidr}, nil, description, nil)
	if err != nil {
		log.Printf("Failed to create suggested network %s: %v", cidr, err)
		http.Error(w, fmt.Sprintf("Failed to create network: %v", err), http.StatusInternalServerError)
		return
	}

	// Log the event
	h.eventLogService.Log(models.NetworkCreated, fmt.Sprintf("Network %s created from suggestion", cidr), "")

	log.Printf("Created network from suggestion: %s (ID: %s)", cidr, network.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"network": network,
		"message": fmt.Sprintf("Network %s created successfully", cidr),
	})
}

func (h *WebHandler) getVersion() string {
	return h.version
}

// templateFuncMap returns the helpers available to the HTML templates.
// Extracted from NewWebHandler so the helpers can be unit-tested directly.
// templateFuncMap is deliberately empty.
//
// index.html and login.html use only Go's builtin actions now — every helper
// that used to live here served the server-rendered device modal and radial
// map, both of which the console replaced with JSON endpoints and a canvas.
//
// Do NOT add "or" or "eq" overrides here. Earlier versions did, and they broke
// boolean `or` (it coalesces rather than disjoins) and `eq` on named string
// types like models.DeviceStatus. internal/web/template_funcs_test.go pins that
// behaviour against regression.
func templateFuncMap() template.FuncMap {
	return template.FuncMap{}
}
