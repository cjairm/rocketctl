package templates

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed docker-compose.prod.yml.tmpl
var dockerComposeProdTemplate string

//go:embed Caddyfile.tmpl
var caddyfileTemplate string

//go:embed env.production.example.tmpl
var envExampleTemplate string

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
	funcMap := template.FuncMap{
		"upper": strings.ToUpper,
	}
	tmpl, err := template.New("docker-compose.prod.yml").
		Funcs(funcMap).
		Parse(dockerComposeProdTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse docker-compose.prod.yml template: %w", err)
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create docker-compose.prod.yml: %w", err)
	}
	defer file.Close()
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
	if err := os.MkdirAll(caddyDir, 0755); err != nil {
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
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute Caddyfile template: %w", err)
	}

	fmt.Printf("✓ Generated %s\n", outputPath)
	return nil
}

// GenerateEnvProductionExample generates .env.production.example for a service
func GenerateEnvProductionExample(data TemplateData, outputPath string) error {
	tmpl, err := template.New("env.production.example").Parse(envExampleTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse .env.production.example template: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create .env.production.example: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute .env.production.example template: %w", err)
	}

	fmt.Printf("✓ Generated %s\n", outputPath)
	return nil
}

// RenderDockerComposeProd renders docker-compose.prod.yml template to a string
func RenderDockerComposeProd(data TemplateData) (string, error) {
	funcMap := template.FuncMap{
		"upper": strings.ToUpper,
	}
	tmpl, err := template.New("docker-compose.prod.yml").
		Funcs(funcMap).
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

// RenderCaddyfile renders Caddyfile template to a string
func RenderCaddyfile(data TemplateData) (string, error) {
	tmpl, err := template.New("Caddyfile").Parse(caddyfileTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse Caddyfile template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute Caddyfile template: %w", err)
	}
	return buf.String(), nil
}

// RenderEnvExample renders .env.example template to a string
func RenderEnvExample(data TemplateData) (string, error) {
	tmpl, err := template.New("env.example").Parse(envExampleTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse .env.example template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute .env.example template: %w", err)
	}

	return buf.String(), nil
}
