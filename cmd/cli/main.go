// NegaLog - Missing Log Detection Tool
//
// NegaLog is a batch log analysis tool that detects the absence of expected logs.
// Define what logs SHOULD exist, and NegaLog reports what's missing.
package main

import (
	"os"

	"negalog/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
