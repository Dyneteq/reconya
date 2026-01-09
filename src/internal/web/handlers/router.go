package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
)

func (h *WebHandler) SetupRoutes() *mux.Router {
	r := mux.NewRouter()

	// Web pages - only login and dashboard
	r.HandleFunc("/", h.ServePage("dashboard")).Methods("GET")

	// Authentication
	r.HandleFunc("/login", h.Login).Methods("GET", "POST")
	r.HandleFunc("/logout", h.Logout).Methods("POST")

	// API endpoints
	api := r.PathPrefix("/api").Subrouter()

	// Device endpoints
	api.HandleFunc("/devices", h.APIDevices).Methods("GET")
	api.HandleFunc("/devices/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}/modal", h.APIDeviceModal).Methods("GET")
	api.HandleFunc("/devices/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", h.APIUpdateDevice).Methods("PUT")
	api.HandleFunc("/devices/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", h.APIDeleteDevice).Methods("DELETE")

	// Dashboard metrics
	api.HandleFunc("/dashboard-metrics", h.APIDashboardMetrics).Methods("GET")

	// Static file serving
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(findPath("static")))))

	// 404 handler - redirect to dashboard
	r.NotFoundHandler = http.HandlerFunc(h.NotFound)

	return r
}

func (h *WebHandler) NotFound(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
