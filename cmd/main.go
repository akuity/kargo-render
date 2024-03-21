package main

import (
	"context"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

func main() {
	// These two lines are required to suppress undesired log output from the Argo
	// CD repo server, which Kargo Render uses as a library. This does NOT
	// interfere with using the Kargo Render CLI's own --debug flag.
	if err := os.Setenv("ARGOCD_LOG_LEVEL", "PANIC"); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	log.SetLevel(log.PanicLevel)
	// This line makes all log output go to stderr, leaving stdout for actual
	// program output only. This is important for cases where machine readable
	// output (e.g. JSON) is requested.
	log.SetOutput(os.Stderr)

	if err := newRootCommand().ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}
