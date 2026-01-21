package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sanixdarker/skill-md/internal/app"
	"github.com/sanixdarker/skill-md/internal/server"
	sshserver "github.com/sanixdarker/skill-md/internal/ssh"
	"github.com/spf13/cobra"
)

var (
	servePort        int
	serveSSHPort     int
	serveDBPath      string
	serveDebug       bool
	serveNoSSH       bool
	serveGitHubToken string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the web server",
	Long: `Start the Skill MD web server with the UI and API endpoints.

Optionally starts an SSH server for terminal UI access.

Examples:
  skillmd serve
  skillmd serve --port 8080 --ssh-port 2222
  skillmd serve --no-ssh

Connect via SSH:
  ssh localhost -p 2222`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Use environment variable if flag not set
		githubToken := serveGitHubToken
		if githubToken == "" {
			githubToken = os.Getenv("GITHUB_TOKEN")
		}

		cfg := &app.Config{
			Port:        servePort,
			DBPath:      serveDBPath,
			Debug:       serveDebug,
			GitHubToken: githubToken,
		}

		application, err := app.New(cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize application: %w", err)
		}
		defer application.Close()

		srv := server.New(application)

		// Handle graceful shutdown
		done := make(chan os.Signal, 1)
		signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

		// Start SSH server if enabled
		var sshSrv *sshserver.Server
		if !serveNoSSH {
			sshSrv, err = sshserver.New(sshserver.Config{
				Port:     serveSSHPort,
				Registry: application.RegistryService,
			})
			if err != nil {
				application.Logger.Warn("failed to initialize SSH server", "error", err)
			} else {
				go func() {
					if err := sshSrv.ListenAndServe(); err != nil {
						application.Logger.Error("SSH server error", "error", err)
					}
				}()
				fmt.Printf("SSH TUI available at ssh://localhost:%d\n", serveSSHPort)
			}
		}

		go func() {
			<-done
			application.Logger.Info("shutting down servers...")

			// Shutdown SSH server
			if sshSrv != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				sshSrv.Shutdown(ctx)
			}

			srv.Shutdown()
		}()

		application.Logger.Info("starting server", "port", cfg.Port)
		fmt.Printf("Skill MD web server running at http://localhost:%d\n", cfg.Port)

		return srv.Start()
	},
}

func init() {
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 8080, "HTTP port to listen on")
	serveCmd.Flags().IntVar(&serveSSHPort, "ssh-port", 2222, "SSH port for TUI access")
	serveCmd.Flags().StringVar(&serveDBPath, "db", "./skill-md.db", "Path to SQLite database")
	serveCmd.Flags().BoolVar(&serveDebug, "debug", false, "Enable debug mode")
	serveCmd.Flags().BoolVar(&serveNoSSH, "no-ssh", false, "Disable SSH server")
	serveCmd.Flags().StringVar(&serveGitHubToken, "github-token", "", "GitHub API token (or set GITHUB_TOKEN env var)")

	rootCmd.AddCommand(serveCmd)
}
