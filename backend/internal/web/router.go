package web

import (
	"html/template"
	"net/http"

	"github.com/gorilla/mux"
)

func (h *WebHandler) SetupRoutes() *mux.Router {
	r := mux.NewRouter()

	// Web pages - consolidated page serving
	r.HandleFunc("/", h.ServePage("dashboard")).Methods("GET")
	r.HandleFunc("/home", h.Home).Methods("GET") // Home has complex logic, keep separate
	r.HandleFunc("/devices", h.ServePage("devices")).Methods("GET")
	r.HandleFunc("/networks", h.ServePage("networks")).Methods("GET")
	r.HandleFunc("/logs", h.ServePage("logs")).Methods("GET")
	r.HandleFunc("/alerts", h.ServePage("alerts")).Methods("GET")
	r.HandleFunc("/settings", h.ServePage("settings")).Methods("GET")
	r.HandleFunc("/about", h.ServePage("about")).Methods("GET")
	r.HandleFunc("/targets", h.ServePage("targets")).Methods("GET")

	// Authentication
	r.HandleFunc("/login", h.Login).Methods("GET", "POST")
	r.HandleFunc("/logout", h.Logout).Methods("POST")

	// API endpoints
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/devices", h.APIDevices).Methods("GET")
	api.HandleFunc("/devices/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}/modal", h.APIDeviceModal).Methods("GET")
	api.HandleFunc("/devices/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", h.APIUpdateDevice).Methods("PUT")
	api.HandleFunc("/devices/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", h.APIDeleteDevice).Methods("DELETE")
	api.HandleFunc("/devices/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}/rescan", h.APIRescanDevice).Methods("POST")
	api.HandleFunc("/devices/new-scan", h.APINewScan).Methods("GET")
	api.HandleFunc("/test-ipv6", h.APITestIPv6).Methods("POST")
	api.HandleFunc("/targets", h.APITargets).Methods("GET")
	api.HandleFunc("/system-status", h.APISystemStatus).Methods("GET")
	api.HandleFunc("/dashboard-metrics", h.APIDashboardMetrics).Methods("GET")
	api.HandleFunc("/event-logs", h.APIEventLogs).Methods("GET")
	api.HandleFunc("/event-logs-table", h.APIEventLogsTable).Methods("GET")
	api.HandleFunc("/network-map", h.APINetworkMap).Methods("GET")
	api.HandleFunc("/traffic-core", h.APITrafficCore).Methods("GET")
	api.HandleFunc("/device-list", h.APIDeviceList).Methods("GET")
	api.HandleFunc("/devices/cleanup-names", h.APICleanupDeviceNames).Methods("POST")
	api.HandleFunc("/devices/cleanup-network-broadcast", h.APICleanupNetworkBroadcastDevices).Methods("POST")
	api.HandleFunc("/networks", h.APINetworks).Methods("GET")
	api.HandleFunc("/networks", h.APICreateNetwork).Methods("POST")
	api.HandleFunc("/networks/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", h.APIUpdateNetwork).Methods("PUT")
	api.HandleFunc("/networks/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", h.APIDeleteNetwork).Methods("DELETE")
	api.HandleFunc("/networks/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}/delete-info", h.APINetworkDeleteInfo).Methods("GET")
	api.HandleFunc("/networks/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}/force-delete", h.APIForceDeleteNetwork).Methods("DELETE")
	api.HandleFunc("/network-modal", h.APINetworkModal).Methods("GET")
	api.HandleFunc("/network-modal/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", h.APINetworkModal).Methods("GET")
	api.HandleFunc("/network-delete-modal/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", h.APINetworkDeleteModal).Methods("GET")

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
	api.HandleFunc("/detected-networks-debug", h.APIDetectedNetworksDebug).Methods("GET")
	api.HandleFunc("/networks-debug", h.APINetworksDebug).Methods("GET")
	api.HandleFunc("/network-suggestion", h.APINetworkSuggestion).Methods("POST")

	// Static file serving
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	// 404 handler
	r.NotFoundHandler = http.HandlerFunc(h.NotFound)

	return r
}

func (h *WebHandler) APIRescanDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["id"]

	// TODO: Trigger rescan logic
	w.Write([]byte("<div>Rescan triggered (not implemented yet)</div>"))

	// Return updated modal
	device, _ := h.deviceService.FindByID(deviceID)
	if device != nil {
		if err := h.templates.ExecuteTemplate(w, "components/device-modal.html", device); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (h *WebHandler) APINewScan(w http.ResponseWriter, r *http.Request) {
	// Create a modal for scanning a new IP
	data := struct {
		Title   string
		Message string
		Action  string
	}{
		Title:   "Scan New Device",
		Message: "Would you like to scan this IP address for devices?",
		Action:  "Start Scan",
	}

	modalHTML := `
<div class="flex justify-between items-center mb-4 pb-3 border-b border-green-600">
    <h5 class="text-xl font-bold text-green-500">{{.Title}}</h5>
    <button type="button" class="text-gray-400 hover:text-white text-xl" onclick="closeModal()">
        <i class="ti ti-x"></i>
    </button>
</div>
<div class="mb-4">
    <p class="text-gray-300 mb-4">{{.Message}}</p>
    <div class="bg-blue-600 bg-opacity-10 border border-blue-500 rounded p-3">
        <i class="ti ti-info-circle mr-2 text-blue-400"></i>
        <span class="text-blue-400">This will perform a network scan on the selected IP address to detect any devices.</span>
    </div>
</div>
<div class="flex justify-end gap-2 pt-3 border-t border-green-600">
    <button type="button" class="border border-gray-500 text-gray-300 hover:bg-gray-700 hover:text-white px-3 py-2 rounded text-sm transition-colors" onclick="closeModal()">Cancel</button>
    <button type="button" class="bg-green-600 hover:bg-green-700 text-white px-3 py-2 rounded text-sm transition-colors" onclick="startScan()">{{.Action}}</button>
</div>
<script>
function startScan() {
    // TODO: Implement scan functionality
    alert('Scan functionality not yet implemented');
}
</script>`

	tmpl, err := template.New("new-scan-modal").Parse(modalHTML)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *WebHandler) NotFound(w http.ResponseWriter, r *http.Request) {
	// For unknown routes, serve the main SPA and let JavaScript handle the 404
	h.Index(w, r)
}
