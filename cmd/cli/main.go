package main

import (
	"os"

	"github.com/opendownload/opendownload/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
