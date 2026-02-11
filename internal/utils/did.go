package utils

import (
	"net/url"
	"strings"
)

// ConvertHTTPSClientIDtoDID converts an HTTPS client_id to DID format
// Example: https://verifier.demo.walt.id/openid4vc/verify -> did:web:verifier.demo.walt.id:openid4vc:verify
func ConvertHTTPSClientIDtoDID(clientID string) string {
	// Only convert if it's an HTTPS URL
	if !strings.HasPrefix(clientID, "https://") {
		return clientID
	}

	// Parse the URL
	u, err := url.Parse(clientID)
	if err != nil {
		return clientID
	}

	// Start building DID
	did := "did:web:" + u.Host

	// Add path segments (skip empty ones)
	if u.Path != "" && u.Path != "/" {
		pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
		for _, part := range pathParts {
			if part != "" {
				did += ":" + part
			}
		}
	}

	return did
}
