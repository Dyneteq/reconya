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

type deviceInfo struct {
	IP     string `json:"ip"`
	Status string `json:"status"`
}

type eventLogInfo struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

type geolocationInfo struct {
	City        string  `json:"city"`
	Region      string  `json:"region"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	ISP         string  `json:"isp"`
}

type sensorWithStats struct {
	models.Sensor
	DevicesTotal  int              `json:"devices_total"`
	DevicesOnline int              `json:"devices_online"`
	Devices       []deviceInfo     `json:"devices"`
	RecentLogs    []eventLogInfo   `json:"recent_logs"`
	Geolocation   *geolocationInfo `json:"geolocation,omitempty"`
}

func (h *WebHandler) GetSensors(w http.ResponseWriter, r *http.Request) {
	sensors, err := h.sensorService.GetAllSensors(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := make([]sensorWithStats, 0, len(sensors))
	for _, sensor := range sensors {
		stats := sensorWithStats{Sensor: *sensor, Devices: []deviceInfo{}, RecentLogs: []eventLogInfo{}}

		devices, err := h.deviceService.FindBySensorID(sensor.ID)
		if err != nil {
			log.Printf("GetSensors: FindBySensorID error for %s: %v", sensor.ID, err)
		}

		// Fallback: if no devices found by sensor_id, try by network_cidr
		// and update those devices with the sensor_id for future queries
		if len(devices) == 0 && sensor.NetworkCIDR != nil && *sensor.NetworkCIDR != "" {
			network, err := h.networkService.FindByCIDR(*sensor.NetworkCIDR)
			if err == nil && network != nil {
				networkDevices, err := h.deviceService.FindByNetworkID(network.ID)
				if err == nil {
					for _, d := range networkDevices {
						// Update device with sensor_id
						dCopy := d
						dCopy.SensorID = &sensor.ID
						h.deviceService.CreateOrUpdate(&dCopy)
						devices = append(devices, &dCopy)
					}
				}
			}
		}

		stats.DevicesTotal = len(devices)
		for _, d := range devices {
			if d.Status == models.DeviceStatusOnline {
				stats.DevicesOnline++
			}
			stats.Devices = append(stats.Devices, deviceInfo{
				IP:     d.IPv4,
				Status: string(d.Status),
			})
		}

		logs, err := h.eventLogService.GetBySensorID(sensor.ID, 10)
		if err != nil {
			log.Printf("GetSensors: GetBySensorID error for %s: %v", sensor.ID, err)
		}
		for _, l := range logs {
			stats.RecentLogs = append(stats.RecentLogs, eventLogInfo{
				Type:        string(l.Type),
				Description: l.Description,
				CreatedAt:   l.CreatedAt.Format("15:04:05"),
			})
		}

		// Fetch geolocation for sensor's public IP
		if sensor.PublicIP != nil && *sensor.PublicIP != "" {
			geo, err := h.systemStatusService.FetchGeolocation(*sensor.PublicIP)
			if err == nil && geo != nil {
				stats.Geolocation = &geolocationInfo{
					City:        geo.City,
					Region:      geo.Region,
					Country:     geo.Country,
					CountryCode: geo.CountryCode,
					Latitude:    geo.Latitude,
					Longitude:   geo.Longitude,
					ISP:         geo.ISP,
				}
			}
		}

		result = append(result, stats)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
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

	network, err := h.networkService.FindOrCreateWithSensor(networkCIDR, sensorObj.ID)
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
			SensorID:  &sensorObj.ID,
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

	if savedCount > 0 {
		h.eventLogService.LogForSensor(
			models.DeviceOnline,
			fmt.Sprintf("Received %d devices from sensor scan", savedCount),
			sensorObj.ID,
		)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"saved":   savedCount,
		"total":   len(devices),
		"success": true,
	})
}
