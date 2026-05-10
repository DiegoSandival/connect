package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"hosts/internal/config"
	"hosts/internal/network"
	"hosts/internal/portal"
	"hosts/internal/sessions"
	"hosts/internal/uploads"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load("config.json")
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	uploadService := uploads.NewService(cfg.UploadDir, cfg.UploadLimitBytes())
	if err := uploadService.EnsureDir(); err != nil {
		logger.Error("ensure uploads dir", "error", err)
		os.Exit(1)
	}

	controller := network.NewWindowsFirewallController(
		cfg.Network.FirewallRulePrefix,
		cfg.Network.HotspotSubnet,
		cfg.Network.FirewallEnabled,
	)
	if err := controller.Prepare(context.Background(), network.PrepareOptions{
		PortalPort:    cfg.Network.PortalPort,
		HotspotSubnet: cfg.Network.HotspotSubnet,
	}); err != nil {
		logger.Warn("firewall prepare failed", "error", err)
	}

	sessionStatePath := filepath.Join(cfg.UploadDir, ".sessions.json")
	sessionManager, err := sessions.NewManager(cfg.AccessDuration(), cfg.Network.ActivationMode, controller, sessionStatePath)
	if err != nil {
		logger.Error("create session manager", "error", err)
		os.Exit(1)
	}
	portalServer, err := portal.NewServer(cfg, logger, sessionManager, uploadService, controller)
	if err != nil {
		logger.Error("create portal server", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              cfg.Network.ListenAddress,
		Handler:           portalServer.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("portal ready", "listen", cfg.Network.ListenAddress, "portal_url", cfg.Network.PortalBaseURL, "uploads", cfg.UploadDir, "mode", cfg.Network.ActivationMode, "controller", controller.Name())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
