package templates

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/cjairm/rocketctl/internal/config"
)

//go:embed docker-compose.prod.yml.tmpl
var dockerComposeProdTemplate string

//go:embed Caddyfile.tmpl
var caddyfileTemplate string

// composeFuncs is shared by both compose renderers so the version variable
// spelled in the template can never drift from the one the Go code sets.
var composeFuncs = template.FuncMap{
	"envkey": config.EnvVersionKey,
}

// TemplateData holds data for template rendering
type TemplateData struct {
	Project    string
	Service    string
	Services   []string
	Registry   string
	Region     string
	Domain     string
	Email      string
	IsMonorepo bool
}

// GenerateDockerComposeProd generates docker-compose.prod.yml
func GenerateDockerComposeProd(data TemplateData, outputPath string) error {
	tmpl, err := template.New("docker-compose.prod.yml").
		Funcs(composeFuncs).
		Parse(dockerComposeProdTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse docker-compose.prod.yml template: %w", err)
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create docker-compose.prod.yml: %w", err)
	}
	defer func() { _ = file.Close() }()
	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute docker-compose.prod.yml template: %w", err)
	}
	fmt.Printf("✓ Generated %s\n", outputPath)
	return nil
}

// GenerateCaddyfile generates caddy/Caddyfile
func GenerateCaddyfile(data TemplateData, outputPath string) error {
	// Create caddy directory if it doesn't exist
	caddyDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(caddyDir, 0o755); err != nil {
		return fmt.Errorf("failed to create caddy directory: %w", err)
	}

	tmpl, err := template.New("Caddyfile").Parse(caddyfileTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse Caddyfile template: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create Caddyfile: %w", err)
	}
	defer func() { _ = file.Close() }()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute Caddyfile template: %w", err)
	}

	fmt.Printf("✓ Generated %s\n", outputPath)
	return nil
}

// RenderDockerComposeProd renders docker-compose.prod.yml template to a string
func RenderDockerComposeProd(data TemplateData) (string, error) {
	tmpl, err := template.New("docker-compose.prod.yml").
		Funcs(composeFuncs).
		Parse(dockerComposeProdTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse docker-compose.prod.yml template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute docker-compose.prod.yml template: %w", err)
	}
	return buf.String(), nil
}
