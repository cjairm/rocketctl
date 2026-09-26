package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestEveryCommandIsDocumented keeps --help useful: every command, including
// subcommands, needs a summary, a description and at least one example.
func TestEveryCommandIsDocumented(t *testing.T) {
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		// cobra's own help and completion commands document themselves.
		if c.Name() == "help" || c.Name() == "completion" {
			return
		}
		t.Run(c.CommandPath(), func(t *testing.T) {
			if c.Short == "" {
				t.Error("missing Short")
			}
			if c.Long == "" {
				t.Error("missing Long")
			}
			if c.Example == "" {
				t.Error("missing Example")
			}
		})
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
}
