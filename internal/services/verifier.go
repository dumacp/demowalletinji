package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dumacp/demowalletinji/internal/config"
	"github.com/dumacp/demowalletinji/internal/models"
	"github.com/dumacp/demowalletinji/internal/utils"
)

// VerifierService handles communication with the verifier service
type VerifierService struct {
	client *http.Client
	config *config.VerifierConfig
}

// NewVerifierService creates a new verifier service
func NewVerifierService(client *http.Client, cfg *config.VerifierConfig) *VerifierService {
	return &VerifierService{
		client: client,
		config: cfg,
	}
}

// CreateVerificationRequest creates a new verification request for credentials
func (s *VerifierService) CreateVerificationRequest(ctx context.Context, requestFilePath string) (*models.VerifyResp, error) {
	// Load and expand template
	expanded, err := utils.LoadAndExpandVerifierRequest(requestFilePath)
	if err != nil {
		return nil, fmt.Errorf("verifier request setup failed: %w", err)
	}

	// Create HTTP request
	url := s.config.BaseURL + s.config.VerifyPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(expanded))
	if err != nil {
		return nil, fmt.Errorf("failed to create verifier request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("verifier request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("verifier error %d: %s", resp.StatusCode, string(respBody))
	}

	url = utils.ExtractURLFromResponse(respBody)
	if !strings.HasPrefix(url, "openid4vp://") {
		return nil, fmt.Errorf("unexpected verifier response (not an openid4vp URI): %s", url)
	}

	return &models.VerifyResp{URL: url}, nil
}

// GetVerificationStatus checks the status of a verification process
func (s *VerifierService) GetVerificationStatus(ctx context.Context, sessionID string) (*models.VerifyStatusResp, error) {
	url := fmt.Sprintf("%s%s/%s", s.config.BaseURL, s.config.SessionPath, sessionID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create status request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("status request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status error %d: %s", resp.StatusCode, string(respBody))
	}

	return &models.VerifyStatusResp{
		Status: "received",
		Data:   string(respBody),
	}, nil
}

// BaseURL returns the base URL of the verifier service
func (s *VerifierService) BaseURL() string {
	return s.config.BaseURL
}

// GetSession returns detailed session information from the verifier
func (s *VerifierService) GetSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s%s/%s", s.config.BaseURL, s.config.SessionPath, sessionID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create session request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("session request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("session error %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse JSON response into a map for flexible handling
	var sessionData map[string]interface{}
	if err := json.Unmarshal(respBody, &sessionData); err != nil {
		return nil, fmt.Errorf("failed to parse session response: %w", err)
	}

	return sessionData, nil
}
