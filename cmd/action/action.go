package action

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/akuityio/bookkeeper"
	"github.com/akuityio/bookkeeper/internal/version"
)

func Run() {
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

	res, err := bookkeeper.NewService(
		&bookkeeper.ServiceOptions{
			LogLevel: bookkeeper.LogLevel(logger.Level),
		},
	).RenderManifests(context.Background(), req)
	if err != nil {
		logger.Fatal(err)
	}

	switch res.ActionTaken {
	case bookkeeper.ActionTakenNone:
		fmt.Println(
			"\nThis request would not change any state. No action was taken.",
		)
	case bookkeeper.ActionTakenOpenedPR:
		fmt.Printf(
			"\nOpened PR %s\n",
			res.PullRequestURL,
		)
	case bookkeeper.ActionTakenPushedDirectly:
		fmt.Printf(
			"\nCommitted %s to branch %s\n",
			res.CommitID,
			req.TargetBranch,
		)
	case bookkeeper.ActionTakenUpdatedPR:
		fmt.Printf(
			"\nUpdated an existing PR to %s\n",
			req.TargetBranch,
		)
	}
}
