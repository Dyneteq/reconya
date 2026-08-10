package web

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

func (h *WebHandler) SetupRoutes() *mux.Router {
	r := mux.NewRouter()

	// Web pages - consolidated page serving
	r.HandleFunc("/", h.ServePage("dashboard")).Methods("GET")
	r.HandleFunc("/devices", h.ServePage("devices")).Methods("GET")
	r.HandleFunc("/networks", h.ServePage("networks")).Methods("GET")
	r.HandleFunc("/logs", h.ServePage("logs")).Methods("GET")
	r.HandleFunc("/alerts", h.ServePage("alerts")).Methods("GET")
	r.HandleFunc("/settings", h.ServePage("settings")).Methods("GET")

	// Authentication
	r.HandleFunc("/login", h.Login).Methods("GET", "POST")
	r.HandleFunc("/logout", h.Logout).Methods("POST")

	// API endpoints
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/devices/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}/modal", h.APIDeviceModal).Methods("GET")
	api.HandleFunc("/devices/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", h.APIUpdateDevice).Methods("PUT")
	api.HandleFunc("/devices/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", h.APIDeleteDevice).Methods("DELETE")
	api.HandleFunc("/devices/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}/rescan", h.APIRescanDevice).Methods("POST")
	api.HandleFunc("/ping-internet", h.APIPingInternet).Methods("GET")
	api.HandleFunc("/dashboard-metrics", h.APIDashboardMetrics).Methods("GET")
	api.HandleFunc("/event-logs", h.APIEventLogs).Methods("GET")
	api.HandleFunc("/event-logs-table", h.APIEventLogsTable).Methods("GET")
	api.HandleFunc("/devices/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}/events", h.APIDeviceEvents).Methods("GET")

	// Alert endpoints
	api.HandleFunc("/alerts", h.APIAlerts).Methods("GET")
	api.HandleFunc("/alerts/ack-all", h.APIAckAllAlerts).Methods("POST")
	api.HandleFunc("/alerts/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}/ack", h.APIAckAlert).Methods("POST")
	api.HandleFunc("/device-list", h.APIDeviceList).Methods("GET")
	api.HandleFunc("/devices/cleanup-names", h.APICleanupDeviceNames).Methods("POST")
	api.HandleFunc("/devices/cleanup-network-broadcast", h.APICleanupNetworkBroadcastDevices).Methods("POST")
	api.HandleFunc("/networks", h.APINetworks).Methods("GET")
	api.HandleFunc("/networks", h.APICreateNetwork).Methods("POST")
	api.HandleFunc("/networks/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", h.APIUpdateNetwork).Methods("PUT")
	api.HandleFunc("/networks/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", h.APIDeleteNetwork).Methods("DELETE")

	// Scan management endpoints
	api.HandleFunc("/scan/status", h.APIScanStatus).Methods("GET")
	api.HandleFunc("/scan/start", h.APIScanStart).Methods("POST")
	api.HandleFunc("/scan/stop", h.APIScanStop).Methods("POST")
	api.HandleFunc("/scan/control", h.APIScanControl).Methods("GET")
	api.HandleFunc("/scan/select-network", h.APIScanSelectNetwork).Methods("POST")
	api.HandleFunc("/about", h.APIAbout).Methods("GET")

	// Settings endpoints
	api.HandleFunc("/settings", h.APISettings).Methods("GET")
	api.HandleFunc("/settings/screenshots", h.APISettingsScreenshots).Methods("POST")

	// Network detection endpoints
	api.HandleFunc("/detected-networks", h.APIDetectedNetworks).Methods("GET")
	api.HandleFunc("/network-suggestion", h.APINetworkSuggestion).Methods("POST")

	// Static file serving from embedded filesystem
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.FS(h.staticFS))))

	// 404 handler
	r.NotFoundHandler = http.HandlerFunc(h.NotFound)

	return r
}

// APIRescanDevice queues an immediate port scan for one device, backing the
// console drawer's RESCAN HOST button.
//
// The scan runs in the background: a full port sweep takes far longer than a
// request should, so this returns as soon as the work is queued and the caller
// picks up the result from the next device poll.
func (h *WebHandler) APIRescanDevice(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	deviceID := mux.Vars(r)["id"]

	device, err := h.deviceService.FindByID(deviceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if device == nil {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}

	go h.portScanService.Run(*device)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Port scan queued for " + device.IPv4,
	})
}

func (h *WebHandler) NotFound(w http.ResponseWriter, r *http.Request) {
	// For unknown routes, serve the main SPA and let JavaScript handle the 404
	h.Index(w, r)
}
