//go:build !relay

package main

import (
	"fmt"
	"os"
)

func runRelay(args []string) {
	fmt.Fprintln(os.Stderr, "Error: 'relay' mode not compiled into this binary.")
	fmt.Fprintln(os.Stderr, "Build with -tags relay to enable relay mode.")
	os.Exit(1)
}