// ABOUTME: Entry point for the pulse binary.
// ABOUTME: Declares build-injected version variables and runs the root command.
package main

import (
	"fmt"
	"os"
)

// Build-injected via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	rootCmd.Version = fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
