package network

import (
	"net/http"
)

type NetworkHandlers struct {
	Service *NetworkService
}

func NewNetworkHandlers(service *NetworkService) *NetworkHandlers {
	return &NetworkHandlers{Service: service}
}

func (h *NetworkHandlers) GetNetwork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// This handler is deprecated - networks are now managed through the web interface
	http.Error(w, "This endpoint is deprecated. Use the web interface to manage networks.", http.StatusGone)
}
