package main

import (
	"fmt"
	"os"

	"github.com/opendownload/opendownload/internal/nativehost"
)

func main() {
	if err := nativehost.Run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
