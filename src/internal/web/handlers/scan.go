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

	devices, err := h.deviceService.FindAll()
	if err != nil {
		log.Printf("Failed to load devices: %v", err)
	}

	oneHourAgo := time.Now().Add(-1 * time.Hour)
	devicesFound := 0
	devicesOnline := 0
	for _, device := range devices {
		if device.LastSeenOnlineAt == nil && device.Status != models.DeviceStatusOnline {
			continue
		}
		if device.Status == models.DeviceStatusOffline {
			if device.LastSeenOnlineAt == nil || device.LastSeenOnlineAt.Before(oneHourAgo) {
				continue
			}
		}
		devicesFound++
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
