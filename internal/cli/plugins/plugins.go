// Package plugins provides exec-based plugin support for negalog.
// Plugins are separate binaries named negalog-<command> that are discovered
// and executed when an unknown command is invoked.
//
// This follows the same pattern used by kubectl and git for plugins.
package plugins

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// KnownPlugins lists plugins that have official implementations available.
// These get special error messages directing users where to obtain them.
var KnownPlugins = map[string]string{
	"watch": "Continuous log monitoring. Available at: https://collicutt.net/software/negalog/#negalog-watch",
}

// ErrPluginNotFound is returned when no plugin binary can be located.
var ErrPluginNotFound = errors.New("plugin not found")

// FindPlugin searches for a plugin binary named negalog-<command>.
// It searches in the following locations in order:
//  1. Same directory as the negalog binary
//  2. ~/.negalog/plugins/
//  3. Anywhere in PATH
//
// Returns the full path to the plugin binary if found.
func FindPlugin(command string) (string, error) {
	pluginName := "negalog-" + command

	// 1. Check same directory as negalog binary
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		candidate := filepath.Join(execDir, pluginName)
		if isExecutable(candidate) {
			return candidate, nil
		}
	}

	// 2. Check ~/.negalog/plugins/
	if homeDir, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(homeDir, ".negalog", "plugins", pluginName)
		if isExecutable(candidate) {
			return candidate, nil
		}
	}

	// 3. Check PATH
	if path, err := exec.LookPath(pluginName); err == nil {
		return path, nil
	}

	return "", ErrPluginNotFound
}

// Execute runs a plugin with the given arguments.
// It connects stdin, stdout, and stderr to the plugin process
// and returns the plugin's exit code.
func Execute(pluginPath string, args []string) int {
	cmd := exec.Command(pluginPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		// Extract exit code from error if available
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		// If we can't get the exit code, return 1
		fmt.Fprintf(os.Stderr, "Error executing plugin: %v\n", err)
		return 1
	}

	return 0
}

// FormatNotFoundError returns a helpful error message when a plugin is not found.
// If the command is a known plugin, includes information about where to get it.
func FormatNotFoundError(command string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("unknown command %q for \"negalog\"\n", command))

	// Check if this is a known plugin
	if info, ok := KnownPlugins[command]; ok {
		sb.WriteString(fmt.Sprintf("\n%q is available as a plugin.\n", command))
		sb.WriteString(info)
		sb.WriteString("\n\nInstall the plugin binary as one of:\n")
	} else {
		sb.WriteString("\nIf this is a plugin, install the binary as one of:\n")
	}

	// Show installation locations
	sb.WriteString(fmt.Sprintf("  - negalog-%s in the same directory as negalog\n", command))
	sb.WriteString(fmt.Sprintf("  - ~/.negalog/plugins/negalog-%s\n", command))
	sb.WriteString(fmt.Sprintf("  - negalog-%s anywhere in your PATH\n", command))

	sb.WriteString("\nRun 'negalog --help' for usage.")

	return sb.String()
}

// isExecutable checks if a file exists and is executable.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// On Unix, check executable bit
	// On Windows, just check if file exists (executable bit doesn't apply)
	if info.Mode().IsRegular() {
		// Check if any execute bit is set
		return info.Mode()&0111 != 0
	}

	return false
}
