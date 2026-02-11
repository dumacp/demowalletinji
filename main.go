package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultAddr = ":8080"

	issuerBase   = "https://issuer.devportal.nebulae.com.co"
	verifierBase = "https://verifier.devportal.nebulae.com.co"

	issuerIssuePath           = "/openid4vc/sdjwt/issue"
	verifierVerifyPath        = "/openid4vc/verify"
	verifierSessionPathPrefix = "/openid4vc/session/" // + state

	pollTimeout           = 90 * time.Second
	issueRequestTimeout   = 20 * time.Second
	verifyRequestTimeout  = 20 * time.Second
	sessionRequestTimeout = 15 * time.Second
	httpClientTimeout     = 30 * time.Second
)

type server struct {
	mux   *http.ServeMux
	httpc *http.Client

	// in-memory session store (demo-grade)
	mu       sync.Mutex
	sessions map[string]portalSession // portal_session -> session data

	// store verifier challenges (demo-grade)
	chMu       sync.Mutex
	challenges map[string]challenge // state -> challenge
}

type portalSession struct {
	AuthMethod     string    `json:"auth_method"`
	AssuranceLevel string    `json:"assurance_level"`
	LoginAt        time.Time `json:"login_at"`
}

type challenge struct {
	State     string
	CreatedAt time.Time
}

// verificationRequestPayload defines the golden request template for verifier.
// It specifies which credentials to request and what fields to require.
var verificationRequestPayload = map[string]any{
	"request_credentials": []any{
		map[string]any{
			"format": "vc+sd-jwt", // Cambio a dc+sd-jwt para compatibilidad con Lissi
			"vct":    "https://issuer.devportal.nebulae.com.co/draft13/identity_credential",
			"input_descriptor": map[string]any{
				"id":     "citizen-access",
				"format": map[string]any{"vc+sd-jwt": map[string]any{}},
				// "format": map[string]any{"vc+sd-jwt": map[string]any{"alg": []string{"EdDSA"}}},
				"constraints": map[string]any{
					"limit_disclosure": "required",
					"fields": []any{
						// map[string]any{
						// 	"path": []any{"$.vct"},
						// 	"filter": map[string]any{
						// 		"type":    "string",
						// 		"pattern": "https://issuer.devportal.nebulae.com.co/draft13/identity_credential",
						// 	},
						// },
						map[string]any{
							"path": []any{"$.citizen_status"},
							"filter": map[string]any{
								"type":  "string",
								"const": "active",
							},
						},
						map[string]any{
							"path": []any{"$.is_over_18"},
							"filter": map[string]any{
								"type":  "boolean",
								"const": true,
							},
						},
					},
				},
			},
		},
	},
	// "vp_policies": []any{"signature_sd-jwt-vc", "presentation-definition"},
	"vp_policies": []any{"presentation-definition"},
	"vc_policies": []any{"not-before", "expired"},
}

// --- Requests/Responses ---

type issueResp struct {
	Offer string `json:"offer"`
}

// We don’t know exact verifier response schema; we parse flexibly.
type verifyResp struct {
	State                string `json:"state"`
	AuthorizationRequest string `json:"authorizationRequest"`
	SessionUrl           string `json:"sessionUrl"`
	// some implementations might use different keys; we’ll fallback via map.
}

type sessionResp struct {
	State              string          `json:"state"`
	Status             string          `json:"status"`
	VerificationResult *bool           `json:"verificationResult"`
	Raw                json.RawMessage `json:"-"`
}

// --- Helpers ---
// Decodifica el payload del JWT sin verificar firma (solo para extraer client_id)
func extractClientIDFromRequestJWT(jwt string) (string, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid jwt format")
	}
	payloadB64 := parts[1]
	// JWT usa base64url sin padding
	b, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return "", fmt.Errorf("decode jwt payload: %w", err)
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return "", fmt.Errorf("unmarshal jwt payload: %w", err)
	}
	if v, ok := m["client_id"].(string); ok && v != "" {
		return v, nil
	}
	// fallback: a veces usan "iss" como client_id en algunos stacks
	if v, ok := m["iss"].(string); ok && v != "" {
		return v, nil
	}
	return "", fmt.Errorf("client_id not found in jwt payload")
}

