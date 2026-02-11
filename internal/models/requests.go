package models

import (
	"encoding/json"
)

// IssueResp represents the response from credential issuance
type IssueResp struct {
	Offer string `json:"offer"`
}

// VerifyResp represents the response from verification initiation
type VerifyResp struct {
	URL                  string `json:"url"`
	State                string `json:"state"`
	AuthorizationRequest string `json:"authorizationRequest"`
	SessionUrl           string `json:"sessionUrl"`
}

// SessionResp represents the response from session polling
type SessionResp struct {
	State              string          `json:"state"`
	Status             string          `json:"status"`
	VerificationResult *bool           `json:"verificationResult"`
	Raw                json.RawMessage `json:"-"`
}

// VerificationRequest represents the verification request payload
type VerificationRequest struct {
	RequestCredentials []RequestCredential `json:"request_credentials"`
	VpPolicies         []string            `json:"vp_policies"`
	VcPolicies         []string            `json:"vc_policies"`
}

// RequestCredential represents a single credential request
type RequestCredential struct {
	Format          string           `json:"format"`
	VCT             string           `json:"vct"`
	InputDescriptor *InputDescriptor `json:"input_descriptor"`
}

// InputDescriptor represents the input descriptor for credential requests
type InputDescriptor struct {
	ID          string                 `json:"id"`
	Format      map[string]interface{} `json:"format"`
	Constraints *Constraints           `json:"constraints"`
}

// Constraints represents the constraints for credential requests
type Constraints struct {
	LimitDisclosure string  `json:"limit_disclosure"`
	Fields          []Field `json:"fields"`
}

// Field represents a field constraint
type Field struct {
	Path   []string               `json:"path"`
	Filter map[string]interface{} `json:"filter"`
}

// VerifyStatusResp represents the response from verification status check
type VerifyStatusResp struct {
	Status string `json:"status"`
	Data   string `json:"data"`
}
