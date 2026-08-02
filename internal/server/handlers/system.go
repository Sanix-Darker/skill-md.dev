package handlers

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sanixdarker/skillf/internal/app"
	sshserver "github.com/sanixdarker/skillf/internal/ssh"
)

// SystemHandler handles runtime/system endpoints.
type SystemHandler struct {
	app *app.App
}

// NewSystemHandler creates a new SystemHandler.
func NewSystemHandler(application *app.App) *SystemHandler {
	return &SystemHandler{app: application}
}

// Health returns a lightweight health response for probes.
func (h *SystemHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	payload := map[string]interface{}{
		"status":    "ok",
		"service":   "skillf",
		"version":   h.app.Config.Version,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		h.app.Logger.Error("failed to render health payload", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// System returns runtime metadata for web UX and automation.
func (h *SystemHandler) System(w http.ResponseWriter, r *http.Request) {
	sshEnabled := h.app.Config.SSHEnabled
	sshHost := normalizeExternalHost(h.app.Config.SSHHost)
	sshPort := h.app.Config.SSHPort
	sshKeyPath := strings.TrimSpace(h.app.Config.SSHKeyPath)
	sshUser := strings.TrimSpace(h.app.Config.SSHUser)

	publicHost := normalizeExternalHost(r.Header.Get("X-Forwarded-Host"))
	if publicHost == "" {
		publicHost = normalizeExternalHost(r.Host)
	}
	if publicHost == "" {
		publicHost = normalizeExternalHost(h.app.Config.PublicHost)
	}
	if publicHost == "" {
		publicHost = "127.0.0.1"
	}
	scheme := resolveScheme(r, publicHost)
	webBaseURL := buildPublicURL(scheme, publicHost, h.app.Config.Port)
	localWebURL := fmt.Sprintf("http://127.0.0.1:%d", h.app.Config.Port)

	publicHostOnly := stripHostPort(publicHost)
	sshTargetHost := publicHostOnly
	if sshHost != "" {
		sshTargetHost = stripHostPort(sshHost)
	}
	if sshTargetHost == "" {
		sshTargetHost = publicHostOnly
	}
	if sshPort <= 0 {
		sshPort = 0
	}
	connectionTarget := sshTargetHost
	if sshUser != "" {
		connectionTarget = fmt.Sprintf("%s@%s", sshUser, sshTargetHost)
	}
	connectCmd := ""
	sshStatus := "warning"
	sshError := strings.TrimSpace(h.app.Config.SSHStartupError)
	if !sshEnabled {
		sshStatus = "disabled"
	}
	if sshEnabled && sshPort > 0 {
		connectCmd = fmt.Sprintf("ssh -p %d %s", sshPort, connectionTarget)
	}
	if connectCmd == "" {
		connectCmd = "ssh -p <port> <host>"
	}
	connectCommands := []map[string]string{
		{
			"label":   "Connect",
			"command": connectCmd,
		},
	}
	sshCheckCommands := buildSSHChecks(webBaseURL, localWebURL, connectCmd, sshEnabled)
	sshDiagnostics := checkSSHKeyDiagnostics(sshKeyPath)
	if status, ok := sshDiagnostics["status"].(string); ok && status != "ok" && strings.TrimSpace(sshError) == "" {
		sshError = fmt.Sprintf("%v", sshDiagnostics["message"])
	}
	if !sshEnabled && strings.TrimSpace(sshError) == "" {
		sshError = "ssh disabled"
	}
	if !sshEnabled && sshDiagnostics["status"] == "ok" {
		sshDiagnostics["status"] = "disabled"
		sshDiagnostics["message"] = "SSH service is currently disabled"
	}

	if sshEnabled && sshError == "" && sshDiagnostics["status"] != "ok" {
		if message, ok := sshDiagnostics["message"].(string); ok {
			sshError = message
		}
	}
	sshReady := sshEnabled && sshPort > 0 && sshError == "" && sshDiagnostics["status"] == "ok"
	if sshReady {
		sshStatus = "ready"
	}
	remediation := buildSSHRemediation(sshEnabled, sshError, sshDiagnostics)
	recommendedAction := buildSSHAction(sshReady, sshEnabled, sshError, sshDiagnostics)

	w.Header().Set("Content-Type", "application/json")
	payload := map[string]interface{}{
		"service":          "skillf",
		"version":          h.app.Config.Version,
		"status":           "ok",
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
		"public_host":      publicHost,
		"public_host_only": publicHostOnly,
		"public_url":       webBaseURL,
		"web": map[string]interface{}{
			"port":         h.app.Config.Port,
			"host":         publicHost,
			"hostOnly":     publicHostOnly,
			"listen_host":  strings.TrimSpace(h.app.Config.ListenHost),
			"scheme":       scheme,
			"base_url":     webBaseURL,
			"public_url":   webBaseURL,
			"local_url":    localWebURL,
			"health":       webBaseURL + "/health",
			"local_health": localWebURL + "/health",
			"status":       "ok",
		},
		"ssh": map[string]interface{}{
			"enabled":            sshEnabled,
			"status":             sshStatus,
			"host":               sshTargetHost,
			"user":               sshUser,
			"port":               sshPort,
			"connect_cmd":        connectCmd,
			"connect_commands":   connectCommands,
			"host_check":         sshCheckCommands,
			"operator_checks":    sshCheckCommands,
			"ready":              sshReady,
			"diagnostic_error":   sshError,
			"recommended_action": recommendedAction,
			"remediation":        remediation,
			"key":                sshDiagnostics,
		},
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		h.app.Logger.Error("failed to render system payload", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func resolveScheme(r *http.Request, host string) string {
	forwardedScheme := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")))
	if forwardedScheme == "https" || forwardedScheme == "http" {
		return forwardedScheme
	}
	if r.TLS != nil {
		return "https"
	}
	if !isLocalHost(host) {
		return "https"
	}
	return "http"
}

func stripHostPort(host string) string {
	if host == "" {
		return ""
	}

	host = strings.TrimSpace(strings.SplitN(host, ",", 2)[0])
	if strings.HasPrefix(host, "[") {
		if end := strings.Index(host, "]"); end > 1 {
			return strings.TrimSpace(host[1:end])
		}
		return ""
	}

	if parsedIP := net.ParseIP(host); parsedIP != nil {
		return host
	}

	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	if host == "" {
		return host
	}
	return host
}

func buildPublicURL(scheme, host string, port int) string {
	cleanHost := normalizeExternalHost(host)
	if cleanHost == "" {
		cleanHost = "127.0.0.1"
	}

	if _, _, err := net.SplitHostPort(cleanHost); err == nil {
		return fmt.Sprintf("%s://%s", strings.TrimSpace(scheme), cleanHost)
	}
	if isLocalHost(cleanHost) {
		return fmt.Sprintf("%s://%s:%d", strings.TrimSpace(scheme), cleanHost, port)
	}
	return fmt.Sprintf("%s://%s", strings.TrimSpace(scheme), cleanHost)
}

func checkSSHKeyDiagnostics(path string) map[string]interface{} {
	resolvedPath, err := sshserver.ResolveKeyPath(path)
	if err != nil {
		return map[string]interface{}{
			"path":       "",
			"filename":   "",
			"configured": false,
			"status":     "invalid_path",
			"readable":   false,
			"regular":    false,
			"mode":       "n/a",
			"size":       0,
			"message":    err.Error(),
		}
	}

	diagnostics := map[string]interface{}{
		"path":       maskKeyPath(resolvedPath),
		"filename":   filepath.Base(resolvedPath),
		"configured": strings.TrimSpace(path) != "",
		"status":     "not_configured",
		"readable":   false,
		"regular":    false,
		"mode":       "n/a",
		"size":       0,
		"message":    "SSH host key is not configured",
	}

	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return diagnostics
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		diagnostics["status"] = "missing"
		diagnostics["message"] = "SSH host key file is missing or has not been created yet"
		return diagnostics
	}

	diagnostics["readable"] = true
	diagnostics["size"] = info.Size()
	diagnostics["mode"] = fmt.Sprintf("%04o", info.Mode().Perm())
	diagnostics["regular"] = info.Mode().IsRegular()

	if !info.Mode().IsRegular() {
		diagnostics["status"] = "invalid"
		diagnostics["message"] = "SSH host key must be a regular file"
		return diagnostics
	}

	if info.Mode().Perm() != 0600 {
		diagnostics["status"] = "insecure_permissions"
		diagnostics["message"] = "SSH host key should use mode 0600"
		return diagnostics
	}

	diagnostics["status"] = "ok"
	diagnostics["message"] = "SSH host key file looks valid"
	return diagnostics
}

func buildSSHChecks(publicURL, localURL, connectCmd string, sshEnabled bool) []string {
	checks := []string{
		"curl -fsS " + publicURL + "/health",
	}
	if localURL != "" && localURL != publicURL {
		checks = append(checks, "curl -fsS "+localURL+"/health")
	}
	if sshEnabled {
		checks = append(checks, connectCmd)
	}
	checks = append(checks, "skillf merge skill-a.md skill-b.md -o merged-skill.md")
	return checks
}

func buildSSHAction(ready, enabled bool, startupError string, diagnostics map[string]interface{}) string {
	if ready {
		return "SSH TUI is ready. Connect from your terminal and use merge, browse, and convert from one session."
	}
	if !enabled {
		if strings.Contains(strings.ToLower(startupError), "--no-ssh") {
			return "SSH is disabled in the current process. Restart skillf without --no-ssh to expose the terminal UI."
		}
		return "SSH is not available yet. Fix the runtime issue and restart the service before using the terminal UI."
	}
	if status, _ := diagnostics["status"].(string); status == "insecure_permissions" {
		return "SSH host key permissions are too open. Tighten them to mode 0600, then restart the service."
	}
	if status, _ := diagnostics["status"].(string); status == "missing" {
		return "The SSH host key is missing. Ensure the service can create the key file and restart it."
	}
	if strings.TrimSpace(startupError) != "" {
		return "SSH failed to start cleanly. Resolve the startup error and restart the service."
	}
	return "SSH is partially configured. Review the host key diagnostics and service startup logs."
}

func buildSSHRemediation(enabled bool, startupError string, diagnostics map[string]interface{}) []string {
	steps := []string{}
	if !enabled {
		steps = append(steps, "Restart skillf without --no-ssh if you want terminal access enabled.")
	}
	if status, _ := diagnostics["status"].(string); status == "insecure_permissions" {
		steps = append(steps, "Update the SSH host key file permissions to mode 0600.")
	}
	if status, _ := diagnostics["status"].(string); status == "invalid" {
		steps = append(steps, "Replace the configured SSH host key path with a regular file.")
	}
	if status, _ := diagnostics["status"].(string); status == "missing" {
		steps = append(steps, "Ensure the service user can create the SSH host key file in its configured directory.")
	}
	if strings.TrimSpace(startupError) != "" {
		steps = append(steps, "Inspect the skillf service logs after the next restart to confirm the SSH listener starts.")
	}
	if len(steps) == 0 {
		steps = append(steps, "No remediation required.")
	}
	return steps
}

func normalizeExternalHost(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return ""
	}

	trimmedValue = strings.SplitN(trimmedValue, ",", 2)[0]
	if strings.Contains(trimmedValue, "://") {
		parsedURL, err := url.Parse(trimmedValue)
		if err == nil && parsedURL.Host != "" {
			return parsedURL.Host
		}
	}
	if strings.ContainsAny(trimmedValue, "/?#") {
		parsedURL, err := url.Parse("https://" + trimmedValue)
		if err == nil && parsedURL.Host != "" {
			return parsedURL.Host
		}
	}
	return strings.TrimSuffix(trimmedValue, "/")
}

func isLocalHost(host string) bool {
	hostOnly := stripHostPort(host)
	hostOnly = strings.Trim(hostOnly, "[]")
	if hostOnly == "" || hostOnly == "localhost" {
		return true
	}
	return net.ParseIP(hostOnly) != nil
}

func maskKeyPath(path string) string {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return ""
	}

	home, err := os.UserHomeDir()
	if err == nil {
		relativePath, relativeErr := filepath.Rel(home, trimmedPath)
		if relativeErr == nil && relativePath != "." && !strings.HasPrefix(relativePath, "..") {
			return filepath.ToSlash(filepath.Join("~", relativePath))
		}
		if filepath.Clean(trimmedPath) == filepath.Clean(home) {
			return "~"
		}
	}

	fileName := filepath.Base(trimmedPath)
	if fileName == "." || fileName == string(filepath.Separator) {
		return "hidden"
	}
	return filepath.ToSlash(filepath.Join("...", fileName))
}
