// Command rubric is a cat(1) clone with wings — a Go port of bat with syntax
// highlighting, git integration, line numbers, paging and more.
//
// This root entry point exists so `go install goforge.dev/rubric@latest` works.
// The canonical project lives under projects/rubric (goforge Polylith layout);
// both share the same bases/cli entry point.
package main

import (
	"os"

	"goforge.dev/rubric/bases/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
