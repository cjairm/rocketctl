package cmd

import (
	"errors"
	"fmt"
	"testing"

	"github.com/cjairm/rocketctl/internal/migrate"
)

func TestExitCode(t *testing.T) {
	appFailure := &migrate.ExitError{Code: 3, Command: "bin/migrate --apply", LogPath: "x.log"}
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"success", nil, 0},
		{"ordinary error", errors.New("boom"), 1},
		{"app exit code is passed through", appFailure, 3},
		{"wrapped app exit code is passed through", fmt.Errorf("migrate: %w", appFailure), 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCode(tt.err); got != tt.want {
				t.Errorf("exitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}
