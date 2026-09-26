package compose

import (
	"reflect"
	"strings"
	"testing"
)

const repo = "123.dkr.ecr.us-east-2.amazonaws.com/myapp_api"

func TestPinImages(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		tags        map[string]string
		want        string
		wantUpdated map[string][]string
	}{
		{
			name: "services sharing a built image all get the tag, sorted, outside images are untouched",
			in: `services:
  caddy:
    image: caddy:2-alpine
  myapp-worker:
    image: ` + repo + `:1.3.0
    command: ["run"]
  myapp-api:
    image: ` + repo + `:1.3.0
`,
			tags: map[string]string{repo: "1.4.0"},
			want: `services:
  caddy:
    image: caddy:2-alpine
  myapp-worker:
    image: ` + repo + `:1.4.0
    command: ["run"]
  myapp-api:
    image: ` + repo + `:1.4.0
`,
			wantUpdated: map[string][]string{
				repo: {"myapp-api", "myapp-worker"},
			},
		},
		{
			name: "services sharing one line are each rewritten once",
			in: `services: {a: {image: reg/x:1}, b: {image: reg/x:1}}
`,
			tags: map[string]string{"reg/x": "1.4.0"},
			want: `services: {a: {image: reg/x:1.4.0}, b: {image: reg/x:1.4.0}}
`,
			wantUpdated: map[string][]string{"reg/x": {"a", "b"}},
		},
		{
			name: "columns count characters, not bytes",
			in: `services: {ñandú: {image: reg/x:1}, b: {image: reg/x:1}}
`,
			tags: map[string]string{"reg/x": "1.4.0"},
			want: `services: {ñandú: {image: reg/x:1.4.0}, b: {image: reg/x:1.4.0}}
`,
			wantUpdated: map[string][]string{"reg/x": {"b", "ñandú"}},
		},
		{
			name: "one service per image changes only its image line",
			in: `# production stack
services:
  myapp-api:
    image: reg.example.com/myapp_api:${API_VERSION:-latest}  # pinned by deploy
    restart: unless-stopped
  myapp-web:
    image: "reg.example.com/myapp_web:0.9.0"
`,
			tags: map[string]string{
				"reg.example.com/myapp_api": "0.1.4",
				"reg.example.com/myapp_web": "2.0.1",
			},
			want: `# production stack
services:
  myapp-api:
    image: reg.example.com/myapp_api:0.1.4  # pinned by deploy
    restart: unless-stopped
  myapp-web:
    image: "reg.example.com/myapp_web:2.0.1"
`,
			wantUpdated: map[string][]string{
				"reg.example.com/myapp_api": {"myapp-api"},
				"reg.example.com/myapp_web": {"myapp-web"},
			},
		},
		{
			name: "matches the whole repository, not a prefix",
			in: `services:
  other:
    image: ` + repo + `_v2:1.0.0
`,
			tags: map[string]string{repo: "1.4.0"},
			want: `services:
  other:
    image: ` + repo + `_v2:1.0.0
`,
			wantUpdated: map[string][]string{},
		},
		{
			name: "registry with a port and an untagged image",
			in: `services:
  api:
    image: localhost:5000/myapp_api
`,
			tags: map[string]string{"localhost:5000/myapp_api": "0.2.0"},
			want: `services:
  api:
    image: localhost:5000/myapp_api:0.2.0
`,
			wantUpdated: map[string][]string{
				"localhost:5000/myapp_api": {"api"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, updated, err := PinImages([]byte(tt.in), tt.tags)
			if err != nil {
				t.Fatalf("PinImages() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("PinImages() =\n%s\nwant\n%s", got, tt.want)
			}
			if !reflect.DeepEqual(updated, tt.wantUpdated) {
				t.Errorf("PinImages() updated = %v, want %v", updated, tt.wantUpdated)
			}
		})
	}
}

func TestPinImagesRejectsInvalidYAML(t *testing.T) {
	_, _, err := PinImages([]byte("services: [unclosed"), map[string]string{repo: "1.4.0"})
	if err == nil || !strings.Contains(err.Error(), "docker-compose.prod.yml") {
		t.Errorf("PinImages() error = %v, want one naming docker-compose.prod.yml", err)
	}
}

func TestPinImagesRejectsImageSpanningLines(t *testing.T) {
	in := "services:\n  api:\n    image: >-\n      " + repo + ":1.3.0\n"
	_, _, err := PinImages([]byte(in), map[string]string{repo: "1.4.0"})
	if err == nil || !strings.Contains(err.Error(), "write it on one line") {
		t.Errorf("PinImages() error = %v, want one asking for a single line", err)
	}
}
