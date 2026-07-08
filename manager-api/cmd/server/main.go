// Command jabali-sounder is the entry point for the Jabali Sounder control
// plane. It serves the HTTP API (`jabali-sounder serve`) and administrative
// CLI commands (`jabali-sounder migrate up`, …).
package main

import (
	"fmt"
	"os"
)

const defaultConfigPath = "/etc/jabali-sounder/config.toml"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
