//go:build !server

package main

import (
	"fmt"
	"os"
)

func runServe(args []string) {
	fmt.Fprintln(os.Stderr, "Error: 'serve' mode not compiled into this binary.")
	fmt.Fprintln(os.Stderr, "Build with -tags server to enable server mode.")
	os.Exit(1)
}