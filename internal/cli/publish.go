package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/sanixdarker/skillf/pkg/skill"
	"github.com/sanixdarker/skillf/pkg/validation"
	"github.com/spf13/cobra"
)

var (
	publishRegistry  string
	publishToken     string
	publishDryRun    bool
	publishForce     bool
	publishSkipValid bool
)

var publishCmd = &cobra.Command{
	Use:   "publish [file]",
	Short: "Publish a SKILL.md to a registry",
	Long: `Publish a SKILL.md file to a skill registry.

  The registry URL can be specified with --registry or the SKILLF_REGISTRY
environment variable. Authentication is done via --token or SKILLF_TOKEN.

Examples:
  skillf publish SKILL.md
  skillf publish SKILL.md --registry https://skills.sh
  skillf publish SKILL.md --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		// Read file
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		// Parse skill
		s, err := skill.Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse skill: %w", err)
		}

		// Validate unless skipped
		if !publishSkipValid {
			opts := validation.DefaultOptions()
			opts.MCPCompat = true
			validator := validation.NewValidator(opts)
			result := validator.Validate(s)

			if !result.Valid {
				fmt.Println("Validation failed:")
				for _, issue := range result.Issues {
					if issue.Severity >= validation.SeverityError {
						fmt.Printf("  [%s] %s: %s\n", issue.Severity, issue.Code, issue.Message)
					}
				}
				if !publishForce {
					return fmt.Errorf("skill validation failed, use --force to publish anyway")
				}
				fmt.Println("\nContinuing with --force...")
			}
		}

		// Get registry URL
		registry := publishRegistry
		if registry == "" {
			registry = os.Getenv("SKILLF_REGISTRY")
		}
		if registry == "" {
			registry = "https://skills.sh"
		}
		registry = strings.TrimSuffix(registry, "/")

		// Get token
		token := publishToken
		if token == "" {
			token = os.Getenv("SKILLF_TOKEN")
		}

		// Build publish payload
		payload := PublishPayload{
			Name:        s.Frontmatter.Name,
			Version:     s.Frontmatter.Version,
			Description: s.Frontmatter.Description,
			Content:     string(content),
			ContentHash: hashContent(string(content)),
			Tags:        s.Frontmatter.Tags,
		}

		// Dry run - just show what would be published
		if publishDryRun {
			fmt.Println("Dry run - would publish:")
			fmt.Printf("  Registry: %s\n", registry)
			fmt.Printf("  Name: %s\n", payload.Name)
			fmt.Printf("  Version: %s\n", payload.Version)
			fmt.Printf("  Content hash: %s\n", payload.ContentHash)
			fmt.Printf("  Tags: %v\n", payload.Tags)
			fmt.Printf("  Content size: %d bytes\n", len(payload.Content))
			return nil
		}

		// Publish to registry
		if token == "" {
			return fmt.Errorf("no authentication token provided (use --token or SKILLF_TOKEN)")
		}

		result, err := publishToRegistry(registry, token, payload)
		if err != nil {
			return fmt.Errorf("publish failed: %w", err)
		}

		fmt.Printf("✓ Published %s@%s\n", payload.Name, payload.Version)
		fmt.Printf("  URL: %s\n", result.URL)
		fmt.Printf("  Hash: %s\n", result.ContentHash)

		return nil
	},
}

// PublishPayload is the payload sent to the registry.
type PublishPayload struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	Content     string   `json:"content"`
	ContentHash string   `json:"content_hash"`
	Tags        []string `json:"tags,omitempty"`
}

// PublishResult is the response from the registry.
type PublishResult struct {
	Success     bool   `json:"success"`
	URL         string `json:"url"`
	ContentHash string `json:"content_hash"`
	Message     string `json:"message,omitempty"`
}

func publishToRegistry(registry, token string, payload PublishPayload) (*PublishResult, error) {
	url := registry + "/api/skills"

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "skillf/"+Version)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("registry error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result PublishResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

func hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

func init() {
	publishCmd.Flags().StringVarP(&publishRegistry, "registry", "r", "", "Registry URL (default: SKILLF_REGISTRY or skills.sh)")
	publishCmd.Flags().StringVarP(&publishToken, "token", "t", "", "Authentication token (default: SKILLF_TOKEN)")
	publishCmd.Flags().BoolVar(&publishDryRun, "dry-run", false, "Show what would be published without actually publishing")
	publishCmd.Flags().BoolVar(&publishForce, "force", false, "Publish even if validation fails")
	publishCmd.Flags().BoolVar(&publishSkipValid, "skip-validation", false, "Skip validation entirely")
	rootCmd.AddCommand(publishCmd)
}
