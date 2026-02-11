package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// ExtractClientIDFromRequestJWT extracts client_id from JWT payload without signature verification
func ExtractClientIDFromRequestJWT(jwt string) (string, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid jwt format")
	}

	payloadB64 := parts[1]
	// JWT uses base64url without padding
	b, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return "", fmt.Errorf("decode jwt payload: %w", err)
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return "", fmt.Errorf("unmarshal jwt payload: %w", err)
	}

	// Try "client_id" first
	if v, ok := m["client_id"].(string); ok && v != "" {
		return v, nil
	}

	// fallback: sometimes use "iss" as client_id in some stacks
	if v, ok := m["iss"].(string); ok && v != "" {
		return v, nil
	}

	return "", fmt.Errorf("client_id not found in jwt payload")
}

// ExtractClientIDFromOpenID4VP extracts client_id from openid4vp:// URL
func ExtractClientIDFromOpenID4VP(openidURL string) (string, error) {
	if !strings.HasPrefix(openidURL, "openid4vp://") {
		return "", fmt.Errorf("not an openid4vp URL")
	}

	// Parse the URL
	u, err := url.Parse(openidURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse openid4vp URL: %w", err)
	}

	// Get client_id from query parameters
	clientID := u.Query().Get("client_id")
	if clientID == "" {
		return "", fmt.Errorf("client_id not found in URL")
	}

	return clientID, nil
}

// ReplaceClientIDInOpenID4VP replaces the client_id parameter in an openid4vp:// URL with a new value
func ReplaceClientIDInOpenID4VP(openidURL string, newClientID string) (string, error) {
	if !strings.HasPrefix(openidURL, "openid4vp://") {
		return "", fmt.Errorf("not an openid4vp URL")
	}

	// Parse the URL
	u, err := url.Parse(openidURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse openid4vp URL: %w", err)
	}

	// Get query parameters
	q := u.Query()
	// Replace client_id with new value
	q.Set("client_id", newClientID)
	// Rebuild the URL with updated query parameters
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// BuildInjiOID4VPAuthRequest builds an OpenID4VP authorization request for Inji wallet
func BuildInjiOID4VPAuthRequest(clientID string, requestURI string) (string, error) {
	u := url.URL{
		Scheme: "openid4vp",
		Host:   "authorize",
	}

	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("request_uri", requestURI)
	// Note: Don't add scope/response_type/response_mode here,
	// because Inji gets all that from the request object (JWT) in request_uri.

	u.RawQuery = q.Encode()
	return u.String(), nil
}