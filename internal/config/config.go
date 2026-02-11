package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration
type Config struct {
	Server     ServerConfig     `json:"server"`
	Issuer     IssuerConfig     `json:"issuer"`
	Verifier   VerifierConfig   `json:"verifier"`
	Timeouts   TimeoutConfig    `json:"timeouts"`
	SessionTTL int              `json:"session_ttl"`
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Address      string `json:"address"`
	Mode         string `json:"mode"`
	Timeout      int    `json:"timeout"`
	ReadTimeout  int    `json:"read_timeout"`
	WriteTimeout int    `json:"write_timeout"`
	IdleTimeout  int    `json:"idle_timeout"`
}

// IssuerConfig holds issuer-specific configuration
type IssuerConfig struct {
	BaseURL   string `json:"base_url"`
	IssuePath string `json:"issue_path"`
}

// VerifierConfig holds verifier-specific configuration
type VerifierConfig struct {
	BaseURL           string `json:"base_url"`
	VerifyPath        string `json:"verify_path"`
	SessionPath       string `json:"session_path"`
}

// TimeoutConfig holds timeout configuration
type TimeoutConfig struct {
	IssueRequest   time.Duration `json:"issue_request"`
	VerifyRequest  time.Duration `json:"verify_request"`
	SessionRequest time.Duration `json:"session_request"`
	HTTPClient     time.Duration `json:"http_client"`
}

// LoadConfig loads configuration from environment variables with defaults
func LoadConfig() *Config {
	config := NewDefaultConfig()

	// Override with environment variables if present
	if val := os.Getenv("SERVER_ADDRESS"); val != "" {
		config.Server.Address = val
	}
	if val := os.Getenv("GIN_MODE"); val != "" {
		config.Server.Mode = val
	}
	if val := os.Getenv("ISSUER_BASE_URL"); val != "" {
		config.Issuer.BaseURL = val
	}
	if val := os.Getenv("VERIFIER_BASE_URL"); val != "" {
		config.Verifier.BaseURL = val
	}
	if val := os.Getenv("SESSION_TTL"); val != "" {
		if ttl, err := strconv.Atoi(val); err == nil {
			config.SessionTTL = ttl
		}
	}

	return config
}

// NewDefaultConfig returns a default configuration
func NewDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Address:      ":8080",
			Mode:         "debug",
			Timeout:      30,
			ReadTimeout:  15,
			WriteTimeout: 15,
			IdleTimeout:  60,
		},
		Issuer: IssuerConfig{
			BaseURL:   "https://issuer.devportal.nebulae.com.co",
			IssuePath: "/openid4vc/sdjwt/issue",
		},
		Verifier: VerifierConfig{
			BaseURL:     "https://verifier.devportal.nebulae.com.co",
			VerifyPath:  "/openid4vc/verify",
			SessionPath: "/openid4vc/session",
		},
		Timeouts: TimeoutConfig{
			IssueRequest:   20 * time.Second,
			VerifyRequest:  20 * time.Second,
			SessionRequest: 15 * time.Second,
			HTTPClient:     30 * time.Second,
		},
		SessionTTL: 60, // minutes
	}
}