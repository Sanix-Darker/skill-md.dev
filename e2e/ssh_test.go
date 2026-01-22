package e2e

import (
	"context"
	"testing"
	"time"

	sshserver "github.com/sanixdarker/skill-md/internal/ssh"
)

func TestSSHServerCreation(t *testing.T) {
	// Test that SSH server can be created with valid config
	srv, err := sshserver.New(sshserver.Config{
		Port:     12222, // Use different port from main server
		Registry: testApp.RegistryService,
	})
	if err != nil {
		t.Fatalf("failed to create SSH server: %v", err)
	}

	if srv == nil {
		t.Fatal("SSH server is nil")
	}

	// Verify port is set correctly
	if srv.Port() != 12222 {
		t.Errorf("expected port 12222, got %d", srv.Port())
	}
}

func TestSSHServerAddress(t *testing.T) {
	srv, err := sshserver.New(sshserver.Config{
		Port:     12223,
		Registry: testApp.RegistryService,
	})
	if err != nil {
		t.Fatalf("failed to create SSH server: %v", err)
	}

	addr := srv.Addr()
	if addr != ":12223" {
		t.Errorf("expected addr :12223, got %s", addr)
	}
}

func TestSSHServerStartAndShutdown(t *testing.T) {
	srv, err := sshserver.New(sshserver.Config{
		Port:     12224,
		Registry: testApp.RegistryService,
	})
	if err != nil {
		t.Fatalf("failed to create SSH server: %v", err)
	}

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown should work
	ctx, cancel := testContextWithTimeout(2 * time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("shutdown failed: %v", err)
	}

	// Check server stopped cleanly
	select {
	case err := <-errCh:
		// Server should return nil or ErrServerClosed
		if err != nil && err.Error() != "ssh: Server closed" {
			t.Errorf("unexpected server error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("server did not stop within timeout")
	}
}

// testContextWithTimeout creates a context with timeout for tests
func testContextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}
