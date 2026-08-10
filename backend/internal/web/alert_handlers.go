package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"reconya/db"
	"reconya/models"

	"github.com/gorilla/mux"
)

// APIAlerts returns the alert feed plus the per-severity counts the console's
// header tile renders.
//
// Query parameters:
//
//	state=open|acked|resolved  repeatable; defaults to open+acked, because the
//	                           feed shows acknowledged alerts greyed out rather
//	                           than hiding them
//	severity=critical|…        repeatable
//	limit=<n>                  defaults to 100
func (h *WebHandler) APIAlerts(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := models.AlertQuery{Limit: 100}

	for _, s := range r.URL.Query()["state"] {
		query.States = append(query.States, models.AlertState(strings.ToLower(s)))
	}
	if len(query.States) == 0 {
		query.States = []models.AlertState{models.AlertStateOpen, models.AlertStateAcked}
	}

	for _, s := range r.URL.Query()["severity"] {
		query.Severities = append(query.Severities, models.AlertSeverity(strings.ToLower(s)))
	}

	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			query.Limit = n
		}
	}

	alerts, err := h.alertService.List(r.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Counts are always open-only regardless of the state filter: they drive the
	// header tile, which is about what still needs attention.
	counts, err := h.alertService.Counts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alerts":     alerts,
		"counts":     counts,
		"open_count": counts.Total(),
	})
}

// APIAckAlert acknowledges a single alert.
func (h *WebHandler) APIAckAlert(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id := mux.Vars(r)["id"]
	if id == "" {
		http.Error(w, "alert id is required", http.StatusBadRequest)
		return
	}

	err := h.alertService.Ack(r.Context(), id)
	if err == db.ErrNotFound {
		http.Error(w, "alert not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// APIAckAllAlerts acknowledges every open alert.
func (h *WebHandler) APIAckAllAlerts(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	n, err := h.alertService.AckAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "acknowledged": n})
}

// APIDeviceEvents returns the activity trail for one device, which the console
// drawer's HISTORY tab renders.
func (h *WebHandler) APIDeviceEvents(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	user := h.getUserFromSession(session)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	deviceID := mux.Vars(r)["id"]
	if deviceID == "" {
		http.Error(w, "device id is required", http.StatusBadRequest)
		return
	}

	logs, err := h.eventLogService.GetAllByDeviceId(deviceID, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"logs": logs})
}
