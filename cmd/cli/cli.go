package cli

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

func Run() {
	// These two lines are required to suppress log output from the Argo CD repo
	// server, which Kargo Render uses as a library.
	//
	// Without suppressing this, requests for machine-readable output (e.g. JSON)
	// will be polluted with log output and attempts to parse it (e.g. by Kargo)
	// will fail.
	//
	// This does NOT interfere with using the Kargo Render CLI's own --debug flag,
	// however, choosing that will, once again, result in some amount of
	// unparsable output.
	os.Setenv("ARGOCD_LOG_LEVEL", "PANIC")
	log.SetLevel(log.PanicLevel)

	cmd, err := newRootCommand()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err = cmd.Execute(); err != nil {
		// Cobra will display the error for us. No need to do it ourselves.
		os.Exit(1)
	}
}
