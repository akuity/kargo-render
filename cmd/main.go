package main

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/akuity/bookkeeper/cmd/action"
	"github.com/akuity/bookkeeper/cmd/cli"
)

const binaryNameEnvVar = "BOOKKEEPER_BINARY_NAME"

func main() {
	binaryName := filepath.Base(os.Args[0])
	if val := os.Getenv(binaryNameEnvVar); val != "" {
		binaryName = val
	}

	switch binaryName {
	case "bookkeeper":
		cli.Run()
	case "bookkeeper-action":
		action.Run()
	default:
		log.Fatalf("unrecognized component name %q", binaryName)
	}
}
