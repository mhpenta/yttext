package main

import (
	"os"

	"github.com/mhpenta/yttext/cli"
)

func main() {
	app := cli.NewCLI()
	os.Exit(app.Run())
}
