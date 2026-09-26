package cmd

import (
	"reflect"
	"testing"

	"github.com/cjairm/rocketctl/internal/config"
)

func TestVersionEnvAssignments(t *testing.T) {
	tests := []struct {
		name     string
		versions map[string]string
		want     string
	}{
		{
			name:     "single service",
			versions: map[string]string{"backend": "0.1.4"},
			want:     "BACKEND_VERSION='0.1.4' ",
		},
		{
			name:     "monorepo is sorted for a stable command",
			versions: map[string]string{"web": "2.0.1", "api": "0.1.4"},
			want:     "API_VERSION='0.1.4' WEB_VERSION='2.0.1' ",
		},
		{
			name:     "hyphenated service becomes a shell identifier",
			versions: map[string]string{"my-api": "1.0.0"},
			want:     "MY_API_VERSION='1.0.0' ",
		},
		{
			name:     "no services yields an empty prefix",
			versions: map[string]string{},
			want:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := versionEnvAssignments(tt.versions); got != tt.want {
				t.Errorf("versionEnvAssignments() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPinnedImageLines(t *testing.T) {
	cfg := &config.Config{
		Project:  "myapp",
		Registry: "reg.example.com",
		Services: []string{"api", "web"},
	}
	versions := map[string]string{"api": "1.4.0", "web": "2.0.1"}

	tags := builtImageTags(cfg, versions)
	wantTags := map[string]string{
		"reg.example.com/myapp_api": "1.4.0",
		"reg.example.com/myapp_web": "2.0.1",
	}
	if !reflect.DeepEqual(tags, wantTags) {
		t.Errorf("builtImageTags() = %v, want %v", tags, wantTags)
	}

	updated := map[string][]string{
		"reg.example.com/myapp_api": {"myapp-api", "myapp-worker"},
	}
	want := []string{
		"myapp_api:1.4.0 → myapp-api, myapp-worker",
		"myapp_web:2.0.1 → ⚠️  not pinned: no service in docker-compose.prod.yml uses reg.example.com/myapp_web (YAML anchors and merge keys are not followed)",
	}
	if got := pinnedImageLines(cfg, versions, updated); !reflect.DeepEqual(got, want) {
		t.Errorf("pinnedImageLines() = %q, want %q", got, want)
	}
}

func TestRemoteCleanupCommand(t *testing.T) {
	withVolumes := "docker container prune -f && docker image prune -a -f && docker volume prune -f && docker network prune -f && docker system prune -a -f"
	if got := remoteCleanupCommand(true); got != withVolumes {
		t.Errorf("remoteCleanupCommand(true) = %q, want %q", got, withVolumes)
	}
	withoutVolumes := "docker container prune -f && docker image prune -a -f && docker network prune -f && docker system prune -a -f"
	if got := remoteCleanupCommand(false); got != withoutVolumes {
		t.Errorf("remoteCleanupCommand(false) = %q, want %q", got, withoutVolumes)
	}
}

func TestVolumePruneIsSafe(t *testing.T) {
	tests := []struct {
		serverVersion string
		want          bool
	}{
		{"27.4.0", true},
		{"23.0.0", true},
		{"23.0.0\n", true},
		{"22.06.0-beta.0", false},
		{"20.10.24", false},
		{"", false},
		{"unknown", false},
	}
	for _, tt := range tests {
		if got := volumePruneIsSafe(tt.serverVersion); got != tt.want {
			t.Errorf("volumePruneIsSafe(%q) = %v, want %v", tt.serverVersion, got, tt.want)
		}
	}
}

func TestStaleLocalImages(t *testing.T) {
	cfg := &config.Config{Project: "myapp", Registry: "reg.example.com", Service: "api"}
	images := []string{
		"myapp_api:1.3.0",
		"myapp_api:1.2.0",
		"myapp_api:1.1.0",
		"reg.example.com/myapp_api:1.3.0",
		"reg.example.com/myapp_api:1.2.0",
		"reg.example.com/myapp_api:1.0.0",
		"myapp_api:<none>",
	}
	want := []string{"myapp_api:1.1.0", "reg.example.com/myapp_api:1.0.0"}
	if got := staleLocalImages(cfg, "api", "1.3.0", images); !reflect.DeepEqual(got, want) {
		t.Errorf("staleLocalImages() = %v, want %v", got, want)
	}
}
