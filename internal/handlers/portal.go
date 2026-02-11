package handlers

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
)

// PortalHandler serves the main portal interface
func (h *Handler) PortalHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	log.Printf("🌐 Serving portal interface")

	// Template data
	data := struct {
		Title       string
		Environment string
		Features    []string
	}{
		Title:       "OpenID4VC Demo Portal",
		Environment: h.GinMode,
		Features: []string{
			"SD-JWT-VC Credential Issuance",
			"OpenID4VP Verification",
			"Walt.id Stack Integration",
			"Mobile Wallet Support (Lissi, Inji)",
			"Session Management",
		},
	}

	// Parse and execute template
	tmplPath := filepath.Join("configs", "web", "index.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Printf("❌ Error parsing template: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Template error")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("❌ Error executing template: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Template execution error")
		return
	}

	log.Printf("✅ Portal interface served successfully")
}

// HealthHandler provides a health check endpoint
func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	h.enableCORS(w)

	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	h.writeJSONResponse(w, map[string]interface{}{
		"status":  "healthy",
		"service": "openid4vc-demo-portal",
		"mode":    h.GinMode,
	})
}

// MeHandler returns current user session information in a clean professional format
func (h *Handler) MeHandler(w http.ResponseWriter, r *http.Request) {
	h.enableCORS(w)

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	log.Printf("👤 User session info requested")

	// For demo purposes, return a clean session info
	// In a real application, you would check authentication tokens/cookies here
	response := map[string]interface{}{
		"authenticated":     true,
		"auth_method":       "demo_session",
		"assurance_level":   "low",
		"session_type":      "portal_demo",
		"capabilities":      []string{"credential_issuance", "credential_verification"},
		"environment":       h.GinMode,
		"portal_name":       "OpenID4VC Demo Portal",
		"supported_wallets": []string{"Lissi", "Inji"},
		"features": map[string]bool{
			"sd_jwt_vc_issuance":  true,
			"openid4vp_verify":    true,
			"walt_id_integration": true,
			"session_management":  true,
		},
	}

	h.writeJSONResponse(w, response)
}
