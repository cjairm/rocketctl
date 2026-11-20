package templates

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRenderDockerComposeProdUsesShellSafeVersionVars(t *testing.T) {
	out, err := RenderDockerComposeProd(TemplateData{
		Project:    "myapp",
		Services:   []string{"my-api"},
		Registry:   "reg.example.com",
		IsMonorepo: true,
	})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	want := "image: reg.example.com/myapp_my-api:${MY_API_VERSION:-latest}"
	if !strings.Contains(out, want) {
		t.Errorf("rendered compose is missing %q\ngot:\n%s", want, out)
	}
	if strings.Contains(out, "MY-API_VERSION") {
		t.Error("rendered compose still contains the shell-invalid MY-API_VERSION")
	}
}

func TestRenderDockerComposeProdOmitsObsoleteVersionAttribute(t *testing.T) {
	out, err := RenderDockerComposeProd(TemplateData{
		Project:  "myapp",
		Services: []string{"api"},
		Registry: "reg.example.com",
	})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "version:") {
		t.Error("rendered compose still declares the obsolete top-level version attribute")
	}
}

func TestEdgeComposeIsValidYAML(t *testing.T) {
	var doc map[string]any
	if err := yaml.Unmarshal(EdgeCompose, &doc); err != nil {
		t.Fatalf("edge compose is not valid YAML: %v", err)
	}
	if _, ok := doc["services"]; !ok {
		t.Error("edge compose has no services")
	}
}

func TestRenderWithDomainLeavesProxyToTheEdge(t *testing.T) {
	data := TemplateData{Project: "myapp", Services: []string{"api"}, Registry: "reg.example.com", Domain: "myapp.com"}
	out, err := RenderDockerComposeProd(data)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if strings.Contains(out, "caddy") || strings.Contains(out, "80:80") {
		t.Errorf("compose still has its own Caddy:\n%s", out)
	}
	if !strings.Contains(out, "edge:\n    external: true") {
		t.Errorf("compose does not declare the external edge network:\n%s", out)
	}
	if err := yaml.Unmarshal([]byte(out), &map[string]any{}); err != nil {
		t.Errorf("rendered compose is not valid YAML: %v", err)
	}

	dir := t.TempDir()
	path := dir + "/Caddyfile"
	if err := GenerateCaddyfile(data, path); err != nil {
		t.Fatal(err)
	}
	caddy, _ := os.ReadFile(path)
	if strings.Contains(string(caddy), "email") {
		t.Errorf("Caddyfile has a global email block:\n%s", caddy)
	}
}
