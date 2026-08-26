package main

import (
	"fmt"
	"os"

	"github.com/franciscoperez/apuntes-cli/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stdin); err != nil {
		fmt.Fprintln(os.Stderr, "apuntes:", err)
		os.Exit(1)
	}
}
