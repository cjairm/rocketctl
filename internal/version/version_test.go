package version

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCalculateBump(t *testing.T) {
	tests := []struct {
		name    string
		current string
		bump    string
		want    string
		wantErr bool
	}{
		{
			name:    "patch increments the patch component",
			current: "0.1.0",
			bump:    "patch",
			want:    "0.1.1",
		},
		{
			name:    "minor increments minor and resets patch",
			current: "0.1.3",
			bump:    "minor",
			want:    "0.2.0",
		},
		{
			name:    "major increments major and resets minor and patch",
			current: "1.2.3",
			bump:    "major",
			want:    "2.0.0",
		},
		{
			name:    "multi digit components are handled",
			current: "10.20.30",
			bump:    "patch",
			want:    "10.20.31",
		},
		{name: "unknown bump type is rejected", current: "1.0.0", bump: "huge", wantErr: true},
		{name: "empty bump type is rejected", current: "1.0.0", bump: "", wantErr: true},
		{name: "two component version is rejected", current: "1.0", bump: "patch", wantErr: true},
		{
			name:    "prerelease suffix is rejected",
			current: "1.0.0-beta",
			bump:    "patch",
			wantErr: true,
		},
		{name: "non numeric component is rejected", current: "1.x.0", bump: "patch", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CalculateBump(tt.current, tt.bump)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("CalculateBump(%q, %q) = %q, want error", tt.current, tt.bump, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("CalculateBump(%q, %q) unexpected error: %v", tt.current, tt.bump, err)
			}
			if got != tt.want {
				t.Errorf("CalculateBump(%q, %q) = %q, want %q", tt.current, tt.bump, got, tt.want)
			}
		})
	}
}

func TestSetThenGetRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".rocket-version")

	if err := Set(path, "2.3.4"); err != nil {
		t.Fatalf("Set unexpected error: %v", err)
	}

	got, err := Get(path)
	if err != nil {
		t.Fatalf("Get unexpected error: %v", err)
	}
	if got != "2.3.4" {
		t.Errorf("Get = %q, want %q", got, "2.3.4")
	}
}

func TestGetInitialisesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".rocket-version")

	got, err := Get(path)
	if err != nil {
		t.Fatalf("Get unexpected error: %v", err)
	}
	if got != "0.1.0" {
		t.Errorf("Get on missing file = %q, want %q", got, "0.1.0")
	}

	// Get persists the initial version as a deliberate side effect.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Get did not create %s: %v", path, err)
	}
	if string(data) != "0.1.0\n" {
		t.Errorf("file contents = %q, want %q", string(data), "0.1.0\n")
	}
}

func TestGetTrimsSurroundingWhitespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".rocket-version")
	if err := os.WriteFile(path, []byte("  1.2.3\n\n"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	got, err := Get(path)
	if err != nil {
		t.Fatalf("Get unexpected error: %v", err)
	}
	if got != "1.2.3" {
		t.Errorf("Get = %q, want %q", got, "1.2.3")
	}
}

func TestGetRejectsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".rocket-version")
	if err := os.WriteFile(path, []byte("   \n"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if _, err := Get(path); err == nil {
		t.Fatal("Get on a whitespace-only file returned nil error, want error")
	}
}

func TestGetRejectsMalformedVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".rocket-version")
	if err := os.WriteFile(path, []byte("not-a-version\n"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if _, err := Get(path); err == nil {
		t.Fatal("Get on a malformed version returned nil error, want error")
	}
}

func TestSetRejectsInvalidVersionWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".rocket-version")

	if err := Set(path, "1.0"); err == nil {
		t.Fatal("Set with invalid semver returned nil error, want error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Set created %s despite rejecting the input", path)
	}
}

func TestStale(t *testing.T) {
	tests := []struct {
		name    string
		tags    []string
		current string
		want    []string
	}{
		{
			name:    "keeps current and the one before it, oldest first",
			tags:    []string{"1.3.0", "1.0.0", "1.2.0", "1.1.0"},
			current: "1.3.0",
			want:    []string{"1.0.0", "1.1.0"},
		},
		{
			name:    "compares numerically, not as text",
			tags:    []string{"1.10.0", "1.9.0", "1.8.0"},
			current: "1.10.0",
			want:    []string{"1.8.0"},
		},
		{
			name:    "keeps tags newer than current",
			tags:    []string{"2.0.0", "1.2.0", "1.1.0", "1.0.0"},
			current: "1.2.0",
			want:    []string{"1.0.0"},
		},
		{
			name:    "leaves tags that are not X.Y.Z alone",
			tags:    []string{"latest", "v1.0.0", "1.0.0-rc1", "0.9.0", "0.8.0", "1.0.0"},
			current: "1.0.0",
			want:    []string{"0.8.0"},
		},
		{
			name:    "nothing to remove with only current and previous",
			tags:    []string{"1.2.0", "1.3.0"},
			current: "1.3.0",
			want:    nil,
		},
		{
			name:    "current missing from tags still keeps the newest older one",
			tags:    []string{"1.1.0", "1.2.0"},
			current: "1.3.0",
			want:    []string{"1.1.0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Stale(tt.tags, tt.current); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Stale(%v, %q) = %v, want %v", tt.tags, tt.current, got, tt.want)
			}
		})
	}
}
