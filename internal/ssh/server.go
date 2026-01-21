// Package ssh provides an SSH server for the TUI interface.
package ssh

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"github.com/sanixdarker/skill-md/internal/registry"
	"github.com/sanixdarker/skill-md/internal/tui"
)

// validateKeyPermissions checks that the SSH key file has secure permissions (0600).
func validateKeyPermissions(keyPath string) error {
	info, err := os.Stat(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Key doesn't exist yet, will be created
		}
		return fmt.Errorf("failed to stat key file: %w", err)
	}

	perms := info.Mode().Perm()
	if perms != 0600 {
		return fmt.Errorf("SSH key has insecure permissions %o, expected 0600", perms)
	}
	return nil
}

// Server represents the SSH server.
type Server struct {
	registry *registry.Service
	server   *ssh.Server
	port     int
	keyPath  string
}

// Config holds server configuration.
type Config struct {
	Port     int
	KeyPath  string
	Registry *registry.Service
}

// New creates a new SSH server.
func New(cfg Config) (*Server, error) {
	if cfg.Port == 0 {
		cfg.Port = 2222
	}

	if cfg.KeyPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home dir: %w", err)
		}
		cfg.KeyPath = filepath.Join(home, ".ssh", "skill-md_ed25519")
	}

	// Ensure key directory exists
	keyDir := filepath.Dir(cfg.KeyPath)
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create key directory: %w", err)
	}

	// Validate key permissions if key exists
	if err := validateKeyPermissions(cfg.KeyPath); err != nil {
		return nil, err
	}

	s := &Server{
		registry: cfg.Registry,
		port:     cfg.Port,
		keyPath:  cfg.KeyPath,
	}

	server, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf(":%d", cfg.Port)),
		wish.WithHostKeyPath(cfg.KeyPath),
		wish.WithMiddleware(
			bubbletea.Middleware(s.teaHandler),
			logging.Middleware(),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH server: %w", err)
	}

	s.server = server
	return s, nil
}

// teaHandler returns a bubbletea handler for the session.
func (s *Server) teaHandler(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
	_, _, _ = sess.Pty()

	renderer := bubbletea.MakeRenderer(sess)

	// Override lipgloss renderer for this session
	lipgloss.SetDefaultRenderer(renderer)

	model := tui.NewModel(s.registry)

	// Send initial window size
	return model, []tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithOutput(sess),
		tea.WithInput(sess),
	}
}

// Start starts the SSH server.
func (s *Server) Start() error {
	log.Info("Starting SSH server", "port", s.port)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("SSH server error", "error", err)
		}
	}()

	<-done
	log.Info("Shutting down SSH server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return s.server.Shutdown(ctx)
}

// ListenAndServe starts the SSH server and blocks.
func (s *Server) ListenAndServe() error {
	log.Info("SSH server listening", "port", s.port)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Addr returns the server address string.
func (s *Server) Addr() string {
	return fmt.Sprintf(":%d", s.port)
}

// Port returns the configured port.
func (s *Server) Port() int {
	return s.port
}
