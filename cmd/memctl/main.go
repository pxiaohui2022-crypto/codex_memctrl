package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pxiaohui2022-crypto/codex_memctrl/internal/cli"
)

var version = "dev"

func main() {
	app := cli.New(version, os.Stdout, os.Stderr)
	if err := app.Run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
