# Guía de Integración con Walt.id APIs

Esta guía documentas los endpoints específicos de Walt.id utilizados en el proyecto y cómo integrarlos.

## 1. Emisión de Credenciales (Issuer API)

### Endpoint de Emisión
```
POST https://issuer.devportal.nebulae.com.co/openid4vc/sdjwtvc/issue
```

### Headers requeridos:
```
Content-Type: application/json
```

### Request Body ejemplo:
```json
{
  "issuerDid": "did:key:z6MkoLzFfMmVhfQpZZJHFh4TRdNEUbJ8z4PzE4uxFVYYhx8C",
  "subjectDid": "did:key:z6MkvU7e3KpoKG29hoJAbzQ9PN9VNigrG6fGxgtYHREoavD5",
  "schemaId": "VerifiableCredential",
  "claimData": {
    "name": "Juan Pérez",
    "email": "juan.perez@ejemplo.com",
    "documentNumber": "12345678",
    "birthDate": "1990-01-01"
  }
}
```

### Response ejemplo:
```json
{
  "format": "vc+sd-jwt",
  "credential": "eyJ0eXAiOiJ2Yy1zZC1qd3...",
  "credentialResponse": {
    "format": "vc+sd-jwt",
    "credential": "eyJ0eXAiOiJ2Yy1zZC1qd3..."
  },
  "qrCodeUri": "openid-credential-offer://..."
}
```

### Implementación en Go:
```go
func (s *IssuerService) IssueCredential(request IssueCredentialRequest) (*IssueCredentialResponse, error) {
    url := fmt.Sprintf("%s/openid4vc/sdjwtvc/issue", s.config.WaltIDIssuerURL)
    
    payload, err := json.Marshal(request)
    if err != nil {
        return nil, err
    }
    
    resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var response IssueCredentialResponse
    if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
        return nil, err
    }
    
    return &response, nil
}
```

## 2. Verificación de Credenciales (Verifier API)

### 2.1 Inicializar Sesión de Verificación

```
POST https://verifier.devportal.nebulae.com.co/openid4vc/verify
```

### Headers requeridos:
```
Content-Type: application/json
openId4VPProfile: DEFAULT  # ⚠️ IMPORTANTE para compatibilidad con Lissi
```

### Request Body:
```json
{
  "request_credentials": [
    {
      "format": "jwt_vc_json",
      "type": "VerifiableCredential"
    }
  ]
}
```

### Response ejemplo:
```json
{
  "presentationDefinition": {
    "id": "session-abc123",
    "input_descriptors": [...]
  },
  "sessionId": "abc123-def456-ghi789",
  "requestUri": "openid4vp://authorize?request_uri=https://verifier.devportal.nebulae.com.co/openid4vc/request/abc123-def456-ghi789"
}
```

### Implementación en Go:
```go
func (s *VerifierService) InitVerificationSession() (*VerificationSessionResponse, error) {
    url := fmt.Sprintf("%s/openid4vc/verify", s.config.WaltIDVerifierURL)
    
    request := map[string]interface{}{
        "request_credentials": []map[string]interface{}{
            {
                "format": "jwt_vc_json",
                "type":   "VerifiableCredential",
            },
        },
    }
    
    payload, err := json.Marshal(request)
    if err != nil {
        return nil, err
    }
    
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("openId4VPProfile", "DEFAULT") // IMPORTANTE para Lissi
    
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var response VerificationSessionResponse
    if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
        return nil, err
    }
    
    return &response, nil
}
```

### 2.2 Verificar Estado de la Sesión

```
GET https://verifier.devportal.nebulae.com.co/openid4vc/session/{sessionId}
```

### Response ejemplo (pendiente):
```json
{
  "id": "abc123-def456-ghi789",
  "presentationDefinition": {
    "id": "session-abc123",
    "input_descriptors": [...]
  },
  "presentations": null,
  "verificationResult": null
}
```

### Response ejemplo (completada):
```json
{
  "id": "abc123-def456-ghi789",
  "presentationDefinition": {
    "id": "session-abc123",
    "input_descriptors": [...]
  },
  "presentations": [
    {
      "vp_token": "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9...",
      "presentation_submission": {
        "id": "submission-123",
        "definition_id": "session-abc123",
        "descriptor_map": [...]
      }
    }
  ],
  "verificationResult": true
}
```

