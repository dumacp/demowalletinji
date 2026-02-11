package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dumacp/demowalletinji/internal/config"
	"github.com/dumacp/demowalletinji/internal/models"
	"github.com/dumacp/demowalletinji/internal/utils"
)

// IssuerService handles communication with the issuer service
type IssuerService struct {
	client *http.Client
	config *config.IssuerConfig
}

// NewIssuerService creates a new issuer service
func NewIssuerService(client *http.Client, cfg *config.IssuerConfig) *IssuerService {
	return &IssuerService{
		client: client,
		config: cfg,
	}
}

// IssueCredential issues a new credential by calling the issuer API
func (s *IssuerService) IssueCredential(ctx context.Context, requestFilePath string) (*models.IssueResp, error) {
	// Load and expand template
	expanded, err := utils.LoadAndExpandIssuerRequest(requestFilePath)
	if err != nil {
		return nil, fmt.Errorf("issuer request setup failed: %w", err)
	}

	// Create HTTP request
	url := s.config.BaseURL + s.config.IssuePath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(expanded))
	if err != nil {
		return nil, fmt.Errorf("failed to create issuer request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("issuer request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("issuer error %d: %s", resp.StatusCode, string(respBody))
	}

	offer := utils.ExtractOfferFromResponse(respBody)
	if !strings.HasPrefix(offer, "openid-credential-offer://") {
		return nil, fmt.Errorf("unexpected issuer response (not an openid-credential-offer URI): %s", offer)
	}

	return &models.IssueResp{Offer: offer}, nil
}
