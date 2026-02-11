package utils

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/dumacp/demowalletinji/internal/models"
)

// ExtractStringField searches for a field value across multiple possible keys.
// Used for flexible parsing of verifier/issuer responses with varying field names.
func ExtractStringField(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := data[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// ExtractOfferFromResponse extracts the credential offer from issuer response.
// The offer may be returned as raw URI or nested in JSON fields.
func ExtractOfferFromResponse(respBody []byte) string {
	offer := strings.TrimSpace(string(respBody))
	if strings.HasPrefix(offer, "{") {
		// Response is JSON; try common field names
		var m map[string]any
		if err := json.Unmarshal(respBody, &m); err == nil {
			offerKeys := []string{"offer", "openidCredentialOffer", "openid_credential_offer", "credential_offer", "credentialOffer"}
			offer = ExtractStringField(m, offerKeys...)
		}
	}
	return strings.TrimSpace(offer)
}

// ParseVerifierResponse extracts key fields from verifier response,
// trying multiple possible field names for flexibility.
func ParseVerifierResponse(respBody []byte) models.VerifyResp {
	var vr models.VerifyResp
	// First attempt: try struct tags (if they match)
	_ = json.Unmarshal(respBody, &vr)

	// Check if response is a direct URI (starts with openid4vp://)
	if vr.State == "" && vr.AuthorizationRequest == "" {
		rawStr := strings.TrimSpace(string(respBody))
		if strings.HasPrefix(rawStr, "openid4vp://") {
			// Response is a direct URI; extract state from the query parameters
			vr.AuthorizationRequest = rawStr

			// First try to parse state from the URI query parameters
			if idx := strings.Index(rawStr, "state="); idx != -1 {
				stateStart := idx + 6 // len("state=")
				stateEnd := strings.IndexAny(rawStr[stateStart:], "&")
				if stateEnd == -1 {
					vr.State = rawStr[stateStart:]
				} else {
					vr.State = rawStr[stateStart : stateStart+stateEnd]
				}
			}

			// If state not found in query params, try to extract from request_uri
			if vr.State == "" {
				if idx := strings.Index(rawStr, "request_uri="); idx != -1 {
					requestURIStart := idx + 12 // len("request_uri=")
					requestURIEnd := strings.IndexAny(rawStr[requestURIStart:], "&")
					var requestURI string
					if requestURIEnd == -1 {
						requestURI = rawStr[requestURIStart:]
					} else {
						requestURI = rawStr[requestURIStart : requestURIStart+requestURIEnd]
					}

					// URL decode the request_uri
					if decodedURI, err := url.QueryUnescape(requestURI); err == nil {
						// Extract state from the last part of the path
						if lastSlash := strings.LastIndex(decodedURI, "/"); lastSlash != -1 {
							vr.State = decodedURI[lastSlash+1:]
						}
					}
				}
			}
			return vr
		}
	}

	// Fallback: search for fields manually if not found in JSON
	if vr.State == "" || vr.AuthorizationRequest == "" {
		var m map[string]any
		if err := json.Unmarshal(respBody, &m); err == nil {
			if vr.State == "" {
				vr.State = ExtractStringField(m, "state")
			}
			if vr.AuthorizationRequest == "" {
				authReqKeys := []string{"authorizationRequest", "authorization_request", "authRequest", "request"}
				vr.AuthorizationRequest = ExtractStringField(m, authReqKeys...)
			}
			if vr.SessionUrl == "" {
				sessionKeys := []string{"sessionUrl", "session_url"}
				vr.SessionUrl = ExtractStringField(m, sessionKeys...)
			}
		}
	}
	return vr
}

// LoadAndExpandIssuerRequest reads a JSON file and expands env vars
func LoadAndExpandIssuerRequest(filepath string) (string, error) {
	// Read file from configs directory
	fullPath := "configs/" + filepath
	return loadAndExpandTemplate(fullPath, true)
}

// LoadAndExpandVerifierRequest reads a JSON file and expands env vars for verifier
func LoadAndExpandVerifierRequest(filepath string) (string, error) {
	// Read file from configs directory
	fullPath := "configs/" + filepath
	return loadAndExpandTemplate(fullPath, false)
}

// loadAndExpandTemplate is a helper function to load and expand JSON templates
func loadAndExpandTemplate(filepath string, isIssuer bool) (string, error) {
	// Read file
	bodyBytes, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}

	content := string(bodyBytes)

	// Replace special variables first
	if strings.Contains(content, "$RANDOM_ID") {
		randomID, _ := RandToken(16)
		content = strings.ReplaceAll(content, "$RANDOM_ID", randomID)
	}

	// Find all $VAR_NAME patterns and replace with env values
	for _, envStr := range os.Environ() {
		parts := strings.SplitN(envStr, "=", 2)
		if len(parts) != 2 {
			continue
		}
		varName := parts[0]
		varValue := parts[1]

		// Replace $VAR_NAME with its value
		placeholder := "$" + varName
		content = strings.ReplaceAll(content, placeholder, varValue)
	}

	// Validate JSON structure
	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return "", err
	}

	return content, nil
}

// ExtractURLFromResponse extracts the verification URL from verifier response.
func ExtractURLFromResponse(respBody []byte) string {
	resp := ParseVerifierResponse(respBody)
	if resp.AuthorizationRequest != "" {
		return resp.AuthorizationRequest
	}

	// Fallback: try to parse as raw URL
	url := strings.TrimSpace(string(respBody))
	if strings.HasPrefix(url, "openid4vp://") {
		return url
	}

	return ""
}

// RandToken generates a random token of n bytes, base64url encoded
func RandToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// JSONWrite writes a JSON response to http.ResponseWriter
func JSONWrite(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// MustMethod validates HTTP method and returns error response if invalid
func MustMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		w.Header().Set("Allow", method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}
