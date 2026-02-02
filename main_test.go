package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestLoadAndExpandIssuerRequest(t *testing.T) {
	// Set env vars with actual JSON objects (as they would be exported)
	testIssuerKeyJSON := `{"type":"jwk","jwk":{"kty":"OKP","d":"test","crv":"Ed25519","kid":"test_kid","x":"test_x"}}`
	testIssuerDid := `did:jwk:test_did`

	os.Setenv("ISSUER_KEY", testIssuerKeyJSON)
	os.Setenv("ISSUER_DID", testIssuerDid)

	t.Run("valid_json_after_expansion", func(t *testing.T) {
		expanded, err := loadAndExpandIssuerRequest("req-issuer-sdjwt.json")
		if err != nil {
			t.Fatalf("loadAndExpandIssuerRequest failed: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal([]byte(expanded), &payload); err != nil {
			t.Fatalf("expanded JSON is invalid: %v\nContent:\n%s", err, expanded)
		}

		t.Logf("✅ JSON is valid after expansion")
	})

	t.Run("issuerKey_is_object_not_string", func(t *testing.T) {
		expanded, err := loadAndExpandIssuerRequest("req-issuer-sdjwt.json")
		if err != nil {
			t.Fatalf("loadAndExpandIssuerRequest failed: %v", err)
		}

		var payload map[string]any
		json.Unmarshal([]byte(expanded), &payload)

		// issuerKey should be an OBJECT (map), not a string
		issuerKeyVal, ok := payload["issuerKey"]
		if !ok {
			t.Fatalf("missing issuerKey field")
		}

		issuerKeyObj, isObj := issuerKeyVal.(map[string]any)
		if !isObj {
			t.Fatalf("issuerKey should be a JSON object, but got: %T", issuerKeyVal)
		}

		// Verify structure
		if jwkType, ok := issuerKeyObj["type"].(string); ok && jwkType == "jwk" {
			t.Logf("✅ issuerKey is a JSON object with correct structure")
		} else {
			t.Errorf("issuerKey.type not found or wrong")
		}
	})

	t.Run("issuerDid_is_string", func(t *testing.T) {
		expanded, err := loadAndExpandIssuerRequest("req-issuer-sdjwt.json")
		if err != nil {
			t.Fatalf("loadAndExpandIssuerRequest failed: %v", err)
		}

		var payload map[string]any
		json.Unmarshal([]byte(expanded), &payload)

		issuerDid, ok := payload["issuerDid"].(string)
		if !ok {
			t.Fatalf("issuerDid should be a string, got: %T", payload["issuerDid"])
		}

		if issuerDid == testIssuerDid {
			t.Logf("✅ issuerDid properly expanded")
		} else {
			t.Errorf("issuerDid mismatch. Expected: %s, Got: %s", testIssuerDid, issuerDid)
		}
	})

	t.Run("required_fields_present", func(t *testing.T) {
		expanded, err := loadAndExpandIssuerRequest("req-issuer-sdjwt.json")
		if err != nil {
			t.Fatalf("loadAndExpandIssuerRequest failed: %v", err)
		}

		var payload map[string]any
		json.Unmarshal([]byte(expanded), &payload)

		requiredFields := []string{"issuerKey", "issuerDid", "credentialConfigurationId", "credentialData", "authenticationMethod"}
		for _, field := range requiredFields {
			if _, ok := payload[field]; !ok {
				t.Errorf("missing required field: %s", field)
			}
		}

		if len(requiredFields) == 5 {
			t.Logf("✅ all required fields present")
		}
	})

	t.Run("missing_file", func(t *testing.T) {
		_, err := loadAndExpandIssuerRequest("/nonexistent/file.json")
		if err == nil {
			t.Error("expected error for missing file")
		} else {
			t.Logf("✅ correctly errored for missing file: %v", err)
		}
	})

	t.Run("print_expanded_json", func(t *testing.T) {
		expanded, err := loadAndExpandIssuerRequest("req-issuer-sdjwt.json")
		if err != nil {
			t.Fatalf("loadAndExpandIssuerRequest failed: %v", err)
		}

		var payload map[string]any
		json.Unmarshal([]byte(expanded), &payload)

		prettyJSON, _ := json.MarshalIndent(payload, "", "  ")
		t.Logf("\n\n=== Expanded JSON ===\n%s\n", string(prettyJSON))
	})
}
