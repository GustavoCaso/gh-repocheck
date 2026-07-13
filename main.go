package main

import (
	"os"

	"github.com/GustavoCaso/gh-repocheck/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, os.Stdin))
}
