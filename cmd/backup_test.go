package cmd

import (
	"path/filepath"
	"testing"
)

func TestBackupDir(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{"defaults outside any repo, per project", "", filepath.Join("/home/me", ".rocketctl", "backups", "myapp")},
		{"--out overrides the folder", "/mnt/safe", "/mnt/safe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := backupDir("/home/me", "myapp", tt.out); got != tt.want {
				t.Errorf("backupDir() = %q, want %q", got, tt.want)
			}
		})
	}
}
