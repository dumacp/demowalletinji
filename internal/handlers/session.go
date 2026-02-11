package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"path"
	"strings"
	"time"
)

// SessionHandler handles session management (GET /demo/session/{sessionId})
func (h *Handler) SessionHandler(w http.ResponseWriter, r *http.Request) {
	h.enableCORS(w)

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract session ID from path
	sessionID := strings.TrimPrefix(path.Clean(r.URL.Path), "/demo/session/")
	if sessionID == "" || sessionID == "/demo/session" {
		h.writeJSONError(w, http.StatusBadRequest, "Session ID required")
		return
	}

	log.Printf("📊 Checking session status for ID: %s", sessionID)

	// Get session from auth service
	session, exists := h.AuthService.GetSession(sessionID)
	if !exists {
		log.Printf("❌ Session not found or expired: %s", sessionID)
		h.writeJSONError(w, http.StatusNotFound, "Session not found or expired")
		return
	}

	// If this is a verification session, check the verifier for updates
	if session.Type == "verification" && session.VerificationResult == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		log.Printf("🔍 Checking verification status with verifier for session: %s", sessionID)
		statusResp, err := h.VerifierService.GetVerificationStatus(ctx, sessionID)
		if err != nil {
			log.Printf("⚠️ Could not get verification status: %v", err)
		} else {
			// Parse the verification result from verifier response (without logging full response)
			verificationResult := parseVerificationResult(statusResp.Data)
			if verificationResult != nil {
				log.Printf("✅ Verification completed with result: %t", *verificationResult)
				session.VerificationResult = verificationResult
				session.Status = "completed"
				session.UpdatedAt = time.Now()
				h.AuthService.UpdateSession(sessionID, session)
			} else {
				log.Printf("⏳ Verification still pending - no presentations found yet")
			}
		}
	}

	log.Printf("✅ Session found - Status: %s, Type: %s", session.Status, session.Type)

	// Return session information
	response := map[string]interface{}{
		"sessionId": session.ID,
		"status":    session.Status,
		"type":      session.Type,
		"createdAt": session.CreatedAt,
		"updatedAt": session.UpdatedAt,
		"expiresAt": session.ExpiresAt,
	}

	// Include verification result if available
	if session.VerificationResult != nil {
		response["verificationResult"] = *session.VerificationResult

		// Add professional completion details for successful verifications
		if *session.VerificationResult {
			response["message"] = "Credential verification completed successfully"
			response["result"] = "verified"
		} else {
			response["message"] = "Credential verification failed"
			response["result"] = "rejected"
		}
	} else if session.Type == "verification" {
		response["message"] = "Verification in progress - please present your credentials"
		response["result"] = "pending"
	}

	h.writeJSONResponse(w, response)
}

// CreateSessionHandler handles session creation (POST /demo/session)
func (h *Handler) CreateSessionHandler(w http.ResponseWriter, r *http.Request) {
	h.enableCORS(w)

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse request body to get session type
	var req struct {
		Type string `json:"type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Type = "default" // Default session type
	}

	log.Printf("🆕 Creating new session of type: %s", req.Type)

	// Create session using auth service
	session, err := h.AuthService.CreateSession(req.Type)
	if err != nil {
		log.Printf("❌ Error creating session: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	log.Printf("✅ Session created successfully - ID: %s", session.ID)

	h.writeJSONResponse(w, map[string]interface{}{
		"sessionId": session.ID,
		"status":    session.Status,
		"type":      session.Type,
		"createdAt": session.CreatedAt,
		"expiresAt": session.ExpiresAt,
	})
}

// parseVerificationResult extracts verification result from verifier response
func parseVerificationResult(data string) *bool {
	var response map[string]interface{}
	if err := json.Unmarshal([]byte(data), &response); err != nil {
		log.Printf("❌ Failed to parse verifier response: %v", err)
		return nil
	}

	// Walt.id verifier specific checks:

	// 1. PRIORITY: Check for direct verificationResult field (Walt.id newer format)
	if verificationResult, ok := response["verificationResult"].(bool); ok {
		log.Printf("🎯 Verification result: %t", verificationResult)
		return &verificationResult
	}

	// 2. Check for tokenResponse containing VP data (Walt.id newer format)
	if tokenResponse, ok := response["tokenResponse"].(map[string]interface{}); ok {
		log.Printf("📋 Found credential presentation")

		// Check for vp_token within tokenResponse
		if vpToken := tokenResponse["vp_token"]; vpToken != nil {
			log.Printf("✅ Credential verification successful")
			result := true
			return &result
		}

		// Check for presentation_submission within tokenResponse
		if presSubmission := tokenResponse["presentation_submission"]; presSubmission != nil {
			log.Printf("✅ Credential verification successful")
		}
	}

	// 3. Check for presentations array (legacy format)
	if presentations, ok := response["presentations"].([]interface{}); ok {
		if len(presentations) > 0 {
			log.Printf("✅ Credential verification successful (legacy format)")
			result := true
			return &result
		}
	}

	// 4. Check for VP or presentation data structures at root level
	if vpToken := response["vp_token"]; vpToken != nil {
		log.Printf("✅ Credential verification successful")
		result := true
		return &result
	}

	if presSubmission := response["presentation_submission"]; presSubmission != nil {
		log.Printf("✅ Credential verification successful")
		result := true
		return &result
	}

	// 5. Check for explicit status indicators
	if verified, ok := response["verified"].(bool); ok {
		log.Printf("🎯 Verification status: %t", verified)
		return &verified
	}

	if status, ok := response["status"].(string); ok {
		log.Printf("📊 Verification status: %s", status)
		if status == "completed" || status == "verified" || status == "success" {
			result := true
			return &result
		}
		if status == "failed" || status == "rejected" || status == "error" {
			result := false
			return &result
		}
	}

	// 6. If response contains only presentationDefinition but no verification data, it's still pending
	if _, hasId := response["id"]; hasId {
		if _, hasPD := response["presentationDefinition"]; hasPD {
			if _, hasVR := response["verificationResult"]; !hasVR {
				if _, hasTR := response["tokenResponse"]; !hasTR {
					log.Printf("⏳ Verification pending - awaiting credential presentation")
					return nil
				}
			}
		}
	}

	log.Printf("⏳ Verification status unknown - continuing to monitor")
	return nil
}
