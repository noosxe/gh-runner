// Command supervisor is the AIO Supervisor daemon entrypoint.
//
// It manages dynamic pools of ephemeral GitHub/Gitea/Forgejo runner
// containers and serves the embedded web control interface.
//
// NOTE: The Cobra CLI structure (daemon, import, reset-password, backup
// subcommands) is introduced in RUN-6; this scaffold only establishes a
// minimal, buildable entrypoint.
package main

import (
	"fmt"
	"os"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	fmt.Printf("gh-runner supervisor %s (scaffold)\n", version)
	os.Exit(0)
}
