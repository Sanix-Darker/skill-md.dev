package cli

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/user"
	"os/signal"
	"sort"
	"strconv"
	"strings"
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
	servePublicHost  string
	serveListenHost  string
	serveSSHKeyPath  string
	serveSSHUser     string
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

		publicHost := normalizePublicHost(servePublicHost)
		if publicHost == "" {
			publicHost = normalizePublicHost(os.Getenv("SKILLF_PUBLIC_HOST"))
		}
		if publicHost == "" {
			publicHost = "127.0.0.1"
		}
		listenHost := normalizeListenHost(serveListenHost)
		if listenHost == "" {
			listenHost = normalizeListenHost(os.Getenv("SKILLF_LISTEN_HOST"))
		}
		if listenHost == "" {
			listenHost = "0.0.0.0"
		}
		sshHost := normalizePublicHost(servePublicHost)
		if sshHost == "" {
			sshHost = publicHost
		}
		sshKeyPath := strings.TrimSpace(serveSSHKeyPath)
		if sshKeyPath == "" {
			sshKeyPath = strings.TrimSpace(os.Getenv("SKILLF_SSH_KEY_PATH"))
		}
		resolvedSSHKeyPath, err := sshserver.ResolveKeyPath(sshKeyPath)
		if err != nil {
			return err
		}

		cfg := &app.Config{
			Port:       servePort,
			DBPath:     serveDBPath,
			Debug:      serveDebug,
			Version:    Version,
			PublicHost: publicHost,
			ListenHost: listenHost,
			// SSH settings are finalized after SSH server initialization.
			SSHHost:     sshHost,
			SSHPort:     serveSSHPort,
			SSHUser:     resolveSSHUser(serveSSHUser),
			SSHKeyPath:  resolvedSSHKeyPath,
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
				KeyPath:         application.Config.SSHKeyPath,
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
				application.Config.SSHHost = sshHost
				application.Config.SSHKeyPath = strings.TrimSpace(sshSrv.KeyPath())
				application.Config.SSHEnabled = true
			}
		} else {
			application.Config.SSHStartupError = "disabled via --no-ssh"
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

		application.Config.SSHEnabled = sshEnabled
		if !sshEnabled && sshErr != nil {
			application.Config.SSHStartupError = fmt.Sprintf("%v", sshErr)
		}
		printStartupSummary(application, servePort, sshEnabled, sshErr, sshSrv)

		return srv.Start()
	},
}

func printStartupSummary(application *app.App, webPort int, sshEnabled bool, sshErr error, sshSrv *sshserver.Server) {
	host := strings.TrimSpace(application.Config.PublicHost)
	if host == "" {
		host = "127.0.0.1"
	}
	listenHost := normalizeListenHost(application.Config.ListenHost)
	if listenHost == "" {
		listenHost = "0.0.0.0"
	}
	webBaseURL := buildPublicURL(host, webPort)
	localWebURL := primaryListenURL(listenHost, webPort)
	formattedPort, err := localAddresses(listenHost, webPort)
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

	fmt.Printf("\nskillf started\n")
	fmt.Printf("  Web UI:      %s\n", webBaseURL)
	if webBaseURL != localWebURL {
		fmt.Printf("  Local UI:    %s\n", localWebURL)
	}
	fmt.Printf("  Health:      %s/health\n", webBaseURL)
	fmt.Printf("  Docs:        %s\n\n", webBaseURL)

	if !sshEnabled {
		fmt.Println("  SSH TUI:     disabled")
		if serveNoSSH {
			fmt.Println("  - Started in web-only mode with --no-ssh")
		} else if sshErr != nil {
			fmt.Printf("  - Failed to start: %v\n", sshErr)
			if keyPath := strings.TrimSpace(application.Config.SSHKeyPath); keyPath != "" {
				fmt.Printf("  - Fix suggestion: chmod 600 %s && skillf serve\n", keyPath)
			}
			fmt.Println("  - Or start web-only: skillf serve --no-ssh")
		}
		fmt.Println()
	} else {
		fmt.Printf("  SSH TUI:     %s\n", sshSrv.Addr())
		sshHost := application.Config.SSHHost
		if sshHost == "" {
			sshHost = host
		}
		sshCommand := formatSSHCommand(sshSrv.Port(), sshHost, application.Config.SSHUser)
		fmt.Printf("  SSH Connect: %s\n", sshCommand)
		keyPath := strings.TrimSpace(application.Config.SSHKeyPath)
		if keyPath != "" {
			fmt.Printf("  SSH key:     %s\n", keyPath)
		}
		fmt.Println()
	}
	fmt.Println("Diagnostics:")
	fmt.Printf("  - To check web endpoints: curl -fsS %s/health\n", webBaseURL)
	if webBaseURL != localWebURL {
		fmt.Printf("  - Local health check:     curl -fsS %s/health\n", localWebURL)
	}
	fmt.Printf("  - To open UI in terminal: skillf serve --help\n")
}

