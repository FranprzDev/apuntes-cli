package main

import (
	"fmt"
	"os"

	"github.com/franciscoperez/apuntes-cli/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:], os.Stdout, os.Stdin); err != nil {
		fmt.Fprintln(os.Stderr, "apuntes:", err)
		os.Exit(1)
	}
}
