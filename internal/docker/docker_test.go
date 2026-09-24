package docker

import "testing"

func TestMatchesAnyCurrent(t *testing.T) {
	current := []string{
		"myapp_api:0.1.0",
		"reg.example.com/myapp_api:0.1.0",
	}
	tests := []struct {
		name  string
		image string
		want  bool
	}{
		{"bare current image is kept", "myapp_api:0.1.0", true},
		{"registry-tagged current image is kept", "reg.example.com/myapp_api:0.1.0", true},
		{"bare old image is pruned", "myapp_api:0.0.9", false},
		{"registry-tagged old image is pruned", "reg.example.com/myapp_api:0.0.9", false},
		{"unrelated image is pruned", "postgres:16", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesAnyCurrent(tt.image, current); got != tt.want {
				t.Errorf("MatchesAnyCurrent(%q) = %v, want %v", tt.image, got, tt.want)
			}
		})
	}
}
