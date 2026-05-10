package network

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type WindowsFirewallController struct {
	prefix        string
	hotspotSubnet string
	enabled       bool
}

func NewWindowsFirewallController(prefix string, hotspotSubnet string, enabled bool) AccessController {
	if runtime.GOOS != "windows" || !enabled {
		return &NoopController{}
	}

	return &WindowsFirewallController{
		prefix:        sanitizeRuleName(prefix),
		hotspotSubnet: hotspotSubnet,
		enabled:       enabled,
	}
}

func (w *WindowsFirewallController) Name() string {
	return "windows-firewall"
}

func (w *WindowsFirewallController) Prepare(ctx context.Context, options PrepareOptions) error {
	if !w.enabled {
		return nil
	}

	ruleName := fmt.Sprintf("%s-Portal-%d", w.prefix, options.PortalPort)
	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$existing = Get-NetFirewallRule -DisplayName '%s' -ErrorAction SilentlyContinue
if (-not $existing) {
  New-NetFirewallRule -DisplayName '%s' -Direction Inbound -Action Allow -Protocol TCP -LocalPort %d -RemoteAddress %s | Out-Null
}
`, psQuote(ruleName), psQuote(ruleName), options.PortalPort, psQuote(options.HotspotSubnet))

	return runPowerShell(ctx, script)
}

func (w *WindowsFirewallController) EnsureBlocked(ctx context.Context, clientIP string) error {
	return w.BlockClient(ctx, clientIP)
}

func (w *WindowsFirewallController) AllowClient(ctx context.Context, clientIP string) error {
	if !w.enabled {
		return nil
	}

	blockName := w.blockRuleName(clientIP)
	allowName := w.allowRuleName(clientIP)
	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
Get-NetFirewallRule -DisplayName '%s' -ErrorAction SilentlyContinue | Remove-NetFirewallRule | Out-Null
$allow = Get-NetFirewallRule -DisplayName '%s' -ErrorAction SilentlyContinue
if (-not $allow) {
  New-NetFirewallRule -DisplayName '%s' -Direction Outbound -Action Allow -Protocol Any -LocalAddress %s -RemoteAddress Any | Out-Null
}
`, psQuote(blockName), psQuote(allowName), psQuote(allowName), psQuote(clientIP))

	return runPowerShell(ctx, script)
}

func (w *WindowsFirewallController) BlockClient(ctx context.Context, clientIP string) error {
	if !w.enabled {
		return nil
	}

	blockName := w.blockRuleName(clientIP)
	allowName := w.allowRuleName(clientIP)
	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
Get-NetFirewallRule -DisplayName '%s' -ErrorAction SilentlyContinue | Remove-NetFirewallRule | Out-Null
$block = Get-NetFirewallRule -DisplayName '%s' -ErrorAction SilentlyContinue
if (-not $block) {
  New-NetFirewallRule -DisplayName '%s' -Direction Outbound -Action Block -Protocol Any -LocalAddress %s -RemoteAddress Any | Out-Null
}
`, psQuote(allowName), psQuote(blockName), psQuote(blockName), psQuote(clientIP))

	return runPowerShell(ctx, script)
}

func (w *WindowsFirewallController) blockRuleName(clientIP string) string {
	return fmt.Sprintf("%s-Block-%s", w.prefix, sanitizeRuleName(clientIP))
}

func (w *WindowsFirewallController) allowRuleName(clientIP string) string {
	return fmt.Sprintf("%s-Allow-%s", w.prefix, sanitizeRuleName(clientIP))
}

func runPowerShell(ctx context.Context, script string) error {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("powershell: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func sanitizeRuleName(value string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-")
	return replacer.Replace(value)
}

func psQuote(value string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(value, "'", "''"))
}
