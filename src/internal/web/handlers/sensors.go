package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"reconya/db"
	"reconya/internal/sensor"
	"reconya/models"

	"github.com/gorilla/mux"
)

// GetSensors returns all sensors
func (h *WebHandler) GetSensors(w http.ResponseWriter, r *http.Request) {
	sensors, err := h.sensorService.GetAllSensors(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sensors)
}

// CreateSensor creates a new sensor
func (h *WebHandler) CreateSensor(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	sensorObj, err := h.sensorService.CreateSensor(r.Context(), req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sensorObj)
}

// DeleteSensor deletes a sensor
func (h *WebHandler) DeleteSensor(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		http.Error(w, "Sensor ID is required", http.StatusBadRequest)
		return
	}

	err := h.sensorService.DeleteSensor(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RegisterSensor handles sensor registration via token
func (h *WebHandler) RegisterSensor(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token is required", http.StatusUnauthorized)
		return
	}

	var req sensor.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	updatedSensor, err := h.sensorService.Register(r.Context(), token, req)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedSensor)
}

// SensorCommandStatus exposes the latest scan command for a sensor token to remote agents.
func (h *WebHandler) SensorCommandStatus(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token is required", http.StatusUnauthorized)
		return
	}

	sensorObj, err := h.sensorService.GetSensorByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "Sensor not found", http.StatusUnauthorized)
		} else {
			http.Error(w, fmt.Sprintf("Failed to look up sensor: %v", err), http.StatusInternalServerError)
		}
		return
	}

	response := struct {
		ID                  string                  `json:"id"`
		Status              models.SensorStatus     `json:"status"`
		ScanStatus          models.SensorScanStatus `json:"scan_status"`
		LastScanStartedAt   *time.Time              `json:"last_scan_started_at,omitempty"`
		LastScanCompletedAt *time.Time              `json:"last_scan_completed_at,omitempty"`
	}{
		ID:                  sensorObj.ID,
		Status:              sensorObj.Status,
		ScanStatus:          sensorObj.ScanStatus,
		LastScanStartedAt:   sensorObj.LastScanStartedAt,
		LastScanCompletedAt: sensorObj.LastScanCompletedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// StartSensorScan starts a scan on a sensor
func (h *WebHandler) StartSensorScan(w http.ResponseWriter, r *http.Request) {
	h.handleSensorScanAction(w, r, models.SensorScanStatusRunning)
}

// StopSensorScan stops a scan on a sensor
func (h *WebHandler) StopSensorScan(w http.ResponseWriter, r *http.Request) {
	h.handleSensorScanAction(w, r, models.SensorScanStatusIdle)
}

func (h *WebHandler) handleSensorScanAction(w http.ResponseWriter, r *http.Request, scanStatus models.SensorScanStatus) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	sensorID := vars["id"]
	if sensorID == "" {
		http.Error(w, "Sensor ID is required", http.StatusBadRequest)
		return
	}

	updatedSensor, err := h.sensorService.UpdateScanStatus(r.Context(), sensorID, scanStatus)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "Sensor not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to update sensor scan state: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedSensor)
}

// ReceiveSensorDevices handles device data submitted by remote sensors
func (h *WebHandler) ReceiveSensorDevices(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token is required", http.StatusUnauthorized)
		return
	}

	sensorObj, err := h.sensorService.GetSensorByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "Sensor not found", http.StatusUnauthorized)
		} else {
			http.Error(w, fmt.Sprintf("Failed to look up sensor: %v", err), http.StatusInternalServerError)
		}
		return
	}

	var devices []struct {
		IPv4     string  `json:"ipv4"`
		MAC      *string `json:"mac,omitempty"`
		Vendor   *string `json:"vendor,omitempty"`
		Hostname *string `json:"hostname,omitempty"`
		Status   string  `json:"status"`
		Ports    []struct {
			Number   string `json:"number"`
			Protocol string `json:"protocol"`
			Service  string `json:"service,omitempty"`
		} `json:"ports,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&devices); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	networkCIDR := ""
	if sensorObj.NetworkCIDR != nil {
		networkCIDR = *sensorObj.NetworkCIDR
	}
	if networkCIDR == "" {
		http.Error(w, "Sensor has no network configured", http.StatusBadRequest)
		return
	}

	network, err := h.networkService.FindOrCreate(networkCIDR)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to find/create network: %v", err), http.StatusInternalServerError)
		return
	}

	savedCount := 0
	for _, dev := range devices {
		device := &models.Device{
			IPv4:      dev.IPv4,
			MAC:       dev.MAC,
			Vendor:    dev.Vendor,
			Hostname:  dev.Hostname,
			NetworkID: network.ID,
			Status:    models.DeviceStatus(dev.Status),
		}

		for _, p := range dev.Ports {
			device.Ports = append(device.Ports, models.Port{
				Number:   p.Number,
				Protocol: p.Protocol,
				Service:  p.Service,
			})
		}

		if _, err := h.deviceService.CreateOrUpdate(device); err != nil {
			log.Printf("Failed to save device %s: %v", dev.IPv4, err)
			continue
		}
		savedCount++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"saved":   savedCount,
		"total":   len(devices),
		"success": true,
	})
}
