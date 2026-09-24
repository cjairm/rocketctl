package config

import (
	"os"
	"path/filepath"
	"testing"
)

func singleService() *Config {
	return &Config{
		Project:  "myapp",
		Service:  "backend",
		Registry: "reg.example.com",
		Region:   "us-east-2",
	}
}

func monorepo() *Config {
	return &Config{
		Project:  "myapp",
		Services: []string{"api", "web"},
		Registry: "reg.example.com",
		Region:   "us-east-2",
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{name: "single service config is valid", cfg: singleService()},
		{name: "monorepo config is valid", cfg: monorepo()},
		{
			name:    "missing project is rejected",
			cfg:     &Config{Service: "backend", Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: true,
		},
		{
			name:    "missing registry is rejected",
			cfg:     &Config{Project: "myapp", Service: "backend", Region: "us-east-2"},
			wantErr: true,
		},
		{
			name:    "missing region is rejected",
			cfg:     &Config{Project: "myapp", Service: "backend", Registry: "reg.example.com"},
			wantErr: true,
		},
		{
			name: "service and services are mutually exclusive",
			cfg: &Config{
				Project:  "myapp",
				Service:  "backend",
				Services: []string{"api"},
				Registry: "reg.example.com",
				Region:   "us-east-2",
			},
			wantErr: true,
		},
		{
			name:    "neither service nor services is rejected",
			cfg:     &Config{Project: "myapp", Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate returned nil error, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate unexpected error: %v", err)
			}
		})
	}
}

func TestIsMonorepoAndGetServices(t *testing.T) {
	if singleService().IsMonorepo() {
		t.Error("IsMonorepo on a single-service config = true, want false")
	}
	if !monorepo().IsMonorepo() {
		t.Error("IsMonorepo on a monorepo config = false, want true")
	}

	got := singleService().GetServices()
	if len(got) != 1 || got[0] != "backend" {
		t.Errorf("GetServices (single) = %v, want [backend]", got)
	}

	got = monorepo().GetServices()
	if len(got) != 2 || got[0] != "api" || got[1] != "web" {
		t.Errorf("GetServices (monorepo) = %v, want [api web]", got)
	}
}

func TestValidateService(t *testing.T) {
	if err := monorepo().ValidateService("api"); err != nil {
		t.Errorf("ValidateService(api) unexpected error: %v", err)
	}
	if err := monorepo().ValidateService("nope"); err == nil {
		t.Error("ValidateService(nope) returned nil error, want error")
	}
	if err := singleService().ValidateService("backend"); err != nil {
		t.Errorf("ValidateService(backend) unexpected error: %v", err)
	}
	if err := singleService().ValidateService("api"); err == nil {
		t.Error("ValidateService(api) on single-service returned nil error, want error")
	}
}

