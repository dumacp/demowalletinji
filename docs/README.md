# Documentación del Demo Portal Walt.id

Este directorio contiene documentación de las APIs de Walt.id utilizadas en el proyecto de demo portal para OpenID4VC.

## Estructura

- `waltid-api/` - Documentación completa de las APIs de Walt.id
  - `issuer-api.json` - Especificación OpenAPI 3.1 del servicio issuer de Walt.id (175KB)
  - `verifier-api.json` - Especificación OpenAPI 3.1 del servicio verifier de Walt.id (242KB)

## APIs utilizadas en el proyecto

### Walt.id Issuer API

**Servidor:** `https://issuer.devportal.nebulae.com.co`

#### Endpoint principal de emisión de credenciales:
```
POST /{standardVersion}/credential
```

- **Descripción**: Endpoint para emitir credenciales W3C Verifiable y devolver URL de emisión
- **Path Parameter**: `standardVersion` - Versión estándar (valores soportados: `draft13`, `draft11`)
- **Ejemplo de uso en nuestro código**: 
  - URL completa: `https://issuer.devportal.nebulae.com.co/openid4vc/sdjwtvc/issue`
  - Usado en: `internal/services/issuer.go`

### Walt.id Verifier API

**Servidor:** `https://verifier.devportal.nebulae.com.co`

#### Endpoints principales de verificación:

#### 1. Inicializar sesión de verificación
```
POST /openid4vc/verify
```

- **Descripción**: Inicializa una sesión de presentación OIDC con la definición de presentación dada
- **Headers importantes**:
  - `openId4VPProfile`: Perfil de la solicitud VP (valores: `DEFAULT`, `ISO_18013_7_MDOC`, `EBSIV3`, `HAIP`)
    - **⚠️ Importante**: Para compatibilidad con Lissi wallet, usar `DEFAULT`
  - `authorizeBaseUrl`: Base URL del endpoint authorize del wallet (por defecto: `openid4vp://authorize`)
  - `responseMode`: Modo de respuesta (por defecto: `direct_post`)

- **Request Body**: 
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

- **Example Response**: Devuelve una URL que puede ser renderizada como QR code
- **Usado en**: `internal/services/verifier.go` - método `InitVerificationSession`

#### 2. Obtener estado de la sesión
```
GET /openid4vc/session/{id}
```

- **Descripción**: Obtiene información sobre una sesión de presentación OIDC previamente inicializada
- **Path Parameter**: `id` - ID de la sesión
- **Response**: Información de la sesión incluyendo estado actual y resultados
- **Usado en**: `internal/services/verifier.go` - método `CheckVerificationStatus`

#### 3. Obtener credenciales presentadas
```
GET /openid4vc/session/{id}/presented-credentials
```

- **Descripción**: Obtiene credenciales decodificadas asociadas con una sesión de verificación exitosa
- **Disponible solo**: Para sesiones con `verificationResult == true`

## Configuración del proyecto

### Variables de entorno requeridas:

```bash
WALT_ID_ISSUER_URL=https://issuer.devportal.nebulae.com.co
WALT_ID_VERIFIER_URL=https://verifier.devportal.nebulae.com.co
WALT_ID_ISSUER_DID=did:key:z6MkoLzFfMmVhfQpZZJHFh4TRdNEUbJ8z4PzE4uxFVYYhx8C
```

### Compatibilidad con wallets

#### Lissi Wallet
- **Perfil requerido**: `DEFAULT` en header `openId4VPProfile`
- **Formato**: Usar `request_credentials` en lugar de `presentation_definition` para compatibilidad

#### Inji Wallet  
- **Perfil**: Funciona con perfil `DEFAULT`
- **Formato**: Compatible con el formato estándar

## Resolución de problemas

### Error común con Lissi wallet
**Problema**: `ArgumentNullException` al escanear QR de verificación

**Solución**: 
1. Asegurar que el perfil OpenID4VP esté configurado como `DEFAULT`
2. Usar el formato de request con `request_credentials` en lugar de `presentation_definition`
3. Verificar que el endpoint devuelve el formato correcto de URL

### Debugging
Para debuggear las respuestas de la API, revisar los logs del servicio verifier que incluyen:
- `📦 Raw verifier response` - Respuesta completa de la API
- `⏳ Found session with presentationDefinition but no presentations - still pending` - Estado pendiente
- `✅ Verification completed` - Verificación exitosa

## Estructura del código

```
internal/
├── services/
│   ├── issuer.go      # Integración con Walt.id Issuer API
│   ├── verifier.go    # Integración con Walt.id Verifier API
│   └── auth.go        # Gestión de sesiones de verificación
├── handlers/
│   ├── issue.go       # Endpoints de emisión
│   ├── verify.go      # Endpoints de verificación
│   └── session.go     # Endpoints de gestión de sesiones
└── models/
    └── session.go     # Modelos de datos para sesiones
```

## Enlaces útiles

- [Walt.id Documentation](https://docs.walt.id)
- [OpenID4VC Specification](https://openid.net/specs/openid-4-verifiable-credential-issuance-1_0.html)
- [OpenID4VP Specification](https://openid.net/specs/openid-4-verifiable-presentations-1_0.html)