package web

import (
	"html/template"
	"net/http"

	"github.com/gorilla/mux"
)

func (h *WebHandler) SetupRoutes() *mux.Router {
	r := mux.NewRouter()

	// Web pages
	r.HandleFunc("/", h.Index).Methods("GET")
	r.HandleFunc("/home", h.Home).Methods("GET")
	r.HandleFunc("/login", h.Login).Methods("GET", "POST")
	r.HandleFunc("/logout", h.Logout).Methods("POST")
	r.HandleFunc("/targets", h.Targets).Methods("GET")

	// Page routes - each serves with specific page context
	r.HandleFunc("/devices", h.Devices).Methods("GET")
	r.HandleFunc("/logs", h.Logs).Methods("GET")
	r.HandleFunc("/networks", h.Networks).Methods("GET")
	r.HandleFunc("/alerts", h.Alerts).Methods("GET")
	r.HandleFunc("/settings", h.Settings).Methods("GET")
	r.HandleFunc("/about", h.About).Methods("GET")

	// HTMX API endpoints
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
	api.HandleFunc("/world-map", h.APIWorldMap).Methods("GET")
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

	// 404 handler
	r.NotFoundHandler = http.HandlerFunc(h.NotFound)

	return r
}

func (h *WebHandler) Targets(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := PageData{
		Page: "targets",
		User: user,
	}

	if err := h.templates.ExecuteTemplate(w, "targets.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *WebHandler) APIRescanDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["id"]

	// TODO: Trigger rescan logic
	w.Write([]byte("<div>Rescan triggered (not implemented yet)</div>"))

	// Return updated modal
	device, _ := h.deviceService.FindByID(deviceID)
	if device != nil {
		if err := h.templates.ExecuteTemplate(w, "device-modal.html", device); err != nil {
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
<div class="modal-header">
    <h5 class="modal-title text-success">{{.Title}}</h5>
    <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal"></button>
</div>
<div class="modal-body">
    <p>{{.Message}}</p>
    <div class="alert alert-info">
        <i class="bi bi-info-circle me-2"></i>
        This will perform a network scan on the selected IP address to detect any devices.
    </div>
</div>
<div class="modal-footer">
    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">Cancel</button>
    <button type="button" class="btn btn-success" onclick="startScan()">{{.Action}}</button>
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
