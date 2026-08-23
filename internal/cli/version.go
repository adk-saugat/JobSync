package cli

import (
	"fmt"
	"runtime"

	"github.com/saugatadhikari/jobSync/internal/config"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func runVersion(args []string) error {
	_ = args
	fmt.Printf("jobsync %s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
	fmt.Printf("gemini default: %s\n", config.DefaultGeminiModel)
	return nil
}
