package cmd

import (
	"strings"
	"testing"

	"github.com/cjairm/rocketctl/internal/config"
)

func TestRestoreArgs(t *testing.T) {
	single := &config.Config{Project: "myapp", Service: "api"}
	mono := &config.Config{Project: "myapp", Services: []string{"api", "web"}}
	tests := []struct {
		name        string
		cfg         *config.Config
		args        []string
		wantService string
		wantFile    string
		wantErr     string
	}{
		{"single-service, nothing given", single, nil, "api", "", ""},
		{"single-service, its own name", single, []string{"api"}, "api", "", ""},
		{"single-service, just a file", single, []string{"b.tar.gz"}, "api", "b.tar.gz", ""},
		{"single-service, name and file", single, []string{"api", "b.tar.gz"}, "api", "b.tar.gz", ""},
		{"single-service, another name and a file", single, []string{"web", "b.tar.gz"}, "", "", "Available services: [api]"},
		{"monorepo requires a service", mono, nil, "", "", "Available services: [api web]"},
		{"monorepo, a service", mono, []string{"web"}, "web", "", ""},
		{"monorepo, service and file", mono, []string{"web", "b.tar.gz"}, "web", "b.tar.gz", ""},
		{"monorepo, a file is not a service", mono, []string{"b.tar.gz"}, "", "", "Available services: [api web]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, file, err := restoreArgs(tt.cfg, tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("restoreArgs() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("restoreArgs() error = %v", err)
			}
			if service != tt.wantService || file != tt.wantFile {
				t.Errorf("restoreArgs() = %q, %q; want %q, %q", service, file, tt.wantService, tt.wantFile)
			}
		})
	}
}
