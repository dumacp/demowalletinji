package models

import "time"

// Challenge represents a verification challenge for polling
type Challenge struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	VpPolicyID string    `json:"vp_policy_id"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}
