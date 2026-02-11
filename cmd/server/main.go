package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dumacp/demowalletinji/internal/config"
	"github.com/dumacp/demowalletinji/internal/handlers"
	"github.com/dumacp/demowalletinji/internal/services"
)

func main() {
	log.Printf("🚀 Starting OpenID4VC Demo Portal...")

	// Load configuration
	cfg := config.LoadConfig()
	log.Printf("📊 Configuration loaded - Mode: %s", cfg.Server.Mode)

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.Server.Timeout) * time.Second,
	}

	// Initialize services
	issuerService := services.NewIssuerService(httpClient, &cfg.Issuer)
	verifierService := services.NewVerifierService(httpClient, &cfg.Verifier)
	authService := services.NewAuthService(time.Duration(cfg.SessionTTL) * time.Minute)

	// Initialize handlers
	handler := handlers.NewHandler(issuerService, verifierService, authService, cfg.Server.Mode)

	// Setup routes
	mux := http.NewServeMux()

	// Portal routes
	mux.HandleFunc("/", handler.PortalHandler)
	mux.HandleFunc("/health", handler.HealthHandler)

	// Demo API routes
	mux.HandleFunc("/demo/issue", handler.IssueHandler)
	mux.HandleFunc("/demo/verify", handler.VerifyHandler)
	mux.HandleFunc("/demo/session", handler.CreateSessionHandler)
	mux.HandleFunc("/demo/session/", handler.SessionHandler)
	mux.HandleFunc("/demo/me", handler.MeHandler)
	mux.HandleFunc("/demo/verifier-details", handler.VerifierDetailsHandler)

	// Static file server for web assets
	fs := http.FileServer(http.Dir("configs/web/"))
	mux.Handle("/web/", http.StripPrefix("/web/", fs))

	// Create server
	server := &http.Server{
		Addr:           cfg.Server.Address,
		Handler:        mux,
		ReadTimeout:    time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:    time.Duration(cfg.Server.IdleTimeout) * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	// Start server
	log.Printf("🌐 Server starting on %s", cfg.Server.Address)
	log.Printf("📱 Portal available at: http://%s", cfg.Server.Address)
	log.Printf("🔗 Issue endpoint: http://%s/demo/issue", cfg.Server.Address)
	log.Printf("🔍 Verify endpoint: http://%s/demo/verify", cfg.Server.Address)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("❌ Server failed to start: %v", err)
		os.Exit(1)
	}
}
