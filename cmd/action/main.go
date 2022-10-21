package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/akuityio/bookkeeper"
	"github.com/akuityio/bookkeeper/internal/version"
)

func main() {
	version := version.GetVersion()

	if len(os.Args) > 1 && os.Args[1] == "version" {
		versionBytes, err := json.MarshalIndent(version, "", "  ")
		if err != nil {
			logger.Fatal(err)
		}
		fmt.Println(string(versionBytes))
		return
	}

	logger.WithFields(log.Fields{
		"version": version.Version,
		"commit":  version.GitCommit,
	}).Info("Starting Bookkeeper Action")

	req, err := request()
	if err != nil {
		logger.Fatal(err)
	}

	res, err := bookkeeper.NewService().RenderConfig(context.Background(), req)
	if err != nil {
		logger.Fatal(err)
	}

	switch res.ActionTaken {
	case bookkeeper.ActionTakenPushedDirectly:
		fmt.Printf(
			"\nCommitted %s to branch %s\n",
			res.CommitID,
			req.TargetBranch,
		)
	case bookkeeper.ActionTakenOpenedPR:
		fmt.Printf(
			"\nOpened PR %s\n",
			res.PullRequestURL,
		)
	case bookkeeper.ActionTakenNone:
		fmt.Printf(
			"\nNewly rendered configuration does not differ from the head of "+
				"branch %s. No action was taken.\n",
			req.TargetBranch,
		)
	}
}
