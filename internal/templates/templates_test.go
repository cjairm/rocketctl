package templates

import (
	"strings"
	"testing"
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
