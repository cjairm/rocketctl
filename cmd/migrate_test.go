package cmd

import (
	"strings"
	"testing"

	"github.com/cjairm/rocketctl/internal/config"
)

func TestRunningService(t *testing.T) {
	single := &config.Config{Project: "myapp", Service: "api"}
	mono := &config.Config{Project: "myapp", Services: []string{"api", "web"}}
	tests := []struct {
		name    string
		cfg     *config.Config
		args    []string
		want    string
		wantErr string
	}{
		{"single-service infers the service", single, nil, "api", ""},
		{"single-service accepts its own name", single, []string{"api"}, "api", ""},
		// Acting on a running service is too consequential to ignore a name that does not match.
		{
			"single-service rejects another name",
			single,
			[]string{"web"},
			"",
			"Available services: [api]",
		},
		{"monorepo requires a service", mono, nil, "", "Available services: [api web]"},
		{"monorepo takes the named service", mono, []string{"web"}, "web", ""},
		{
			"monorepo rejects an unknown service",
			mono,
			[]string{"db"},
			"",
			"Available services: [api web]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runningService(tt.cfg, tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("runningService() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("runningService() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("runningService() = %q, want %q", got, tt.want)
			}
		})
	}
}
