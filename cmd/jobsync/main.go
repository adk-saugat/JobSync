package main

import (
	"fmt"
	"os"

	"github.com/saugatadhikari/jobSync/internal/cli"
	"github.com/saugatadhikari/jobSync/internal/config"
)

func main() {
	config.LoadDotEnv()
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
