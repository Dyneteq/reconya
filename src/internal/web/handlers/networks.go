package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"reconya/models"

	"github.com/gorilla/mux"
)

// APINetworks returns all networks
func (h *WebHandler) APINetworks(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	log.Printf("APINetworks: Fetching networks for display")
	networksSlice, err := h.networkService.FindAll()
	if err != nil {
		log.Printf("APINetworks: Error getting networks: %v", err)
		networksSlice = []models.Network{}
	}

	log.Printf("APINetworks: Retrieved %d networks from service", len(networksSlice))

	networks := make([]*models.Network, len(networksSlice))
	for i := range networksSlice {
		deviceCount, _ := h.networkService.GetDeviceCount(networksSlice[i].ID)
		networksSlice[i].DeviceCount = deviceCount
		networks[i] = &networksSlice[i]
	}

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

// APINetworkModal returns network data for modal
func (h *WebHandler) APINetworkModal(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	networkID := vars["id"]

	response := map[string]interface{}{
		"network": &models.Network{},
		"error":   "",
	}

	if networkID != "" {
		network, err := h.networkService.FindByID(networkID)
		if err != nil {
			response["error"] = "Network not found"
		} else if network != nil {
			response["network"] = network
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// APICreateNetwork creates a new network
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
	cidr := strings.TrimSpace(r.FormValue("cidr"))
	description := strings.TrimSpace(r.FormValue("description"))

	log.Printf("APICreateNetwork: Received request - name=%s, cidr=%s, description=%s", name, cidr, description)

	if cidr == "" {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   "CIDR address is required",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   "Invalid CIDR format. Please use format like 192.168.1.0/24",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("APICreateNetwork: Calling networkService.Create")
	network, err := h.networkService.Create(name, cidr, description)
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

	h.eventLogService.Log(models.NetworkCreated, fmt.Sprintf("Network %s (%s) created", network.CIDR, network.Name), "")

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

// APIUpdateNetwork updates a network
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
	cidr := strings.TrimSpace(r.FormValue("cidr"))
	description := strings.TrimSpace(r.FormValue("description"))

	if cidr == "" {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   "CIDR address is required",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   "Invalid CIDR format. Please use format like 192.168.1.0/24",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	network, err := h.networkService.Update(networkID, name, cidr, description)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to update network: %v", err),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	h.eventLogService.Log(models.NetworkUpdated, fmt.Sprintf("Network %s (%s) updated", network.CIDR, network.Name), "")

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

// APIDeleteNetwork deletes a network
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

	err = h.networkService.Delete(networkID)
	if err != nil {
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

	if network != nil {
		h.eventLogService.Log(models.NetworkDeleted, fmt.Sprintf("Network %s (%s) deleted", network.CIDR, network.Name), "")
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Network deleted successfully",
	}
	json.NewEncoder(w).Encode(response)
}

// APINetworkDeleteInfo returns information about network deletion including affected devices
func (h *WebHandler) APINetworkDeleteInfo(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	networkID := vars["id"]

	network, err := h.networkService.FindByID(networkID)
	if err != nil {
		http.Error(w, "Network not found", http.StatusNotFound)
		return
	}

	isScanning := false
	if h.scanManager.IsRunning() {
		currentNetwork := h.scanManager.GetCurrentNetwork()
		if currentNetwork != nil && currentNetwork.ID == networkID {
			isScanning = true
		}
	}

	deviceCount, err := h.networkService.GetDeviceCount(networkID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to check network devices: %v", err), http.StatusInternalServerError)
		return
	}

	devices, err := h.deviceService.FindByNetworkID(networkID)
	if err != nil {
		log.Printf("Error fetching devices for network %s: %v", networkID, err)
		devices = []models.Device{}
	}

	deleteInfo := struct {
		Network     *models.Network `json:"network"`
		DeviceCount int             `json:"deviceCount"`
		Devices     []models.Device `json:"devices"`
		IsScanning  bool            `json:"isScanning"`
		CanDelete   bool            `json:"canDelete"`
		Message     string          `json:"message"`
	}{
		Network:     network,
		DeviceCount: deviceCount,
		Devices:     devices,
		IsScanning:  isScanning,
		CanDelete:   !isScanning,
		Message:     "",
	}

	if isScanning {
		deleteInfo.Message = "Cannot delete network: a scan is currently running on this network. Please stop the scan first."
	} else if deviceCount > 0 {
		deleteInfo.Message = fmt.Sprintf("This network contains %d device(s). Deleting the network will also remove these devices from the system.", deviceCount)
	} else {
		deleteInfo.Message = "Are you sure you want to delete this network?"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(deleteInfo)
}

// APIForceDeleteNetwork deletes a network and all its devices with confirmation
func (h *WebHandler) APIForceDeleteNetwork(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	networkID := vars["id"]

	if h.scanManager.IsRunning() {
		currentNetwork := h.scanManager.GetCurrentNetwork()
		if currentNetwork != nil && currentNetwork.ID == networkID {
			http.Error(w, "Cannot delete network: a scan is currently running on this network. Please stop the scan first.", http.StatusConflict)
			return
		}
	}

	network, err := h.networkService.FindByID(networkID)
	if err != nil {
		http.Error(w, "Network not found", http.StatusNotFound)
		return
	}

	deviceCount, err := h.networkService.GetDeviceCount(networkID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to check network devices: %v", err), http.StatusInternalServerError)
		return
	}

	if deviceCount > 0 {
		err = h.deviceService.DeleteByNetworkID(networkID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to delete network devices: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("Deleted %d devices from network %s before network deletion", deviceCount, networkID)
	}

	err = h.networkService.Delete(networkID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete network: %v", err), http.StatusInternalServerError)
		return
	}

	if network != nil {
		message := fmt.Sprintf("Network %s (%s) deleted", network.CIDR, network.Name)
		if deviceCount > 0 {
			message += fmt.Sprintf(" along with %d device(s)", deviceCount)
		}
		h.eventLogService.Log(models.NetworkDeleted, message, "")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Network deleted successfully",
	})
}

// APINetworkDeleteModal returns the network deletion confirmation modal
func (h *WebHandler) APINetworkDeleteModal(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	networkID := vars["id"]

	network, err := h.networkService.FindByID(networkID)
	if err != nil {
		http.Error(w, "Network not found", http.StatusNotFound)
		return
	}

	isScanning := false
	if h.scanManager.IsRunning() {
		currentNetwork := h.scanManager.GetCurrentNetwork()
		if currentNetwork != nil && currentNetwork.ID == networkID {
			isScanning = true
		}
	}

	deviceCount, err := h.networkService.GetDeviceCount(networkID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to check network devices: %v", err), http.StatusInternalServerError)
		return
	}

	devices, err := h.deviceService.FindByNetworkID(networkID)
	if err != nil {
		log.Printf("Error fetching devices for network %s: %v", networkID, err)
		devices = []models.Device{}
	}

	deleteInfo := struct {
		Network     *models.Network `json:"network"`
		DeviceCount int             `json:"deviceCount"`
		Devices     []models.Device `json:"devices"`
		IsScanning  bool            `json:"isScanning"`
		CanDelete   bool            `json:"canDelete"`
		Message     string          `json:"message"`
	}{
		Network:     network,
		DeviceCount: deviceCount,
		Devices:     devices,
		IsScanning:  isScanning,
		CanDelete:   !isScanning,
		Message:     "",
	}

	if isScanning {
		deleteInfo.Message = "Cannot delete network: a scan is currently running on this network. Please stop the scan first."
	} else if deviceCount > 0 {
		deleteInfo.Message = fmt.Sprintf("This network contains %d device(s). Deleting the network will also remove these devices from the system.", deviceCount)
	} else {
		deleteInfo.Message = "Are you sure you want to delete this network?"
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(deleteInfo); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// APIDetectedNetworks returns networks detected on the system
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

	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		http.Error(w, "Invalid CIDR format", http.StatusBadRequest)
		return
	}

	name := fmt.Sprintf("Network %s", cidr)
	description := "Auto-detected network"

	network, err := h.networkService.Create(name, cidr, description)
	if err != nil {
		log.Printf("Failed to create suggested network %s: %v", cidr, err)
		http.Error(w, fmt.Sprintf("Failed to create network: %v", err), http.StatusInternalServerError)
		return
	}

	h.eventLogService.Log(models.NetworkCreated, fmt.Sprintf("Network %s created from suggestion", cidr), "")

	log.Printf("Created network from suggestion: %s (ID: %s)", cidr, network.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"network": network,
		"message": fmt.Sprintf("Network %s created successfully", cidr),
	})
}

// APIDetectedNetworksDebug returns detected networks without authentication (for testing)
func (h *WebHandler) APIDetectedNetworksDebug(w http.ResponseWriter, r *http.Request) {
	detectedNetworks := h.nicIdentifierService.GetDetectedNetworks()

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"detected_networks": detectedNetworks,
		"count":             len(detectedNetworks),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode detected networks", http.StatusInternalServerError)
	}
}

// APINetworksDebug returns all networks in database (for testing)
func (h *WebHandler) APINetworksDebug(w http.ResponseWriter, r *http.Request) {
	networks, err := h.networkService.FindAll()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get networks: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"existing_networks": networks,
		"count":             len(networks),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode networks", http.StatusInternalServerError)
	}
}