// Path values assert filepath.Join's cleaned output: a monorepo service
// directory is "api", not "./api".
func TestPathDerivation(t *testing.T) {
	tests := []struct {
		name    string
		got     func() (string, error)
		want    string
		wantErr bool
	}{
		{
			name: "single service directory is the project root",
			got:  func() (string, error) { return singleService().GetServiceDirectory("backend") },
			want: ".",
		},
		{
			name: "monorepo service directory is a cleaned subfolder",
			got:  func() (string, error) { return monorepo().GetServiceDirectory("api") },
			want: "api",
		},
		{
			name:    "unknown service is rejected",
			got:     func() (string, error) { return monorepo().GetServiceDirectory("nope") },
			wantErr: true,
		},
		{
			name: "single service version file sits at the root",
			got:  func() (string, error) { return singleService().GetVersionFilePath("backend") },
			want: ".rocket-version",
		},
		{
			name: "monorepo version file sits under the service",
			got:  func() (string, error) { return monorepo().GetVersionFilePath("api") },
			want: "api/.rocket-version",
		},
		{
			name: "single service production dockerfile",
			got:  func() (string, error) { return singleService().GetDockerfilePath("backend", true) },
			want: "Dockerfile.production",
		},
		{
			name: "single service development dockerfile",
			got:  func() (string, error) { return singleService().GetDockerfilePath("backend", false) },
			want: "Dockerfile",
		},
		{
			name: "monorepo production dockerfile",
			got:  func() (string, error) { return monorepo().GetDockerfilePath("api", true) },
			want: "api/Dockerfile.production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.got()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("got %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestImageNaming(t *testing.T) {
	cfg := monorepo()

	if got := cfg.GetImageName("api"); got != "myapp_api" {
		t.Errorf("GetImageName = %q, want %q", got, "myapp_api")
	}

	want := "reg.example.com/myapp_api:1.2.3"
	if got := cfg.GetFullImageName("api", "1.2.3"); got != want {
		t.Errorf("GetFullImageName = %q, want %q", got, want)
	}
}

func TestLoadReadsRocketYAMLFromWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	contents := "project: myapp\nservice: backend\nregistry: reg.example.com\nregion: us-east-2\n"
	if err := os.WriteFile(filepath.Join(dir, "rocket.yaml"), []byte(contents), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	t.Chdir(dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load unexpected error: %v", err)
	}
	if cfg.Project != "myapp" || cfg.Service != "backend" {
		t.Errorf("Load = %+v, want project=myapp service=backend", cfg)
	}
}

func TestLoadFailsWhenRocketYAMLIsMissing(t *testing.T) {
	t.Chdir(t.TempDir())

	if _, err := Load(); err == nil {
		t.Fatal("Load without rocket.yaml returned nil error, want error")
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	// Missing the required region field.
	contents := "project: myapp\nservice: backend\nregistry: reg.example.com\n"
	if err := os.WriteFile(filepath.Join(dir, "rocket.yaml"), []byte(contents), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	t.Chdir(dir)

	if _, err := Load(); err == nil {
		t.Fatal("Load with a missing region returned nil error, want error")
	}
}

func TestSaveWritesALoadableConfig(t *testing.T) {
	dir := t.TempDir()
	if err := monorepo().Save(filepath.Join(dir, "rocket.yaml")); err != nil {
		t.Fatalf("Save unexpected error: %v", err)
	}
	t.Chdir(dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load after Save unexpected error: %v", err)
	}
	if !cfg.IsMonorepo() || len(cfg.Services) != 2 {
		t.Errorf("round-tripped config = %+v, want monorepo with 2 services", cfg)
	}
}

func TestEnvVersionKey(t *testing.T) {
	tests := []struct {
		service string
		want    string
	}{
		{"api", "API_VERSION"},
		{"my-api", "MY_API_VERSION"},
		{"web.ui", "WEB_UI_VERSION"},
		{"Api2", "API2_VERSION"},
	}
	for _, tt := range tests {
		t.Run(tt.service, func(t *testing.T) {
			if got := EnvVersionKey(tt.service); got != tt.want {
				t.Errorf("EnvVersionKey(%q) = %q, want %q", tt.service, got, tt.want)
			}
		})
	}
}

func TestValidateRejectsUnsafeNames(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "shell metacharacters in project",
			cfg:     &Config{Project: "a;touch /tmp/pwned", Service: "backend", Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: true,
		},
		{
			name:    "command substitution in project",
			cfg:     &Config{Project: "$(id)", Service: "backend", Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: true,
		},
		{
			name:    "empty service in monorepo list",
			cfg:     &Config{Project: "myapp", Services: []string{"api", "", "web"}, Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: true,
		},
		{
			name:    "path traversal in service name",
			cfg:     &Config{Project: "myapp", Services: []string{"api/../etc"}, Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: true,
		},
		{
			name:    "hyphens and dots stay legal",
			cfg:     &Config{Project: "my.app", Services: []string{"my-api", "web"}, Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: false,
		},
		{
			name:    "plain single service stays legal",
			cfg:     &Config{Project: "myapp", Service: "backend", Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: false,
		},
		{
			name:    "injection via registry",
			cfg:     &Config{Project: "myapp", Service: "backend", Registry: "reg.example.com;id", Region: "us-east-2"},
			wantErr: true,
		},
		{
			name:    "injection via region",
			cfg:     &Config{Project: "myapp", Service: "backend", Registry: "reg.example.com", Region: "us-east-2 $(id)"},
			wantErr: true,
		},
		{
			name:    "real ECR registry stays legal",
			cfg:     &Config{Project: "myapp", Service: "backend", Registry: "123456789.dkr.ecr.us-east-2.amazonaws.com", Region: "us-east-2"},
			wantErr: false,
		},
		{
			name:    "registry with an explicit port stays legal",
			cfg:     &Config{Project: "myapp", Service: "backend", Registry: "reg.example.com:5000", Region: "us-east-2"},
			wantErr: false,
		},
		{
			name:    "services colliding on one compose variable",
			cfg:     &Config{Project: "myapp", Services: []string{"my-api", "my.api"}, Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected an error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}
