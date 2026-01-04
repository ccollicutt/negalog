package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"negalog/pkg/config"
	"negalog/pkg/parser"
)

// NewValidateCommand creates the validate command.
func NewValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <config-file>",
		Short: "Validate a configuration file",
		Long: `Validate a NegaLog configuration file without running analysis.

Checks:
  - YAML syntax
  - Required fields
  - Regex pattern validity
  - Rule type-specific requirements
  - Log source file existence (warning only)`,
		Args: cobra.ExactArgs(1),
		RunE: runValidate,
	}
}

func runValidate(cmd *cobra.Command, args []string) error {
	configPath := args[0]
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	fmt.Printf("Validating %s...\n", configPath)

	// Load and validate config
	cfg, err := config.Load(ctx, configPath)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Report what we found
	fmt.Printf("\nConfiguration valid!\n")
	fmt.Printf("  Log sources: %d pattern(s)\n", len(cfg.LogSources))
	fmt.Printf("  Rules:       %d\n", len(cfg.Rules))

	// List rules
	fmt.Printf("\nRules:\n")
	for i, rule := range cfg.Rules {
		fmt.Printf("  %d. [%s] %s\n", i+1, rule.Type, rule.Name)
		if rule.Description != "" {
			fmt.Printf("     %s\n", rule.Description)
		}
	}

	// Check if log sources exist (warnings only)
	files, err := parser.ExpandGlobs(cfg.LogSources)
	if err != nil {
		fmt.Printf("\nWarning: Error expanding log source patterns: %v\n", err)
	} else if len(files) == 0 {
		fmt.Printf("\nWarning: No files match log source patterns\n")
	} else {
		fmt.Printf("\nLog files matched: %d\n", len(files))
		for _, f := range files {
			fmt.Printf("  - %s\n", f)
		}
	}

	return nil
}
