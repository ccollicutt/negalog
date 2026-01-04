package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set via ldflags at build time.
var Version = "dev"

// NewVersionCommand creates the version command.
func NewVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  "Print the version of NegaLog.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("negalog %s\n", Version)
		},
	}
}
