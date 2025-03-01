package main

import (
	"os"

	"github.com/mhpenta/yttext/pkg/cli"
)

func main() {
	app := cli.NewCLI()
	os.Exit(app.Run())
}
