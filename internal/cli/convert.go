package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/sanixdarker/skillforge/internal/converter"
	"github.com/sanixdarker/skillforge/pkg/skill"
	"github.com/spf13/cobra"
)

var (
	convertFormat string
	convertOutput string
	convertName   string
	convertURL    string
)

var convertCmd = &cobra.Command{
	Use:   "convert [file]",
	Short: "Convert a specification file or URL to SKILL.md format",
	Long: `Convert various specification formats to SKILL.md format.

Supported formats:
  - openapi:      OpenAPI 3.x specifications (YAML/JSON)
  - graphql:      GraphQL schema definitions
  - postman:      Postman collection files
  - asyncapi:     AsyncAPI event-driven API specs (Kafka, MQTT, WebSocket)
  - proto:        Protocol Buffer/gRPC service definitions
  - raml:         RAML 1.0 specifications
  - wsdl:         WSDL/SOAP web service definitions
  - apiblueprint: API Blueprint Markdown specifications
  - pdf:          PDF documents
  - url:          Web pages and documentation URLs
  - text:         Plain text descriptions

Examples:
  skillforge convert api.yaml
  skillforge convert schema.graphql -f graphql
  skillforge convert api.yaml -o skill.md -n "My API"
  skillforge convert events.yaml -f asyncapi
  skillforge convert service.proto -f proto
  skillforge convert api.raml -f raml
  skillforge convert service.wsdl -f wsdl
  skillforge convert api.apib -f apiblueprint
  skillforge convert --url https://docs.example.com/api`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var content []byte
		var sourcePath string
		var format string

		// Create converter manager
		manager := converter.NewManager()

		// Check if URL is provided
		if convertURL != "" {
			// URL conversion
			content = []byte(convertURL)
			sourcePath = convertURL
			format = "url"

			fmt.Printf("Fetching URL: %s\n", convertURL)
		} else if len(args) > 0 {
			// File conversion
			inputPath := args[0]

			// Check if the argument is a URL
			if strings.HasPrefix(inputPath, "http://") || strings.HasPrefix(inputPath, "https://") {
				content = []byte(inputPath)
				sourcePath = inputPath
				format = "url"
				fmt.Printf("Fetching URL: %s\n", inputPath)
			} else {
				// Read input file
				var err error
				content, err = os.ReadFile(inputPath)
				if err != nil {
					return fmt.Errorf("failed to read input file: %w", err)
				}
				sourcePath = inputPath
				format = convertFormat
			}
		} else {
			return fmt.Errorf("please provide a file path or URL (--url)")
		}

		// Determine format if not set
		if format == "" {
			format = manager.DetectFormat(sourcePath, content)
		}

		// Convert
		result, err := manager.Convert(format, content, &converter.Options{
			Name:       convertName,
			SourcePath: sourcePath,
		})
		if err != nil {
			return fmt.Errorf("conversion failed: %w", err)
		}

		// Render output
		output := skill.Render(result)

		// Write output
		if convertOutput != "" {
			if err := os.WriteFile(convertOutput, []byte(output), 0644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}
			fmt.Printf("SKILL.md written to %s\n", convertOutput)
		} else {
			fmt.Println(output)
		}

		return nil
	},
}

func init() {
	convertCmd.Flags().StringVarP(&convertFormat, "format", "f", "", "Input format (openapi, graphql, postman, asyncapi, proto, raml, wsdl, apiblueprint, pdf, url, text)")
	convertCmd.Flags().StringVarP(&convertOutput, "output", "o", "", "Output file path")
	convertCmd.Flags().StringVarP(&convertName, "name", "n", "", "Name for the skill")
	convertCmd.Flags().StringVarP(&convertURL, "url", "u", "", "URL to fetch and convert")

	rootCmd.AddCommand(convertCmd)
}
