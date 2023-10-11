package main

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/akuity/kargo-render/cmd/action"
	"github.com/akuity/kargo-render/cmd/cli"
)

// false positive for "G101: Potential hardcoded credentials"
const binaryNameEnvVar = "KARGO_RENDER_BINARY_NAME" // nolint: gosec

func main() {
	binaryName := filepath.Base(os.Args[0])
	if val := os.Getenv(binaryNameEnvVar); val != "" {
		binaryName = val
	}

	switch binaryName {
	case "kargo-render":
		cli.Run()
	case "kargo-render-action":
		action.Run()
	default:
		log.Fatalf("unrecognized component name %q", binaryName)
	}
}
