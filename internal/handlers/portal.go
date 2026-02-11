package handlers

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"
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

// MeHandler returns enhanced session information including verifier status and policies
func (h *Handler) MeHandler(w http.ResponseWriter, r *http.Request) {
	h.enableCORS(w)

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	log.Printf("👤 Enhanced user session info requested")

	// Enhanced portal information with verifier status
	response := map[string]interface{}{
		"portal_info": map[string]interface{}{
			"name":         "OpenID4VC Demo Portal",
			"environment":  h.GinMode,
			"auth_method":  "demo_session",
			"capabilities": []string{"credential_issuance", "credential_verification_enhanced"},
		},
		"verification_config": map[string]interface{}{
			"config_file":    "req-verifier-sdjwt-enhanced.json",
			"security_level": "high",
			"active_policies": map[string]interface{}{
				"vp_policies": []string{}, // Empty for SD-JWT-VC compatibility
				"vc_policies": []map[string]interface{}{
					{
						"policy":      "signature",
						"description": "Verify cryptographic signature of credential",
						"status":      "active",
					},
					{
						"policy":      "not-before",
						"description": "Check that credential is not used before its valid date",
						"status":      "active",
					},
					{
						"policy":      "expired",
						"description": "Check that credential has not expired",
						"status":      "active",
					},
					{
						"policy":      "allowed-issuer",
						"description": "Only accept credentials from trusted issuers",
						"status":      "active",
						"trusted_issuers": []string{
							"did:web:devportal.nebulae.com.co:issuers:devportal",
							"did:key:z6MkoLzFfMmVhfQpZZJHFh4TRdNEUbJ8z4PzE4uxFVYYhx8C",
							"did:key:zDnaeW7p9QEsvutCEpWetgrcuTwLhVbMm9HTDUEPTjJ5yvZ9b",
						},
					},
				},
			},
			"required_validations": []string{
				"cryptographic_signature",
				"temporal_validity",
				"issuer_trust",
			},
		},
		"session_info": map[string]interface{}{
			"authenticated":     true,
			"session_type":      "portal_enhanced",
			"assurance_level":   "high", // Upgraded due to enhanced policies
			"supported_wallets": []string{"Lissi", "Inji", "SD-JWT-VC Compatible"},
			"features": map[string]bool{
				"sd_jwt_vc_issuance":   true,
				"openid4vp_verify":     true,
				"walt_id_integration":  true,
				"session_management":   true,
				"enhanced_policies":    true,
				"issuer_trust_control": true,
				"temporal_validation":  true,
				"signature_validation": true,
			},
		},
		"status": map[string]interface{}{
			"server":             "online",
			"enhanced_policies":  "active",
			"verifier_endpoint":  "https://verifier.devportal.nebulae.com.co",
			"last_config_update": "2026-02-11T15:58:00Z",
		},
	}

	h.writeJSONResponse(w, response)
}

// VerifierDetailsHandler returns detailed technical information from Walt.id verifier
func (h *Handler) VerifierDetailsHandler(w http.ResponseWriter, r *http.Request) {
	h.enableCORS(w)

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get session ID from query parameter
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Missing session_id parameter")
		return
	}

	log.Printf("🔍 Technical verifier details requested for session: %s", sessionID)

	// Check our local session first
	session, exists := h.AuthService.GetSession(sessionID)
	if !exists {
		h.writeJSONError(w, http.StatusNotFound, "Session not found")
		return
	}

	// Get detailed information from Walt.id verifier
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	verifierResp, err := h.VerifierService.GetSession(ctx, sessionID)
	if err != nil {
		log.Printf("❌ Error fetching verifier details: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Could not fetch verifier details")
		return
	}

	// Combine our session data with Walt.id details
	response := map[string]interface{}{
		"session_summary": map[string]interface{}{
			"session_id":          sessionID,
			"local_status":        session.Status,
			"verification_result": session.VerificationResult,
			"created_at":          session.CreatedAt,
			"expires_at":          session.ExpiresAt,
			"last_updated":        session.UpdatedAt,
		},
		"verifier_details": verifierResp,
		"metadata": map[string]interface{}{
			"enhanced_policies_active": true,
			"config_file":              "req-verifier-sdjwt-enhanced.json",
			"walt_id_endpoint":         h.VerifierService.BaseURL(),
			"query_timestamp":          time.Now().Format(time.RFC3339),
		},
	}

	h.writeJSONResponse(w, response)
}
