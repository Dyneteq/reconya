package device

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reconya/internal/config"
	"reconya/models"
)

type DeviceHandlers struct {
	Service *DeviceService
	Config  *config.Config
}

func NewDeviceHandlers(service *DeviceService, cfg *config.Config) *DeviceHandlers {
	return &DeviceHandlers{Service: service, Config: cfg}
}

func (h *DeviceHandlers) CreateDevice(w http.ResponseWriter, r *http.Request) {
	var createDeviceDto models.Device
	if err := json.NewDecoder(r.Body).Decode(&createDeviceDto); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	deviceEntity := models.Device{}

	createdDevice, err := h.Service.CreateOrUpdate(&deviceEntity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(createdDevice)
}

func (h *DeviceHandlers) UpdateDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Path[len("/devices/"):]
	if deviceID == "" {
		http.Error(w, "device ID is required", http.StatusBadRequest)
		return
	}

	var updateData struct {
		Name    *string `json:"name,omitempty"`
		Comment *string `json:"comment,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	updatedDevice, err := h.Service.UpdateDevice(deviceID, updateData.Name, updateData.Comment)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedDevice)
}

func (h *DeviceHandlers) GetAllDevices(w http.ResponseWriter, r *http.Request) {
	// This handler is deprecated since networks are now user-configurable
	// Return all devices instead of network-specific devices
	foundDevices, err := h.Service.FindAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Returning %d devices from all networks", len(foundDevices))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(foundDevices)
}

// CleanupDeviceNames clears all device names
func (h *DeviceHandlers) CleanupDeviceNames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := h.Service.CleanupAllDeviceNames()
	if err != nil {
		log.Printf("Device name cleanup failed: %v", err)
		http.Error(w, fmt.Sprintf("Cleanup failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{
		"status":  "success",
		"message": "All device names have been cleared successfully",
	}
	json.NewEncoder(w).Encode(response)
}