### Implementación en Go:
```go
func (s *VerifierService) CheckVerificationStatus(sessionID string) (*VerificationStatus, error) {
    url := fmt.Sprintf("%s/openid4vc/session/%s", s.config.WaltIDVerifierURL, sessionID)
    
    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var status VerificationStatus
    if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
        return nil, err
    }
    
    return &status, nil
}
```

## 3. Generación de Códigos QR

### Para Emisión:
```go
import "github.com/skip2/go-qrcode"

func generateQRCode(uri string) ([]byte, error) {
    return qrcode.Encode(uri, qrcode.Medium, 256)
}
```

### Para Verificación:
```go
func (h *VerifyHandler) GenerateVerificationQR(w http.ResponseWriter, r *http.Request) {
    session, err := h.verifierService.InitVerificationSession()
    if err != nil {
        http.Error(w, "Error creating verification session", http.StatusInternalServerError)
        return
    }
    
    // Generar QR code con la URI de autorización
    qrCode, err := qrcode.Encode(session.RequestURI, qrcode.Medium, 256)
    if err != nil {
        http.Error(w, "Error generating QR code", http.StatusInternalServerError)
        return
    }
    
    // Codificar en base64 para mostrar en HTML
    qrBase64 := base64.StdEncoding.EncodeToString(qrCode)
    
    response := map[string]interface{}{
        "sessionId": session.SessionID,
        "qrCode":    "data:image/png;base64," + qrBase64,
        "requestUri": session.RequestURI,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

## 4. Polling de Estado de Verificación

### Implementación JavaScript (Frontend):
```javascript
async function checkVerificationStatus(sessionId) {
    try {
        const response = await fetch(\`/api/verification-status/\${sessionId}\`);
        const data = await response.json();
        
        if (data.status === 'completed') {
            // Verificación completada
            window.location.href = '/verification-success';
        } else if (data.status === 'pending') {
            // Seguir polling
            setTimeout(() => checkVerificationStatus(sessionId), 2000);
        } else {
            // Error o expirado
            console.error('Verification failed:', data.error);
        }
    } catch (error) {
        console.error('Error checking verification status:', error);
    }
}
```

### Implementación Go (Backend):
```go
func (h *VerifyHandler) CheckVerificationStatus(w http.ResponseWriter, r *http.Request) {
    sessionID := mux.Vars(r)["sessionId"]
    
    status, err := h.verifierService.CheckVerificationStatus(sessionID)
    if err != nil {
        http.Error(w, "Error checking verification status", http.StatusInternalServerError)
        return
    }
    
    responseStatus := "pending"
    if status.Presentations != nil && len(status.Presentations) > 0 {
        if status.VerificationResult != nil && *status.VerificationResult {
            responseStatus = "completed"
        } else {
            responseStatus = "failed"
        }
    }
    
    response := map[string]interface{}{
        "status":    responseStatus,
        "sessionId": sessionID,
    }
    
    if responseStatus == "completed" {
        response["result"] = status.VerificationResult
        response["presentations"] = status.Presentations
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

## 5. Compatibilidad con Wallets

### Lissi Wallet
- ✅ Requiere `openId4VPProfile: DEFAULT`
- ✅ Funciona con `request_credentials` format
- ✅ Compatible con `openid4vp://authorize` URLs

### Inji Wallet
- ✅ Funciona con perfil `DEFAULT`
- ✅ Compatible con formato estándar

## 6. Debugging y Logs

### Ejemplo de logs útiles:
```go
log.Printf("📦 Raw verifier response: %s", string(body))
log.Printf("⏳ Found session with presentationDefinition but no presentations - still pending")
log.Printf("✅ Verification completed with result: %v", status.VerificationResult)
log.Printf("❌ Verification failed: %v", err)
```

## 7. Configuración de variables de entorno

### `.env` ejemplo:
```env
WALT_ID_ISSUER_URL=https://issuer.devportal.nebulae.com.co
WALT_ID_VERIFIER_URL=https://verifier.devportal.nebulae.com.co
WALT_ID_ISSUER_DID=did:key:z6MkoLzFfMmVhfQpZZJHFh4TRdNEUbJ8z4PzE4uxFVYYhx8C
PORT=8080
```

Esta guía documenta los endpoints y patrones exactos utilizados en nuestro proyecto funcional.