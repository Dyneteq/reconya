package eventlog

import (
	"encoding/json"
	"net/http"
)

type EventLogHandlers struct {
	Service *EventLogService
}

func NewEventLogHandlers(service *EventLogService) *EventLogHandlers {
	return &EventLogHandlers{Service: service}
}

func (h *EventLogHandlers) FindLatest(w http.ResponseWriter, r *http.Request) {
	eventLogs, err := h.Service.GetAll(10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(eventLogs)
}

func (h *EventLogHandlers) FindAllByDeviceId(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Path[len("/event-log/"):]
	eventLogs, err := h.Service.GetAllByDeviceId(deviceID, 8)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(eventLogs)
}
