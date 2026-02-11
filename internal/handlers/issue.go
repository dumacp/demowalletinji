package handlers

import (
	"context"
	"log"
	"net/http"
	"time"
)

// IssueHandler handles credential issuance requests
func (h *Handler) IssueHandler(w http.ResponseWriter, r *http.Request) {
	h.enableCORS(w)

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	log.Printf("🚀 Starting credential issuance process...")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Issue credential using the issuer service
	issueResp, err := h.IssuerService.IssueCredential(ctx, "req-issuer-sdjwt.json")
	if err != nil {
		log.Printf("❌ Error during credential issuance: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("✅ Credential issued successfully")
	log.Printf("📱 Generated offer URI: %s", issueResp.Offer)

	h.writeJSONResponse(w, issueResp)
}