func localAddresses(listenHost string, webPort int) ([]string, error) {
	if !isAllInterfacesHost(listenHost) {
		return []string{primaryListenURL(listenHost, webPort)}, nil
	}

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
	serveCmd.Flags().StringVar(&servePublicHost, "public-host", "", "Public host name for external SSH and share links (or set SKILLF_PUBLIC_HOST)")
	serveCmd.Flags().StringVar(&serveListenHost, "listen-host", "", "Listen host/interface for HTTP server (or set SKILLF_LISTEN_HOST)")
	serveCmd.Flags().StringVar(&serveSSHKeyPath, "ssh-key", "", "SSH host key path (or set SKILLF_SSH_KEY_PATH)")
	serveCmd.Flags().StringVar(&serveSSHUser, "ssh-user", "", "SSH user shown in generated connect commands")

	rootCmd.AddCommand(serveCmd)
}

func resolveSSHUser(requested string) string {
	if strings.TrimSpace(requested) != "" {
		return strings.TrimSpace(requested)
	}

	currentUser, err := user.Current()
	if err != nil {
		return ""
	}
	return currentUser.Username
}

func formatSSHCommand(port int, host, user string) string {
	targetHost := strings.TrimSpace(host)
	if targetHost == "" {
		targetHost = "127.0.0.1"
	}

	targetUser := strings.TrimSpace(user)
	target := targetHost
	if targetUser != "" {
		target = fmt.Sprintf("%s@%s", targetUser, targetHost)
	}

	if port <= 0 {
		port = 2222
	}

	return fmt.Sprintf("ssh -p %d %s", port, target)
}

func normalizePublicHost(value string) string {
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

func normalizeListenHost(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return ""
	}
	if strings.Contains(trimmedValue, "://") {
		parsedURL, err := url.Parse(trimmedValue)
		if err == nil && parsedURL.Host != "" {
			if parsedHost, _, splitErr := net.SplitHostPort(parsedURL.Host); splitErr == nil {
				return parsedHost
			}
			return parsedURL.Host
		}
	}
	return strings.Trim(trimmedValue, "[]")
}

func isLocalHost(host string) bool {
	hostOnly := host
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		hostOnly = parsedHost
	}
	hostOnly = strings.Trim(hostOnly, "[]")
	if hostOnly == "" {
		return true
	}
	if hostOnly == "localhost" {
		return true
	}
	return net.ParseIP(hostOnly) != nil
}

func isAllInterfacesHost(host string) bool {
	switch strings.TrimSpace(strings.Trim(host, "[]")) {
	case "", "0.0.0.0", "::":
		return true
	default:
		return false
	}
}

func primaryListenURL(listenHost string, port int) string {
	if isAllInterfacesHost(listenHost) {
		return fmt.Sprintf("http://127.0.0.1:%d", port)
	}
	return fmt.Sprintf("http://%s", net.JoinHostPort(strings.Trim(listenHost, "[]"), strconv.Itoa(port)))
}

func buildPublicURL(host string, port int) string {
	normalizedHost := normalizePublicHost(host)
	if normalizedHost == "" {
		normalizedHost = "127.0.0.1"
	}

	scheme := "https"
	if isLocalHost(normalizedHost) {
		scheme = "http"
	}

	if _, _, err := net.SplitHostPort(normalizedHost); err == nil {
		return fmt.Sprintf("%s://%s", scheme, normalizedHost)
	}

	if isLocalHost(normalizedHost) {
		return fmt.Sprintf("%s://%s:%d", scheme, normalizedHost, port)
	}

	return fmt.Sprintf("%s://%s", scheme, normalizedHost)
}
