package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/sanixdarker/skill-md/internal/app"
	"github.com/sanixdarker/skill-md/internal/server"
)

var (
	testApp    *app.App
	testServer *server.Server
	testPort   = 18080
	baseURL    string
)

// TestMain sets up the test environment
func TestMain(m *testing.M) {
	// Create a temporary database for testing
	tmpDir, err := os.MkdirTemp("", "skillmd-e2e-*")
	if err != nil {
		fmt.Printf("failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := tmpDir + "/test.db"

	// Initialize app
	cfg := &app.Config{
		Port:   testPort,
		DBPath: dbPath,
		Debug:  false,
	}

	testApp, err = app.New(cfg)
	if err != nil {
		fmt.Printf("failed to initialize app: %v\n", err)
		os.Exit(1)
	}
	defer testApp.Close()

	// Start server
	testServer = server.New(testApp)
	baseURL = fmt.Sprintf("http://localhost:%d", testPort)

	go func() {
		if err := testServer.Start(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("server error: %v\n", err)
		}
	}()

	// Wait for server to start
	if err := waitForServer(baseURL, 5*time.Second); err != nil {
		fmt.Printf("server failed to start: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	testServer.Shutdown()

	os.Exit(code)
}

// waitForServer waits for the server to be ready
func waitForServer(url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for server")
		case <-ticker.C:
			resp, err := http.Get(url)
			if err == nil {
				resp.Body.Close()
				return nil
			}
		}
	}
}

// getTestURL returns the full URL for a given path
func getTestURL(path string) string {
	return baseURL + path
}
