package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"time"

	"reconya/internal/scan"
	"reconya/models"
)

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

	h.eventLogService.Log(models.ScanStopped, "Network scan stopped", "")

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

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "application/json")

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

// APIScanControlWithError returns the scan control component with an error message
func (h *WebHandler) APIScanControlWithError(w http.ResponseWriter, r *http.Request, errorMsg string) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	networksSlice, err := h.networkService.FindAll()
	if err != nil {
		log.Printf("Error getting networks for scan control: %v", err)
		networksSlice = []models.Network{}
	}

	scanState := h.scanManager.GetState()

	data := map[string]interface{}{
		"networks":  networksSlice,
		"scanState": &scanState,
	}
	if errorMsg != "" {
		data["error"] = errorMsg
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding scan control JSON: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// APIScanSelectNetwork selects a network for scanning
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

type sensorDashboardSummary struct {
	Total   int `json:"total"`
	Online  int `json:"online"`
	Offline int `json:"offline"`
	Pending int `json:"pending"`
}

type sensorDashboardResponse struct {
	ID                  string                  `json:"id"`
	Name                string                  `json:"name"`
	Status              models.SensorStatus     `json:"status"`
	ScanStatus          models.SensorScanStatus `json:"scanStatus"`
	NetworkRange        string                  `json:"networkRange"`
	DevicesFound        int                     `json:"devicesFound"`
	DevicesOnline       int                     `json:"devicesOnline"`
	DevicesIdle         int                     `json:"devicesIdle"`
	PublicIP            string                  `json:"publicIP"`
	Saturation          float64                 `json:"saturation"`
	LastSeenAt          *time.Time              `json:"lastSeenAt,omitempty"`
	LastScanStartedAt   *time.Time              `json:"lastScanStartedAt,omitempty"`
	LastScanCompletedAt *time.Time              `json:"lastScanCompletedAt,omitempty"`
}

// APIDashboardMetrics returns JSON data for aggregated dashboard metrics
func (h *WebHandler) APIDashboardMetrics(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	networks, err := h.networkService.FindAll()
	if err != nil {
		log.Printf("Failed to load networks: %v", err)
	}

	sensors, err := h.sensorService.GetAllSensors(r.Context())
	if err != nil {
		log.Printf("Failed to load sensors: %v", err)
	}

	sensorsOnline := 0
	for _, sensor := range sensors {
		if sensor.Status == models.SensorStatusOnline {
			sensorsOnline++
		}
	}

	devices, err := h.deviceService.FindAll()
	if err != nil {
		log.Printf("Failed to load devices: %v", err)
	}

	devicesFound := len(devices)
	devicesOnline := 0
	for _, device := range devices {
		if device.Status == models.DeviceStatusOnline {
			devicesOnline++
		}
	}

	var totalSaturation float64
	networkCount := 0
	for _, network := range networks {
		if network.CIDR != "" {
			networkDevices, _ := h.deviceService.FindByNetworkID(network.ID)
			sat := calculateSaturation(network.CIDR, len(networkDevices))
			totalSaturation += sat
			networkCount++
		}
	}

	avgSaturation := float64(0)
	if networkCount > 0 {
		avgSaturation = totalSaturation / float64(networkCount)
	}

	summary := map[string]interface{}{
		"networks":      len(networks),
		"devicesFound":  devicesFound,
		"devicesOnline": devicesOnline,
		"sensorsOnline": sensorsOnline,
		"saturation":    avgSaturation,
	}

	payload := map[string]interface{}{
		"summary": summary,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("Error encoding dashboard metrics response: %v", err)
		http.Error(w, fmt.Sprintf("encoding error: %v", err), http.StatusInternalServerError)
	}
}

func (h *WebHandler) buildSensorDashboard(sensor *models.Sensor) sensorDashboardResponse {
	response := sensorDashboardResponse{
		ID:                  sensor.ID,
		Name:                sensor.Name,
		Status:              sensor.Status,
		ScanStatus:          sensor.ScanStatus,
		NetworkRange:        "N/A",
		PublicIP:            "N/A",
		LastSeenAt:          sensor.LastSeenAt,
		LastScanStartedAt:   sensor.LastScanStartedAt,
		LastScanCompletedAt: sensor.LastScanCompletedAt,
	}

	if sensor.IP != nil && *sensor.IP != "" {
		response.PublicIP = *sensor.IP
	}

	if sensor.NetworkCIDR != nil && *sensor.NetworkCIDR != "" {
		response.NetworkRange = *sensor.NetworkCIDR

		network, err := h.networkService.FindByCIDR(*sensor.NetworkCIDR)
		if err != nil {
			log.Printf("Failed to find network for CIDR %s: %v", *sensor.NetworkCIDR, err)
		}

		if network != nil {
			devices, err := h.deviceService.FindByNetworkID(network.ID)
			if err != nil {
				log.Printf("Failed to load devices for network %s: %v", network.ID, err)
			} else {
				response.DevicesFound = len(devices)
				for _, device := range devices {
					switch device.Status {
					case models.DeviceStatusOnline:
						response.DevicesOnline++
					case models.DeviceStatusIdle:
						response.DevicesIdle++
					}
				}
			}
		}

		response.Saturation = calculateSaturation(response.NetworkRange, response.DevicesFound)
	}

	return response
}

func calculateSaturation(cidr string, deviceCount int) float64 {
	if cidr == "" || deviceCount <= 0 {
		return 0
	}

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return 0
	}

	ones, bits := ipNet.Mask.Size()
	if bits != 32 || ones < 1 || ones > 32 {
		return 0
	}

	hostCount := 1 << uint(bits-ones)
	if bits == 32 && ones <= 30 {
		hostCount -= 2 // remove network + broadcast
	}
	if hostCount <= 0 {
		return 0
	}

	percentage := (float64(deviceCount) / float64(hostCount)) * 100
	if percentage < 0 {
		return 0
	}
	if percentage > 100 {
		return 100
	}
	return math.Round(percentage*10) / 10
}
