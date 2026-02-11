package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/dumacp/demowalletinji/internal/services"
)

// Handler contains all dependencies for HTTP handlers
type Handler struct {
	IssuerService   *services.IssuerService
	VerifierService *services.VerifierService
	AuthService     *services.AuthService
	GinMode         string
}

// NewHandler creates a new handler with all services
func NewHandler(issuerSvc *services.IssuerService, verifierSvc *services.VerifierService, authSvc *services.AuthService, ginMode string) *Handler {
	return &Handler{
		IssuerService:   issuerSvc,
		VerifierService: verifierSvc,
		AuthService:     authSvc,
		GinMode:         ginMode,
	}
}

// writeJSONError writes an error response as JSON
func (h *Handler) writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   message,
		"status":  statusCode,
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// writeJSONResponse writes a successful response as JSON
func (h *Handler) writeJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response")
	}
}

// enableCORS sets CORS headers for requests
func (h *Handler) enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}