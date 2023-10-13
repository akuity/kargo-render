package action

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	render "github.com/akuity/kargo-render"
	"github.com/akuity/kargo-render/internal/version"
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
	}).Info("Starting Kargo Render Action")

	req, err := request()
	if err != nil {
		logger.Fatal(err)
	}

	res, err := render.NewService(
		&render.ServiceOptions{
			LogLevel: render.LogLevel(logger.Level),
		},
	).RenderManifests(context.Background(), req)
	if err != nil {
		logger.Fatal(err)
	}

	switch res.ActionTaken {
	case render.ActionTakenNone:
		fmt.Println(
			"\nThis request would not change any state. No action was taken.",
		)
	case render.ActionTakenOpenedPR:
		fmt.Printf(
			"\nOpened PR %s\n",
			res.PullRequestURL,
		)
	case render.ActionTakenPushedDirectly:
		fmt.Printf(
			"\nCommitted %s to branch %s\n",
			res.CommitID,
			req.TargetBranch,
		)
	case render.ActionTakenUpdatedPR:
		fmt.Printf(
			"\nUpdated an existing PR to %s\n",
			req.TargetBranch,
		)
	}
}
