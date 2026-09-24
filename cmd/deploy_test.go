package cmd

import "testing"

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
