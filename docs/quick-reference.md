# Documentación del Demo Portal - Índice Rápido

## 🚀 Inicio Rápido

### Como desarrollador nuevo al proyecto:
1. Lee [README.md](README.md) para entender la estructura general
2. Revisa [waltid-integration-guide.md](waltid-integration-guide.md) para implementación específica
3. Consulta [waltid-api/](waltid-api/) para especificaciones detalladas de API

### Solución de problemas específicos:

#### ❓ "Mi wallet Lissi no funciona"
→ Ver sección "Compatibilidad con wallets" en [waltid-integration-guide.md](waltid-integration-guide.md#5-compatibilidad-con-wallets)

#### ❓ "Error ArgumentNullException en el QR de verificación"
→ Ver "Resolución de problemas" en [README.md](README.md#resolución-de-problemas)

#### ❓ "¿Cómo generar QR codes?"
→ Ver sección "Generación de Códigos QR" en [waltid-integration-guide.md](waltid-integration-guide.md#3-generación-de-códigos-qr)

#### ❓ "¿Cómo verificar el estado de una verificación?"
→ Ver "Polling de Estado" en [waltid-integration-guide.md](waltid-integration-guide.md#4-polling-de-estado-de-verificación)

## 📚 Archivos de Documentación

| Archivo | Descripción | Cuándo usar |
|---------|-------------|-------------|
| [README.md](README.md) | Visión general del proyecto y APIs utilizadas | Primera lectura, referencia general |
| [waltid-integration-guide.md](waltid-integration-guide.md) | Implementación práctica y código de ejemplo | Desarrollo activo, integración |
| [waltid-api/issuer-api.json](waltid-api/issuer-api.json) | Especificación completa OpenAPI del issuer (175KB) | Referencia detallada de endpoints |
| [waltid-api/verifier-api.json](waltid-api/verifier-api.json) | Especificación completa OpenAPI del verifier (242KB) | Referencia detallada de endpoints |

## 🔧 Endpoints Principales (Referencia Rápida)

### Emisión de Credenciales
```
POST https://issuer.devportal.nebulae.com.co/openid4vc/sdjwtvc/issue
```

### Verificación - Iniciar Sesión
```
POST https://verifier.devportal.nebulae.com.co/openid4vc/verify
Header: openId4VPProfile: DEFAULT  # ⚠️ IMPORTANTE para Lissi
```

### Verificación - Verificar Estado
```
GET https://verifier.devportal.nebulae.com.co/openid4vc/session/{sessionId}
```

## 🏗️ Estructura del Código (Referencia)

```
cmd/server/main.go          # Entry point
internal/
├── handlers/
│   ├── issue.go           # POST /issue - Emisión de credenciales
│   ├── verify.go          # POST /verify - Generar QR de verificación  
│   └── session.go         # GET /status/{id} - Estado de verificación
├── services/
│   ├── issuer.go          # Interfaz con Walt.id Issuer API
│   ├── verifier.go        # Interfaz con Walt.id Verifier API
│   └── auth.go            # Gestión de sesiones
└── models/
    └── session.go         # Estructuras de datos
```

## 🎯 Casos de Uso Comunes

### 1. Emitir una credencial nueva
1. Llamar a `/issue` endpoint → devuelve QR code
2. Usuario escanea con su wallet
3. Credencial se guarda en el wallet

### 2. Verificar una credencial
1. Llamar a `/verify` endpoint → devuelve QR code + sessionId  
2. Usuario escanea con su wallet
3. Hacer polling a `/status/{sessionId}` hasta que esté completa
4. Procesar resultado de verificación

## 🐛 Debugging

### Logs importantes a buscar:
```
📦 Raw verifier response     # Respuesta completa de Walt.id
⏳ Found session with...     # Estado pendiente
✅ Verification completed    # Éxito
❌ Verification failed      # Error
```

### Variables de entorno requeridas:
```bash
WALT_ID_ISSUER_URL=https://issuer.devportal.nebulae.com.co
WALT_ID_VERIFIER_URL=https://verifier.devportal.nebulae.com.co  
WALT_ID_ISSUER_DID=did:key:z6MkoLzFfMmVhfQpZZJHFh4TRdNEUbJ8z4PzE4uxFVYYhx8C
```

---

**💡 Tip**: Este archivo está diseñado para respuestas rápidas. Para información detallada, consulta los archivos específicos enlazados.