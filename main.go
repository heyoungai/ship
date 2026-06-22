package main

import (
	"os"
	"github.com/heyoungai/ship/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