// convertHTTPSClientIDtoDID converts an HTTPS client_id to DID format
// Example: https://verifier.demo.walt.id/openid4vc/verify -> did:web:verifier.demo.walt.id:openid4vc:verify
func convertHTTPSClientIDtoDID(clientID string) string {
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

// extractClientIDFromOpenID4VP extracts client_id from openid4vp:// URL
// Example: openid4vp://authorize?client_id=https%3A%2F%2Fverifier.demo.walt.id%2Fopenid4vc%2Fverify&request_uri=...
func extractClientIDFromOpenID4VP(openidURL string) (string, error) {
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

// replaceClientIDInOpenID4VP replaces the client_id parameter in an openid4vp:// URL with a new value
// Example: replaces client_id in openid4vp://authorize?client_id=https%3A%2F%2Fverifier.demo.walt.id%2Fopenid4vc%2Fverify&request_uri=...
func replaceClientIDInOpenID4VP(openidURL string, newClientID string) (string, error) {
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

func buildInjiOID4VPAuthRequest(clientID string, requestURI string) (string, error) {
	u := url.URL{
		Scheme: "openid4vp",
		Host:   "authorize",
	}
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("request_uri", requestURI)

	// Nota: NO agrego scope/response_type/response_mode aquí,
	// porque Inji obtiene todo eso desde el request object (JWT) en request_uri.
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// loadAndExpandIssuerRequest reads req-issuer-sdjwt.json and expands env vars
// treating variables containing JSON objects as raw JSON (not escaped strings).
func loadAndExpandIssuerRequest(filepath string) (string, error) {
	// Read file
	bodyBytes, err := os.ReadFile(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", filepath, err)
	}

	content := string(bodyBytes)

	// Find all $VAR_NAME patterns and replace with env values
	// This matches envsubst behavior: if the value is JSON, it's inserted as-is
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
		return "", fmt.Errorf("invalid JSON after env expansion: %w\nContent:\n%s", err, content)
	}

	fmt.Printf("payload after expansion: %+v\n", content)

	return content, nil
}

func randToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func jsonWrite(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	// Encoding to HTTP response; errors are already sent via WriteHeader
	_ = enc.Encode(v)
}

// extractStringField searches for a field value across multiple possible keys.
// Used for flexible parsing of verifier/issuer responses with varying field names.
func extractStringField(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := data[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// extractOfferFromResponse extracts the credential offer from issuer response.
// The offer may be returned as raw URI or nested in JSON fields.
func extractOfferFromResponse(respBody []byte) string {
	offer := strings.TrimSpace(string(respBody))
	if strings.HasPrefix(offer, "{") {
		// Response is JSON; try common field names
		var m map[string]any
		if err := json.Unmarshal(respBody, &m); err == nil {
			offerKeys := []string{"offer", "openidCredentialOffer", "openid_credential_offer", "credential_offer", "credentialOffer"}
			offer = extractStringField(m, offerKeys...)
		}
	}
	return strings.TrimSpace(offer)
}

// parseVerifierResponse extracts key fields from verifier response,
// trying multiple possible field names for flexibility.
// The verifier may respond with JSON or a direct URI.
func parseVerifierResponse(respBody []byte) verifyResp {
	var vr verifyResp
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
				vr.State = extractStringField(m, "state")
			}
			if vr.AuthorizationRequest == "" {
				authReqKeys := []string{"authorizationRequest", "authorization_request", "authRequest", "request"}
				vr.AuthorizationRequest = extractStringField(m, authReqKeys...)
			}
			if vr.SessionUrl == "" {
				sessionKeys := []string{"sessionUrl", "session_url"}
				vr.SessionUrl = extractStringField(m, sessionKeys...)
			}
		}
	}
	return vr
}

func mustMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		w.Header().Set("Allow", method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func (s *server) setPortalSession(w http.ResponseWriter, data portalSession) (string, error) {
	token, err := randToken(32)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	s.sessions[token] = data
	s.mu.Unlock()

	c := &http.Cookie{
		Name:     "portal_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // keep true since your portal is https; for local http you may need to toggle
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, c)
	return token, nil
}

func (s *server) getPortalSession(r *http.Request) (portalSession, bool) {
	c, err := r.Cookie("portal_session")
	if err != nil || c.Value == "" {
		return portalSession{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ps, ok := s.sessions[c.Value]
	return ps, ok
}

// storeChallenge saves a new verification challenge for polling.
func (s *server) storeChallenge(state string) {
	s.chMu.Lock()
	defer s.chMu.Unlock()
	s.challenges[state] = challenge{State: state, CreatedAt: time.Now()}
}

// isChallengeValid checks if a challenge exists in storage.
func (s *server) isChallengeValid(state string) bool {
	s.chMu.Lock()
	defer s.chMu.Unlock()
	_, ok := s.challenges[state]
	return ok
}

// isChallengeExpired checks if a challenge has exceeded the poll timeout.
func (s *server) isChallengeExpired(state string) bool {
	s.chMu.Lock()
	defer s.chMu.Unlock()
	ch, ok := s.challenges[state]
	if !ok {
		return true
	}
	return time.Since(ch.CreatedAt) > pollTimeout
}

// --- Handlers ---

func (s *server) handleIndex() http.HandlerFunc {
	// serves ./web/index.html
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if !mustMethod(w, r, http.MethodGet) {
			return
		}

		// If you run locally over http://, Secure cookies won't stick.
		// For demo UX, we still serve the page. Session can still be shown server-side.
		http.ServeFile(w, r, filepath.Join("web", "index.html"))
	}
}

func (s *server) handleMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !mustMethod(w, r, http.MethodGet) {
			return
		}
		ps, ok := s.getPortalSession(r)
		if !ok {
			http.Error(w, "not authenticated", http.StatusUnauthorized)
			return
		}
		jsonWrite(w, 200, ps)
	}
}

func (s *server) handleIssue() http.HandlerFunc {
	// POST /demo/issue
	// Calls issuer POST /openid4vc/sdjwt/issue with body from req-issuer-sdjwt.json (env-expanded)
	return func(w http.ResponseWriter, r *http.Request) {
		if !mustMethod(w, r, http.MethodPost) {
			return
		}

		expanded, err := loadAndExpandIssuerRequest("req-issuer-sdjwt.json")
		if err != nil {
			http.Error(w, "issuer request setup failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), issueRequestTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, issuerBase+issuerIssuePath, strings.NewReader(expanded))
		if err != nil {
			http.Error(w, "failed to create issuer request", http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := s.httpc.Do(req)
		if err != nil {
			http.Error(w, "issuer request failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// respBody is used for error reporting; if ReadAll fails, respBody will be empty
		respBody, _ := io.ReadAll(resp.Body)

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			http.Error(w, fmt.Sprintf("issuer error %d: %s", resp.StatusCode, string(respBody)), http.StatusBadGateway)
			return
		}

		offer := extractOfferFromResponse(respBody)
		if !strings.HasPrefix(offer, "openid-credential-offer://") {
			// If issuer ever returns only credential_offer_uri, you could wrap it here.
			http.Error(w, "unexpected issuer response (not an openid-credential-offer URI): "+offer, http.StatusBadGateway)
			return
		}

		jsonWrite(w, 200, issueResp{Offer: offer})

		fmt.Printf("Issued credential offer: %s\n", offer)
	}
}

func (s *server) handleVerify() http.HandlerFunc {
	// POST /demo/verify
	// Calls verifier POST /openid4vc/verify with the verification request payload
	return func(w http.ResponseWriter, r *http.Request) {
		if !mustMethod(w, r, http.MethodPost) {
			return
		}

		b, err := json.Marshal(verificationRequestPayload)
		if err != nil {
			http.Error(w, "failed to marshal verification request", http.StatusInternalServerError)
			return
		}
		fmt.Printf("Verification request payload: %s\n", string(b))

		ctx, cancel := context.WithTimeout(r.Context(), verifyRequestTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, verifierBase+verifierVerifyPath, bytes.NewReader(b))
		if err != nil {
			http.Error(w, "failed to create verifier request", http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("openId4VPProfile", "DEFAULT")
		req.Header.Set("responseMode", "direct_post")
		req.Header.Set("authorizeBaseUrl", "openid4vp://authorize")
		req.Header.Set("Accept", "application/json")

		resp, err := s.httpc.Do(req)
		if err != nil {
			http.Error(w, "verifier request failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// respBody is used for error reporting and response parsing
		respBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			http.Error(w, fmt.Sprintf("verifier error %d: %s", resp.StatusCode, string(respBody)), http.StatusBadGateway)
			return
		}

		respBodyTest := []byte("openid4vp://authorize?client_id=https%3A%2F%2Fverifier.demo.walt.id%2Fopenid4vc%2Fverify&request_uri=https%3A%2F%2Fverifier.demo.walt.id%2Fopenid4vc%2Frequest%2FyzNneSip4DwQ")

		fmt.Printf("respBody: %s\n", string(respBody))
		fmt.Printf("respBody: %s\n", string(respBodyTest))

		vr := parseVerifierResponse(respBody)

		// vr1 := parseVerifierResponse(respBodyTest)

		if vr.State == "" || vr.AuthorizationRequest == "" {
			http.Error(w, "unexpected verifier response: "+string(respBody), http.StatusBadGateway)
			return
		}

		// // 4) GET /openid4vc/request/{state} -> JWT firmado (request object)
		// requestURI := verifierBase + "/openid4vc/request/" + url.PathEscape(vr.State)
		// jwtResp, err := http.NewRequest(http.MethodGet, requestURI, nil)
		// if err != nil {
		// 	http.Error(w, "failed to create request_uri call", http.StatusInternalServerError)
		// 	return
		// }
		// jwtResp.Header.Set("accept", "*/*")

		// jwtHTTPResp, err := s.httpc.Do(jwtResp)
		// if err != nil {
		// 	http.Error(w, "failed to fetch verifier request object: "+err.Error(), http.StatusBadGateway)
		// 	return
		// }
		// defer jwtHTTPResp.Body.Close()
		// jwtBytes, _ := io.ReadAll(jwtHTTPResp.Body)
		// if jwtHTTPResp.StatusCode < 200 || jwtHTTPResp.StatusCode >= 300 {
		// 	http.Error(w, "verifier /request/{id} error: "+string(jwtBytes), http.StatusBadGateway)
		// 	return
		// }

		// requestJWT := strings.TrimSpace(string(jwtBytes))

		// 5) tomar client_id del JWT (no verificamos firma; solo leer claim)
		// clientID, err := extractClientIDFromRequestJWT(requestJWT)
		// if err != nil {
		// 	http.Error(w, "failed to extract client_id from request jwt: "+err.Error(), http.StatusBadGateway)
		// 	return
		// }

		requestURI := vr.AuthorizationRequest

		// 5) extraer client_id del requestURI
		clientID, err := extractClientIDFromOpenID4VP(requestURI)
		if err != nil {
			http.Error(w, "failed to extract client_id from requestURI: "+err.Error(), http.StatusBadGateway)
			return
		}

		// 6) convertir client_id a formato DID para compatibilidad con Inji
		didClientID := convertHTTPSClientIDtoDID(clientID)

		// 7) crear nuevo authRequest reemplazando client_id con el formato DID
		authForInji, err := replaceClientIDInOpenID4VP(requestURI, didClientID)
		if err != nil {
			http.Error(w, "failed to replace client_id in authRequest: "+err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Printf("Original clientID: %s\n", clientID)
		fmt.Printf("DID clientID: %s\n", didClientID)
		fmt.Printf("Original authRequest: %s\n", vr.AuthorizationRequest)
		fmt.Printf("Modified authRequest for Lissi: %s\n", authForInji)

		s.storeChallenge(vr.State)

		// 8) responder al frontend con client_id transformado para Lissi
		jsonWrite(w, 200, map[string]any{
			"state":       vr.State,
			"authRequest": vr.AuthorizationRequest, // ESTE es el que debe ir al QR para Lissi también
			"sessionUrl":  sessionURLFromState(vr.State, vr.SessionUrl),
		})

		fmt.Printf("Created verifier challenge for state: %s\n", vr.State)
	}
}

func sessionURLFromState(state string, sessionUrl string) string {
	if sessionUrl != "" {
		return sessionUrl
	}
	// fallback: construct
	return verifierBase + verifierSessionPathPrefix + state
}

func (s *server) handleSession() http.HandlerFunc {
	// GET /demo/session/{state}
	// Calls verifier GET /openid4vc/session/{state}
	return func(w http.ResponseWriter, r *http.Request) {
		if !mustMethod(w, r, http.MethodGet) {
			return
		}

		state := strings.TrimPrefix(r.URL.Path, "/demo/session/")
		state = strings.TrimSpace(state)
		if state == "" {
			http.Error(w, "missing state", http.StatusBadRequest)
			return
		}

		// Validate challenge exists and hasn't timed out
		if !s.isChallengeValid(state) {
			http.Error(w, "unknown state", http.StatusNotFound)
			return
		}
		if s.isChallengeExpired(state) {
			http.Error(w, "challenge timeout", http.StatusGone)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), sessionRequestTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, verifierBase+verifierSessionPathPrefix+state, nil)
		if err != nil {
			http.Error(w, "failed to create session request", http.StatusInternalServerError)
			return
		}
		req.Header.Set("Accept", "application/json")

		resp, err := s.httpc.Do(req)
		if err != nil {
			http.Error(w, "verifier session request failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// raw is used for both parsing and error reporting
		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			http.Error(w, fmt.Sprintf("verifier session error %d: %s", resp.StatusCode, string(raw)), http.StatusBadGateway)
			return
		}

		// Parse session response; keep raw for debugging
		var sr sessionResp
		// Session response should have minimal structure; if unmarshal fails,
		// we still have raw data for inspection
		_ = json.Unmarshal(raw, &sr)
		sr.Raw = raw

		// If verificationResult == true -> create portal session and return success
		if sr.VerificationResult != nil && *sr.VerificationResult {
			_, err := s.setPortalSession(w, portalSession{
				AuthMethod:     "oid4vp",
				AssuranceLevel: "low",
				LoginAt:        time.Now(),
			})
			if err != nil {
				http.Error(w, "failed to create portal session", http.StatusInternalServerError)
				return
			}
		}

		// Return normalized + raw for debugging
		out := map[string]any{
			"state":              sr.State,
			"status":             sr.Status,
			"verificationResult": sr.VerificationResult,
			"raw":                json.RawMessage(raw),
		}
		jsonWrite(w, 200, out)
	}
}

func main() {
	addr := os.Getenv("PORTAL_ADDR")
	if addr == "" {
		addr = defaultAddr
	}

	s := &server{
		mux:        http.NewServeMux(),
		httpc:      &http.Client{Timeout: httpClientTimeout},
		sessions:   make(map[string]portalSession),
		challenges: make(map[string]challenge),
	}

	// Routes
	s.mux.HandleFunc("/", s.handleIndex())
	s.mux.HandleFunc("/demo/issue", s.handleIssue())
	s.mux.HandleFunc("/demo/verify", s.handleVerify())
	s.mux.HandleFunc("/demo/session/", s.handleSession())
	s.mux.HandleFunc("/demo/me", s.handleMe())

	// CORS for local dev (simple)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Accept")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		s.mux.ServeHTTP(w, r)
	})

	log.Printf("Demo portal listening on %s", addr)
	log.Printf("Issuer: %s%s", issuerBase, issuerIssuePath)
	log.Printf("Verifier: %s%s", verifierBase, verifierVerifyPath)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
