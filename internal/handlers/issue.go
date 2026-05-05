package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// UserFormData represents the data submitted from the user form
// Note: Age verification flags are calculated by backend, not sent by frontend
type UserFormData struct {
	GivenName   string `json:"given_name"`
	FamilyName  string `json:"family_name"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
	Address     struct {
		StreetAddress string `json:"street_address"`
		Locality      string `json:"locality"`
		Region        string `json:"region"`
		Country       string `json:"country"`
	} `json:"address"`
	Birthdate string `json:"birthdate"` // Backend will calculate age and is_over_X from this
}

// AgeVerification contains calculated age verification flags
type AgeVerification struct {
	Age      int  `json:"age"`
	IsOver18 bool `json:"is_over_18"`
	IsOver21 bool `json:"is_over_21"`
	IsOver65 bool `json:"is_over_65"`
}

// calculateAgeVerification calculates age and verification flags from birthdate
// This is the SECURE way - backend controls these critical business decisions
func calculateAgeVerification(birthdate string) (*AgeVerification, error) {
	// Parse birthdate
	birth, err := time.Parse("2006-01-02", birthdate)
	if err != nil {
		return nil, fmt.Errorf("invalid birthdate format: %w", err)
	}

	// Calculate age based on current date
	now := time.Now()
	age := now.Year() - birth.Year()

	// Adjust if birthday hasn't occurred this year
	if now.Month() < birth.Month() || (now.Month() == birth.Month() && now.Day() < birth.Day()) {
		age--
	}

	// Calculate verification flags based on actual calculated age
	return &AgeVerification{
		Age:      age,
		IsOver18: age >= 18,
		IsOver21: age >= 21,
		IsOver65: age >= 65,
	}, nil
}

// CredentialRequest represents the complete credential issuance request
type CredentialRequest struct {
	IssuerKey                 interface{} `json:"issuerKey"`
	IssuerDid                 string      `json:"issuerDid"`
	CredentialConfigurationId string      `json:"credentialConfigurationId"`
	CredentialData            interface{} `json:"credentialData"`
	Mapping                   interface{} `json:"mapping"`
	AuthenticationMethod      string      `json:"authenticationMethod"`
	SelectiveDisclosure       interface{} `json:"selectiveDisclosure"`
}

// createCredentialRequest creates a dynamic credential request from user form data
func createCredentialRequest(formData UserFormData) (*CredentialRequest, error) {
	// SECURITY: Calculate age verification in backend, not frontend
	ageVerification, err := calculateAgeVerification(formData.Birthdate)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate age verification: %w", err)
	}

	log.Printf("🔒 Security: Backend calculated age %d for %s %s",
		ageVerification.Age, formData.GivenName, formData.FamilyName)

	// Create the credential data with user input plus BACKEND-calculated verification flags
	credentialData := map[string]interface{}{
		"given_name":   formData.GivenName,
		"family_name":  formData.FamilyName,
		"email":        formData.Email,
		"phone_number": formData.PhoneNumber,
		"address": map[string]interface{}{
			"street_address": formData.Address.StreetAddress,
			"locality":       formData.Address.Locality,
			"region":         formData.Address.Region,
			"country":        formData.Address.Country,
		},
		"birthdate":       formData.Birthdate,
		"is_over_18":      ageVerification.IsOver18, // Backend-calculated, secure
		"is_over_21":      ageVerification.IsOver21, // Backend-calculated, secure
		"is_over_65":      ageVerification.IsOver65, // Backend-calculated, secure
		"citizen_status":  "active",
		"jurisdiction":    "CO-BOG",
		"assurance_level": "low",
	}

	// Standard mapping configuration
	mapping := map[string]interface{}{
		"id":  "<uuid>",
		"iat": "<timestamp-seconds>",
		"nbf": "<timestamp-seconds>",
		"exp": "<timestamp-in-seconds:365d>",
	}

	// Selective disclosure configuration
	selectiveDisclosure := map[string]interface{}{
		"fields": map[string]interface{}{
			"birthdate": map[string]interface{}{
				"sd": true,
			},
			"given_name": map[string]interface{}{
				"sd": true,
			},
			"family_name": map[string]interface{}{
				"sd": true,
			},
			"address": map[string]interface{}{
				"sd": true,
			},
			"phone_number": map[string]interface{}{
				"sd": true,
			},
			"email": map[string]interface{}{
				"sd": true,
			},
		},
		"decoyMode": "NONE",
		"decoys":    0,
	}

	return &CredentialRequest{
		IssuerKey:                 "$ISSUER_KEY", // This will be processed as a template variable
		IssuerDid:                 "$ISSUER_DID",
		CredentialConfigurationId: "identity_credential_vc+sd-jwt",
		CredentialData:            credentialData,
		Mapping:                   mapping,
		AuthenticationMethod:      "PRE_AUTHORIZED",
		SelectiveDisclosure:       selectiveDisclosure,
	}, nil
}

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

	log.Printf("🚀 Starting dynamic credential issuance process...")

	// Parse user form data from request body
	var formData UserFormData
	if err := json.NewDecoder(r.Body).Decode(&formData); err != nil {
		log.Printf("❌ Error parsing form data: %v", err)
		h.writeJSONError(w, http.StatusBadRequest, "Invalid form data: "+err.Error())
		return
	}

	log.Printf("👤 Creating credential for: %s %s (%s)", formData.GivenName, formData.FamilyName, formData.Email)

	// Create dynamic credential request
	credReq, err := createCredentialRequest(formData)
	if err != nil {
		log.Printf("❌ Error creating credential request: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to create credential request")
		return
	}

	// Create temporary file with the dynamic configuration
	tmpFileName, err := createTempCredentialConfig(credReq)
	if err != nil {
		log.Printf("❌ Error creating temp config: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to create credential configuration")
		return
	}
	defer os.Remove(filepath.Join("configs", tmpFileName)) // Cleanup full path

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Issue credential using the issuer service with dynamic config (just filename, not full path)
	issueResp, err := h.IssuerService.IssueCredential(ctx, tmpFileName)
	if err != nil {
		log.Printf("❌ Error during credential issuance: %v", err)
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("✅ Dynamic credential issued successfully for %s %s", formData.GivenName, formData.FamilyName)
	log.Printf("📱 Generated offer URI: %s", issueResp.Offer)

	h.writeJSONResponse(w, issueResp)
}

// createTempCredentialConfig creates a temporary file with the credential configuration
func createTempCredentialConfig(credReq *CredentialRequest) (string, error) {
	// Create JSON manually to match Walt.id format exactly
	jsonTemplate := fmt.Sprintf(`{
  "issuerKey": $ISSUER_KEY,

  "issuerDid": "$ISSUER_DID",

  "credentialConfigurationId": "identity_credential_vc+sd-jwt",

  "credentialData": {
    "given_name": "%s",
    "family_name": "%s",

    "email": "%s",
    "phone_number": "%s",
    "address": {
      "street_address": "%s",
      "locality": "%s",
      "region": "%s",
      "country": "%s"
    },
    "birthdate": "%s",
    "is_over_18": %t,
    "is_over_21": %t,
    "is_over_65": %t,

    "citizen_status": "active",
    "jurisdiction": "CO-BOG",
    "assurance_level": "low"
  },

  "mapping": {
    "id": "<uuid>",
    "iat": "<timestamp-seconds>",
    "nbf": "<timestamp-seconds>",
    "exp": "<timestamp-in-seconds:365d>"
  },

  "authenticationMethod": "PRE_AUTHORIZED",
  "selectiveDisclosure": {
    "fields": {
      "birthdate": {
        "sd": true
      },
      "given_name": {
        "sd": true
      },
      "family_name": {
        "sd": true
      },
      "address": {
        "sd": true
      },
      "phone_number": {
        "sd": true
      },
      "email": {
        "sd": true
      }
    },
    "decoyMode": "NONE",
    "decoys": 0
  }
}`,
		credReq.CredentialData.(map[string]interface{})["given_name"],
		credReq.CredentialData.(map[string]interface{})["family_name"],
		credReq.CredentialData.(map[string]interface{})["email"],
		credReq.CredentialData.(map[string]interface{})["phone_number"],
		credReq.CredentialData.(map[string]interface{})["address"].(map[string]interface{})["street_address"],
		credReq.CredentialData.(map[string]interface{})["address"].(map[string]interface{})["locality"],
		credReq.CredentialData.(map[string]interface{})["address"].(map[string]interface{})["region"],
		credReq.CredentialData.(map[string]interface{})["address"].(map[string]interface{})["country"],
		credReq.CredentialData.(map[string]interface{})["birthdate"],
		credReq.CredentialData.(map[string]interface{})["is_over_18"], // Backend-calculated
		credReq.CredentialData.(map[string]interface{})["is_over_21"], // Backend-calculated
		credReq.CredentialData.(map[string]interface{})["is_over_65"], // Backend-calculated
	)

	// Create temp filename (just the filename, not full path)
	tmpFileName := fmt.Sprintf("temp-req-%d.json", time.Now().UnixNano())

	// Write to configs directory (where LoadAndExpandIssuerRequest expects files)
	tmpFilePath := filepath.Join("configs", tmpFileName)
	if err := os.WriteFile(tmpFilePath, []byte(jsonTemplate), 0644); err != nil {
		return "", fmt.Errorf("failed to write temp config file: %w", err)
	}

	log.Printf("📄 Created temp credential config: %s", tmpFilePath)
	return tmpFileName, nil // Return only filename, not full path
}
