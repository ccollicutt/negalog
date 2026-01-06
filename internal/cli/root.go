// Package cli provides the command-line interface for NegaLog.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ccollicutt/negalog/internal/cli/commands"
	"github.com/ccollicutt/negalog/internal/cli/plugins"
)

// Execute runs the root command and returns the exit code.
func Execute() int {
	rootCmd := NewRootCommand()

	// Check if the first argument might be a plugin command
	if len(os.Args) > 1 {
		potentialCommand := os.Args[1]
		// Skip flags (start with -)
		if len(potentialCommand) > 0 && potentialCommand[0] != '-' {
			// Check if it's a known built-in command
			if !isBuiltinCommand(rootCmd, potentialCommand) {
				// Try to find and execute a plugin
				if pluginPath, err := plugins.FindPlugin(potentialCommand); err == nil {
					// Plugin found - execute it with remaining args
					return plugins.Execute(pluginPath, os.Args[2:])
				}
				// Plugin not found - will fall through to Cobra which will show error
			}
		}
	}

	if err := rootCmd.Execute(); err != nil {
		// Check if this was an unknown command that could be a plugin
		if len(os.Args) > 1 {
			potentialCommand := os.Args[1]
			if len(potentialCommand) > 0 && potentialCommand[0] != '-' {
				if !isBuiltinCommand(rootCmd, potentialCommand) {
					// Show helpful plugin error message
					_, _ = fmt.Fprintln(os.Stderr, plugins.FormatNotFoundError(potentialCommand))
					return 2
				}
			}
		}
		// Print error to stderr (SilenceErrors prevents Cobra from doing this)
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2 // Configuration or runtime error
	}
	return commands.ExitCode
}

// isBuiltinCommand checks if a command name is a built-in cobra command.
func isBuiltinCommand(rootCmd *cobra.Command, name string) bool {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == name || cmd.HasAlias(name) {
			return true
		}
	}
	// Also check for special commands like help and completion
	return name == "help" || name == "completion"
}

// NewRootCommand creates the root cobra command.
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "negalog",
		Short: "Detect missing logs in your log files",
		Long: `NegaLog is a batch log analysis tool that detects the absence of expected logs.

It identifies:
  - Sequence gaps (start without matching end)
  - Periodic absence (missing heartbeats)
  - Conditional absence (trigger without consequence)

Define what logs SHOULD exist, and NegaLog reports what's missing.

PLUGINS:
  NegaLog supports plugins for extended functionality. Plugins are standalone
  binaries named negalog-<command> that are automatically discovered and invoked.

  Plugin locations (searched in order):
    1. Same directory as the negalog binary
    2. ~/.negalog/plugins/
    3. Anywhere in PATH

  Available plugins:
    watch    Continuous log monitoring (https://collicutt.net/software/negalog/#negalog-watch)`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Add subcommands
	rootCmd.AddCommand(commands.NewAnalyzeCommand())
	rootCmd.AddCommand(commands.NewDetectCommand())
	rootCmd.AddCommand(commands.NewDiagnoseCommand())
	rootCmd.AddCommand(commands.NewValidateCommand())
	rootCmd.AddCommand(commands.NewVersionCommand())

	return rootCmd
}
