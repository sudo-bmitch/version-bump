// Package main defines the version-bump command
package main

import (
	"fmt"
	"os"
)

func main() {
	// execute cobra cli
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
