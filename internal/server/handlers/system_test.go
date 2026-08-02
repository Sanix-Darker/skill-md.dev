package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemHandler_UsesSeparateSSHHost(t *testing.T) {
	application := setupTestApp(t)
	handler := NewSystemHandler(application)

	keyPath := filepath.Join(t.TempDir(), "skillf_ed25519")
	if err := os.WriteFile(keyPath, []byte("test-key"), 0600); err != nil {
		t.Fatalf("failed to write ssh key fixture: %v", err)
	}

	application.Config.Port = 8082
	application.Config.PublicHost = "skillf.sanixdk.xyz"
	application.Config.SSHHost = "178.105.18.9"
	application.Config.SSHPort = 2222
	application.Config.SSHUser = "skillf"
	application.Config.SSHEnabled = true
	application.Config.SSHKeyPath = keyPath

	req := httptest.NewRequest(http.MethodGet, "https://skillf.sanixdk.xyz/api/system", nil)
	req.Host = "skillf.sanixdk.xyz"
	w := httptest.NewRecorder()

	handler.System(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var payload struct {
		SSH struct {
			ConnectCmd        string `json:"connect_cmd"`
			RoutingMode       string `json:"routing_mode"`
			ConnectionNote    string `json:"connection_note"`
			RecommendedAction string `json:"recommended_action"`
			ConnectCommands   []struct {
				Label   string `json:"label"`
				Command string `json:"command"`
			} `json:"connect_commands"`
		} `json:"ssh"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode system payload: %v", err)
	}

	if payload.SSH.ConnectCmd != "ssh -p 2222 skillf@178.105.18.9" {
		t.Fatalf("unexpected connect command: %q", payload.SSH.ConnectCmd)
	}
	if payload.SSH.RoutingMode != "ssh_host_override" {
		t.Fatalf("expected ssh_host_override routing mode, got %q", payload.SSH.RoutingMode)
	}
	if !strings.Contains(payload.SSH.ConnectionNote, "skillf.sanixdk.xyz") || !strings.Contains(payload.SSH.ConnectionNote, "178.105.18.9") {
		t.Fatalf("expected connection note to mention both hosts, got %q", payload.SSH.ConnectionNote)
	}
	if strings.Contains(strings.ToLower(payload.SSH.RecommendedAction), "browse") {
		t.Fatalf("recommended action should not mention removed browse flow: %q", payload.SSH.RecommendedAction)
	}
	if len(payload.SSH.ConnectCommands) != 2 {
		t.Fatalf("expected two connect commands, got %d", len(payload.SSH.ConnectCommands))
	}
	if payload.SSH.ConnectCommands[0].Command != "ssh -p 2222 skillf@178.105.18.9" {
		t.Fatalf("unexpected primary SSH command: %q", payload.SSH.ConnectCommands[0].Command)
	}
	if payload.SSH.ConnectCommands[1].Command != "ssh -p 2222 skillf@skillf.sanixdk.xyz" {
		t.Fatalf("unexpected secondary SSH command: %q", payload.SSH.ConnectCommands[1].Command)
	}
}
