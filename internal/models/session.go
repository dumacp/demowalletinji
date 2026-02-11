package models

import "time"

// Session represents an active session for credential operations
type Session struct {
	ID                 string    `json:"id"`
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	VerificationResult *bool     `json:"verificationResult,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	ExpiresAt          time.Time `json:"expires_at"`
}

// PortalSession represents a user session in the portal
type PortalSession struct {
	AuthMethod     string    `json:"auth_method"`
	AssuranceLevel string    `json:"assurance_level"`
	LoginAt        time.Time `json:"login_at"`
}
