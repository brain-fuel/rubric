// Command rubric is a cat(1) clone with wings — a Go port of bat with syntax
// highlighting, git integration, line numbers, paging and more.
package main

import (
	"os"

	"goforge.dev/rubric/bases/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
