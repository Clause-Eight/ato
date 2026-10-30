package main

import (
	"os"

	"github.com/Clause-Eight/ato/v2/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
