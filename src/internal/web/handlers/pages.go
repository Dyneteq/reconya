package handlers

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"reconya/models"
)

// ServePage serves the main application page with the specified page context
func (h *WebHandler) ServePage(pageName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := h.sessionStore.Get(r, "reconya-session")
		user := h.getUserFromSession(session)
		if user == nil {
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Map root path to dashboard
		if pageName == "" {
			pageName = "dashboard"
		}

		data := PageData{
			Page: pageName,
			User: user,
		}

		log.Printf("DEBUG: Rendering page=%s for path=%s", pageName, r.URL.Path)

		if err := h.templates.ExecuteTemplate(w, "index.html", data); err != nil {
			log.Printf("%s template execution error: %v", pageName, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// Index serves the dashboard page
func (h *WebHandler) Index(w http.ResponseWriter, r *http.Request) {
	h.ServePage("dashboard")(w, r)
}

// Home serves the home/dashboard page with full data
func (h *WebHandler) Home(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", "/login")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Get system status from service
	status, err := h.systemStatusService.GetLatest()
	if err != nil {
		log.Printf("Error getting system status for home page: %v", err)
		status = &models.SystemStatus{
			NetworkID: "N/A",
			PublicIP:  nil,
		}
	} else if status == nil {
		log.Printf("No system status found in database for home page, using fallback")
		status = &models.SystemStatus{
			NetworkID: "N/A",
			PublicIP:  nil,
		}
	}

	// Get current or selected network to determine which network to show
	currentNetwork := h.scanManager.GetSelectedOrCurrentNetwork()
	scanState := h.scanManager.GetState()
	var devices []*models.Device
	var networkCIDR string = "N/A"

	if currentNetwork != nil {
		log.Printf("Home: currentNetwork is not nil, ID: %s", currentNetwork.ID)
		devicesSlice, err := h.deviceService.FindByNetworkID(currentNetwork.ID)
		if err != nil {
			log.Printf("Error getting devices for home page system status %s: %v", currentNetwork.ID, err)
			devices = []*models.Device{}
		} else {
			devices = make([]*models.Device, len(devicesSlice))
			for i := range devicesSlice {
				devices[i] = &devicesSlice[i]
			}
		}
		networkCIDR = currentNetwork.CIDR
	} else {
		log.Println("Home: currentNetwork is nil, falling back to all devices")
		devices, err = h.deviceService.FindAll()
		if err != nil {
			log.Printf("Error getting all devices for home page system status: %v", err)
			devices = []*models.Device{}
		}
	}

	networkMapData := h.buildNetworkMap(devices)

	systemStatusData := &SystemStatusTemplateData{
		SystemStatus: status,
		NetworkCIDR:  networkCIDR,
		NetworkInfo:  networkMapData.NetworkInfo,
		DevicesCount: len(devices),
		ScanState:    &scanState,
	}

	// Get recent event logs
	eventLogSlice, err := h.eventLogService.GetAll(20)
	if err != nil {
		log.Printf("Error getting event logs for home page: %v", err)
		eventLogSlice = []models.EventLog{}
	}

	eventLogs := make([]*models.EventLog, len(eventLogSlice))
	for i := range eventLogSlice {
		eventLogs[i] = &eventLogSlice[i]
	}

	// Get networks list
	networksSlice, err := h.networkService.FindAll()
	if err != nil {
		log.Printf("Error getting networks for home page: %v", err)
		networksSlice = []models.Network{}
	}

	pageData := PageData{
		Page:             "dashboard",
		User:             user,
		SystemStatusData: systemStatusData,
		Devices:          devices,
		EventLogs:        eventLogs,
		Networks:         networksSlice,
		ScanState:        &scanState,
	}

	if err := h.templates.ExecuteTemplate(w, "index.html", pageData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// About serves the about page
func (h *WebHandler) About(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", "/login")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := struct {
		Page    string
		User    *models.User
		Version string
	}{
		Page:    "about",
		User:    user,
		Version: h.getVersionFromPackageJSON(),
	}

	if err := h.templates.ExecuteTemplate(w, "components/about.html", data); err != nil {
		log.Printf("About template execution error: %v", err)
		http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
	}
}

// Devices serves the devices page
func (h *WebHandler) Devices(w http.ResponseWriter, r *http.Request) {
	h.ServePage("devices")(w, r)
}

// Networks serves the networks page
func (h *WebHandler) Networks(w http.ResponseWriter, r *http.Request) {
	h.ServePage("networks")(w, r)
}

// Logs serves the logs page
func (h *WebHandler) Logs(w http.ResponseWriter, r *http.Request) {
	h.ServePage("logs")(w, r)
}

// Alerts serves the alerts page
func (h *WebHandler) Alerts(w http.ResponseWriter, r *http.Request) {
	h.ServePage("alerts")(w, r)
}

// Settings serves the settings page
func (h *WebHandler) Settings(w http.ResponseWriter, r *http.Request) {
	h.ServePage("settings")(w, r)
}

// Login handles login page and authentication
func (h *WebHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		loginTmpl, err := template.ParseFiles(filepath.Join(findPath("templates"), "standalone", "login.html"))
		if err != nil {
			log.Printf("Failed to parse standalone login template: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := struct {
			Page     string
			Error    string
			Username string
		}{
			Page:     "login",
			Error:    "",
			Username: "",
		}
		if err := loginTmpl.Execute(w, data); err != nil {
			log.Printf("Template execution error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Handle POST login
	username := r.FormValue("username")
	password := r.FormValue("password")

	if h.authenticate(username, password) {
		session, _ := h.sessionStore.Get(r, "reconya-session")
		session.Values["user_id"] = username
		session.Values["username"] = username
		session.Save(r, w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	} else {
		data := struct {
			Page     string
			Error    string
			Username string
		}{
			Page:     "login",
			Error:    "Invalid username or password",
			Username: username,
		}
		if err := h.templates.ExecuteTemplate(w, "login.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// Logout handles user logout
func (h *WebHandler) Logout(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	session.Values = make(map[interface{}]interface{})
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
