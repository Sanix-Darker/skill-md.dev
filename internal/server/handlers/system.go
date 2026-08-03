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
	connectCommands := buildSSHConnectCommands(sshEnabled, sshPort, sshUser, sshTargetHost, publicHostOnly)
	connectCmd := ""
	sshStatus := "warning"
	sshError := strings.TrimSpace(h.app.Config.SSHStartupError)
	if !sshEnabled {
		sshStatus = "disabled"
	}
	if len(connectCommands) > 0 {
		connectCmd = strings.TrimSpace(connectCommands[0]["command"])
	}
	if connectCmd == "" {
		connectCmd = "ssh -p <port> <host>"
	}
	sshRoutingMode := buildSSHRoutingMode(sshEnabled, sshTargetHost, publicHostOnly)
	connectionNote := buildSSHConnectionNote(sshEnabled, publicHostOnly, sshTargetHost)
	sshCheckCommands := buildSSHChecks(webBaseURL, localWebURL, connectCommands, sshEnabled, sshTargetHost, sshPort)
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
	remediation := buildSSHRemediation(sshEnabled, sshError, sshDiagnostics, publicHostOnly, sshTargetHost)
	recommendedAction := buildSSHAction(sshReady, sshEnabled, sshError, sshDiagnostics, publicHostOnly, sshTargetHost)

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
			"public_host":        publicHostOnly,
			"user":               sshUser,
			"port":               sshPort,
			"connect_target":     sshTargetHost,
			"connect_cmd":        connectCmd,
			"connect_commands":   connectCommands,
			"routing_mode":       sshRoutingMode,
			"host_override":      !sameHost(sshTargetHost, publicHostOnly),
			"connection_note":    connectionNote,
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

func buildSSHConnectCommand(port int, host, user string) string {
	targetHost := strings.TrimSpace(stripHostPort(host))
	if targetHost == "" {
		return ""
	}

	target := targetHost
	if strings.TrimSpace(user) != "" {
		target = fmt.Sprintf("%s@%s", strings.TrimSpace(user), targetHost)
	}

	if port <= 0 {
		port = 2222
	}

	return fmt.Sprintf("ssh -p %d %s", port, target)
}

func buildSSHConnectCommands(sshEnabled bool, port int, user, sshHost, publicHost string) []map[string]string {
	if !sshEnabled || port <= 0 {
		return nil
	}

	commands := make([]map[string]string, 0, 2)
	primary := buildSSHConnectCommand(port, sshHost, user)
	if primary != "" {
		label := "Connect"
		if !sameHost(sshHost, publicHost) {
			label = "Primary SSH target"
		}
		commands = append(commands, map[string]string{
			"label":   label,
			"command": primary,
		})
	}

	if publicHost != "" && !sameHost(sshHost, publicHost) {
		publicCommand := buildSSHConnectCommand(port, publicHost, user)
		if publicCommand != "" {
			commands = append(commands, map[string]string{
				"label":   "Web hostname",
				"command": publicCommand,
			})
		}
	}

	return commands
}

func buildSSHRoutingMode(sshEnabled bool, sshHost, publicHost string) string {
	if !sshEnabled {
		return "disabled"
	}
	if isLocalHost(sshHost) {
		return "local_only"
	}
	if sameHost(sshHost, publicHost) {
		return "shared_public_host"
	}
	return "ssh_host_override"
}

func buildSSHConnectionNote(sshEnabled bool, publicHost, sshHost string) string {
	if !sshEnabled {
		return ""
	}
	if sshHost == "" {
		return "SSH is enabled, but no advertised host is configured yet."
	}
	if isLocalHost(sshHost) {
		return "SSH is currently advertised on a local or private host. Use the same machine or a private network path to connect."
	}
	if sameHost(sshHost, publicHost) {
		return "SSH and the web UI currently share the same public host."
	}
	if publicHost == "" {
		return fmt.Sprintf("SSH commands use %s as the primary terminal target.", sshHost)
	}
	return fmt.Sprintf("Web UI stays on %s, while SSH commands use %s. Keep the SSH target separate when the web hostname only fronts HTTP(S).", publicHost, sshHost)
}

func buildSSHChecks(publicURL, localURL string, connectCommands []map[string]string, sshEnabled bool, sshHost string, sshPort int) []string {
	checks := []string{
		"curl -fsS " + publicURL + "/health",
	}
	if localURL != "" && localURL != publicURL {
		checks = append(checks, "curl -fsS "+localURL+"/health")
	}
	if sshEnabled && strings.TrimSpace(sshHost) != "" && sshPort > 0 {
		checks = append(checks, fmt.Sprintf("nc -vz %s %d", stripHostPort(sshHost), sshPort))
	}
	for _, item := range connectCommands {
		if command := strings.TrimSpace(item["command"]); command != "" {
			checks = append(checks, command)
		}
	}
	checks = append(checks, "skillf convert --url https://example.com/skill.md -o imported-skill.md")
	checks = append(checks, "skillf merge skill-a.md skill-b.md -o merged-skill.md")
	return checks
}

func buildSSHAction(ready, enabled bool, startupError string, diagnostics map[string]interface{}, publicHost, sshHost string) string {
	if ready {
		if !sameHost(publicHost, sshHost) && strings.TrimSpace(publicHost) != "" && strings.TrimSpace(sshHost) != "" {
			return fmt.Sprintf("SSH TUI is ready. Use %s for terminal access while the web UI stays on %s.", sshHost, publicHost)
		}
		return "SSH TUI is ready. Connect from your terminal to import, merge, and inspect skills from one session."
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

func buildSSHRemediation(enabled bool, startupError string, diagnostics map[string]interface{}, publicHost, sshHost string) []string {
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
	if enabled && !sameHost(publicHost, sshHost) && strings.TrimSpace(sshHost) != "" {
		steps = append(steps, fmt.Sprintf("Use %s for SSH sessions; keep %s reserved for the browser UI unless you add a dedicated TCP route.", sshHost, publicHost))
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
	parsedIP := net.ParseIP(hostOnly)
	if parsedIP == nil {
		return false
	}
	return parsedIP.IsLoopback() || parsedIP.IsPrivate() || parsedIP.IsLinkLocalUnicast() || parsedIP.IsLinkLocalMulticast() || parsedIP.IsUnspecified()
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

func sameHost(left, right string) bool {
	return strings.EqualFold(stripHostPort(left), stripHostPort(right))
}
