package config

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"
)

type Config struct {
	BusinessName string        `json:"business_name"`
	UploadDir    string        `json:"upload_dir"`
	MaxUploadMB  int64         `json:"max_upload_mb"`
	Network      NetworkConfig `json:"network"`
}

type NetworkConfig struct {
	ListenAddress         string `json:"listen_address"`
	PortalBaseURL         string `json:"portal_base_url"`
	PortalPort            int    `json:"portal_port"`
	HotspotSubnet         string `json:"hotspot_subnet"`
	ActivationMode        string `json:"activation_mode"`
	AccessDurationMinutes int    `json:"access_duration_minutes"`
	FirewallEnabled       bool   `json:"firewall_enabled"`
	FirewallRulePrefix    string `json:"firewall_rule_prefix"`
}

func Default() Config {
	return Config{
		BusinessName: "Portal de Impresion",
		UploadDir:    "uploads",
		MaxUploadMB:  64,
		Network: NetworkConfig{
			ListenAddress:         ":8080",
			PortalBaseURL:         "http://192.168.137.1:8080",
			PortalPort:            8080,
			HotspotSubnet:         "192.168.137.0/24",
			ActivationMode:        "manual",
			AccessDurationMinutes: 15,
			FirewallEnabled:       true,
			FirewallRulePrefix:    "PrintPortal",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	cfg.normalize()
	return cfg, nil
}

func (c *Config) normalize() {
	if strings.TrimSpace(c.BusinessName) == "" {
		c.BusinessName = "Portal de Impresion"
	}
	if strings.TrimSpace(c.UploadDir) == "" {
		c.UploadDir = "uploads"
	}
	if c.MaxUploadMB <= 0 {
		c.MaxUploadMB = 64
	}
	if c.Network.PortalPort <= 0 {
		c.Network.PortalPort = 8080
	}
	if strings.TrimSpace(c.Network.ListenAddress) == "" {
		c.Network.ListenAddress = ":8080"
	}
	if strings.TrimSpace(c.Network.PortalBaseURL) == "" {
		c.Network.PortalBaseURL = "http://192.168.137.1:8080"
	}
	if strings.TrimSpace(c.Network.HotspotSubnet) == "" {
		c.Network.HotspotSubnet = "192.168.137.0/24"
	}
	mode := strings.ToLower(strings.TrimSpace(c.Network.ActivationMode))
	if mode != "auto" {
		mode = "manual"
	}
	c.Network.ActivationMode = mode
	if c.Network.AccessDurationMinutes <= 0 {
		c.Network.AccessDurationMinutes = 15
	}
	if strings.TrimSpace(c.Network.FirewallRulePrefix) == "" {
		c.Network.FirewallRulePrefix = "PrintPortal"
	}
}

func (c Config) AccessDuration() time.Duration {
	return time.Duration(c.Network.AccessDurationMinutes) * time.Minute
}

func (c Config) UploadLimitBytes() int64 {
	return c.MaxUploadMB * 1024 * 1024
}
