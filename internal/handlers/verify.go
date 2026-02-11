package handlers

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"
)

// VerifyHandler handles credential verification requests
func (h *Handler) VerifyHandler(w http.ResponseWriter, r *http.Request) {
	h.enableCORS(w)

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	log.Printf("🔍 Starting credential verification process...")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Create verification request using the enhanced verifier service with security policies
	verifyResp, err := h.VerifierService.CreateVerificationRequest(ctx, "req-verifier-sdjwt-enhanced.json")
	if err != nil {
		log.Printf("❌ Error during verification setup: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("✅ Verification request created successfully")
	log.Printf("📱 Generated verification URI: %s", verifyResp.URL)

	// Extract state from the verification URL for frontend compatibility
	state := extractStateFromURL(verifyResp.URL)

	// Create a verification session using the state as session ID
	// This allows frontend polling to work correctly
	if state != "" {
		if err := h.AuthService.CreateVerificationSession(state, "verification"); err != nil {
			log.Printf("⚠️ Warning: could not create verification session: %v", err)
		} else {
			log.Printf("📋 Created verification session with ID: %s", state)
		}
	}

	// Format response for frontend
	response := map[string]interface{}{
		"authRequest": verifyResp.URL,
		"state":       state,
		"url":         verifyResp.URL,
		"sessionUrl":  verifyResp.SessionUrl,
	}

	h.writeJSONResponse(w, response)
}

// extractStateFromURL extracts the state parameter from an OpenID4VP URL
func extractStateFromURL(urlStr string) string {
	if idx := strings.Index(urlStr, "state="); idx != -1 {
		start := idx + 6 // len("state=")
		end := strings.IndexAny(urlStr[start:], "&")
		if end == -1 {
			return urlStr[start:]
		}
		return urlStr[start : start+end]
	}
	return ""
}
