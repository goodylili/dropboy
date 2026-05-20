package main

import (
	"fmt"
	"os"

	"github.com/goodylili/dropboy/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "dropboy: %v\n", err)
		os.Exit(1)
	}
}
