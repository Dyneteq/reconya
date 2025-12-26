package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"reconya/models"

	"github.com/gorilla/mux"
)

// APIDevices returns all devices as JSON
func (h *WebHandler) APIDevices(w http.ResponseWriter, r *http.Request) {
	log.Printf("APIDevices: Request received from %s", r.RemoteAddr)
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	log.Printf("APIDevices: User session: %v", user != nil)
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Unauthorized",
			"success": false,
		})
		return
	}

	allDevices, err := h.deviceService.FindAll()
	if err != nil {
		log.Printf("Error getting all devices: %v", err)
		allDevices = []*models.Device{}
	}

	devicesSlice := make([]models.Device, len(allDevices))
	for i, d := range allDevices {
		devicesSlice[i] = *d
	}

	devices := make([]*models.Device, len(devicesSlice))
	for i := range devicesSlice {
		devices[i] = &devicesSlice[i]
	}

	screenshotsEnabled := h.settingsService.AreScreenshotsEnabled(fmt.Sprintf("%d", user.ID))
	viewMode := r.URL.Query().Get("view")

	log.Printf("APIDevices: Found %d devices, viewMode: %s", len(devices), viewMode)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"devices":            devices,
		"viewMode":           viewMode,
		"screenshotsEnabled": screenshotsEnabled,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode JSON response in APIDevices: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to encode response",
			"success": false,
		})
	}
}

// APIDeviceModal returns device details for modal
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

	screenshotsEnabled := h.settingsService.AreScreenshotsEnabled(fmt.Sprintf("%d", user.ID))

	log.Printf("Device %s IPv6 data: LinkLocal=%v, UniqueLocal=%v, Global=%v, Addresses=%v",
		device.ID, device.IPv6LinkLocal, device.IPv6UniqueLocal, device.IPv6Global, device.IPv6Addresses)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"device":             device,
		"screenshotsEnabled": screenshotsEnabled,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// APIUpdateDevice updates a device's name and comment
func (h *WebHandler) APIUpdateDevice(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	deviceID := vars["id"]

	var data struct {
		Name    string `json:"name"`
		Comment string `json:"comment"`
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

	device, err := h.deviceService.UpdateDevice(deviceID, namePtr, commentPtr)
	if err != nil {
		log.Printf("Failed to update device %s: %v", deviceID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully updated device %s", deviceID)

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

// APIDeleteDevice deletes a device
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

	device, err := h.deviceService.FindByID(deviceID)
	if err != nil {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}

	if device == nil {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}

	err = h.deviceService.Delete(deviceID)
	if err != nil {
		log.Printf("Failed to delete device %s: %v", deviceID, err)
		http.Error(w, fmt.Sprintf("Failed to delete device: %v", err), http.StatusInternalServerError)
		return
	}

	h.eventLogService.Log(models.DeviceDeleted, fmt.Sprintf("Device %s deleted", device.IPv4), "")
	log.Printf("Successfully deleted device %s (%s)", device.IPv4, deviceID)

	w.WriteHeader(http.StatusOK)
}

// APITestIPv6 adds test IPv6 data to a device (for debugging)
func (h *WebHandler) APITestIPv6(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deviceID := r.FormValue("device_id")
	if deviceID == "" {
		http.Error(w, "device_id is required", http.StatusBadRequest)
		return
	}

	ipv6Addresses := map[string]string{
		"link_local":   "fe80::1234:5678:90ab:cdef",
		"unique_local": "fd00::1234:5678:90ab:cdef",
		"global":       "2001:db8::1234:5678:90ab:cdef",
	}

	err := h.deviceService.UpdateDeviceIPv6Addresses(deviceID, ipv6Addresses)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update device IPv6: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("IPv6 addresses added successfully"))
}

// APIDeviceList returns devices in a different format
func (h *WebHandler) APIDeviceList(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	devices, err := h.deviceService.FindAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"devices": devices,
		"total":   len(devices),
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// APICleanupDeviceNames cleans up duplicate or invalid device names
func (h *WebHandler) APICleanupDeviceNames(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err := h.deviceService.CleanupAllDeviceNames()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
	}
	json.NewEncoder(w).Encode(response)
}

// APICleanupNetworkBroadcastDevices removes network and broadcast address devices
func (h *WebHandler) APICleanupNetworkBroadcastDevices(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err := h.deviceService.CleanupNetworkBroadcastDevices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
	}
	json.NewEncoder(w).Encode(response)
}
