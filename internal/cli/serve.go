package cli

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/sanixdarker/skillf/internal/app"
	"github.com/sanixdarker/skillf/internal/server"
	sshserver "github.com/sanixdarker/skillf/internal/ssh"
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
	Long: `Start the Skillf web server with the UI and API endpoints.

Optionally starts an SSH server for terminal UI access.

Examples:
  skillf serve
  skillf serve --port 8080 --ssh-port 2222
  skillf serve --no-ssh

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
		var sshErr error
		sshEnabled := false
		if !serveNoSSH {
			sshSrv, err = sshserver.New(sshserver.Config{
				Port:            serveSSHPort,
				Registry:        application.RegistryService,
				FederatedSource: application.FederatedSource,
			})
			if err != nil {
				application.Logger.Warn("SSH server disabled", "error", err)
				sshErr = err
			} else {
				go func() {
					if err := sshSrv.ListenAndServe(); err != nil {
						application.Logger.Error("SSH server error", "error", err)
					}
				}()
				sshEnabled = true
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

		printStartupSummary(application, servePort, sshEnabled, sshErr, sshSrv)

		return srv.Start()
	},
}

func printStartupSummary(application *app.App, webPort int, sshEnabled bool, sshErr error, sshSrv *sshserver.Server) {
	host := "127.0.0.1"
	webBaseURL := fmt.Sprintf("http://%s:%d", host, webPort)
	formattedPort, err := localAddresses(webPort)
	if err != nil {
		application.Logger.Warn("unable to detect local addresses", "error", err)
	} else {
		if len(formattedPort) > 0 {
			fmt.Println("Available via:")
			for _, addr := range formattedPort {
				fmt.Printf("  - %s\n", addr)
			}
		}
	}

	fmt.Printf("\n🚀 skillf started\n")
	fmt.Printf("  Web UI:      %s\n", webBaseURL)
	fmt.Printf("  Health:      %s/health\n", webBaseURL)
	fmt.Printf("  Docs:        https://skillf.sanixdk.xyz\n\n")

	if !sshEnabled {
		fmt.Println("  SSH TUI:     disabled")
		if serveNoSSH {
			fmt.Println("  - Started in web-only mode with --no-ssh")
		} else if sshErr != nil {
			fmt.Printf("  - Failed to start: %v\n", sshErr)
			fmt.Println("  - Fix suggestion: chmod 600 ~/.ssh/skillf_ed25519 && skillf serve")
			fmt.Println("  - Or start web-only: skillf serve --no-ssh")
		}
		fmt.Println()
	} else {
		fmt.Printf("  SSH TUI:     %s\n", sshSrv.Addr())
		fmt.Printf("  SSH Connect: ssh -p %d 127.0.0.1\n", sshSrv.Port())
		fmt.Printf("  SSH key:     %s\n", sshSrv.KeyPath())
		fmt.Println()
	}
	fmt.Println("Diagnostics:")
	fmt.Printf("  - To check web endpoints: curl -fsS %s/health\n", webBaseURL)
	fmt.Printf("  - To open UI in terminal: skillf serve --help\n")
}

func localAddresses(webPort int) ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	addrs := make([]string, 0, len(ifaces)+1)
	addrs = append(addrs, fmt.Sprintf("http://127.0.0.1:%d", webPort))
	for _, iface := range ifaces {
		addrList, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrList {
			switch value := addr.(type) {
			case *net.IPNet:
				ip := value.IP
				if ip == nil || ip.IsLoopback() {
					continue
				}
				if ip.To4() == nil {
					continue
				}
				addrs = append(addrs, fmt.Sprintf("http://%s:%d", ip.String(), webPort))
			case *net.IPAddr:
				ip := value.IP
				if ip == nil || ip.IsLoopback() {
					continue
				}
				if ip.To4() == nil {
					continue
				}
				addrs = append(addrs, fmt.Sprintf("http://%s:%d", ip.String(), webPort))
			}
		}
	}

	sort.SliceStable(addrs, func(i, j int) bool {
		return addrs[i] < addrs[j]
	})
	seen := make(map[string]struct{}, len(addrs))
	unique := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		unique = append(unique, addr)
	}
	return unique, nil
}

func init() {
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 8080, "HTTP port to listen on")
	serveCmd.Flags().IntVar(&serveSSHPort, "ssh-port", 2222, "SSH port for TUI access")
	serveCmd.Flags().StringVar(&serveDBPath, "db", "./skillf.db", "Path to SQLite database")
	serveCmd.Flags().BoolVar(&serveDebug, "debug", false, "Enable debug mode")
	serveCmd.Flags().BoolVar(&serveNoSSH, "no-ssh", false, "Disable SSH server")
	serveCmd.Flags().StringVar(&serveGitHubToken, "github-token", "", "GitHub API token (or set GITHUB_TOKEN env var)")

	rootCmd.AddCommand(serveCmd)
}
