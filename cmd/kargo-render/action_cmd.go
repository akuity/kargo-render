package main

import (
	"context"
	"fmt"
	"io"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	render "github.com/akuity/kargo-render"
	libLog "github.com/akuity/kargo-render/internal/log"
	libOS "github.com/akuity/kargo-render/internal/os"
	"github.com/akuity/kargo-render/internal/version"
)

type actionOptions struct {
	logger *log.Logger
}

func newActionCommand() *cobra.Command {
	cmdOpts := &actionOptions{
		logger: libLog.LoggerOrDie(),
	}

	return &cobra.Command{
		Use:    "action",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmdOpts.run(cmd.Context(), cmd.OutOrStdout())
		},
	}
}

// run performs manifest rendering in a GitHub Actions-compatible manner.
func (o *actionOptions) run(_ context.Context, out io.Writer) error {
	logger := o.logger

	ver := version.GetVersion()
	logger.WithFields(log.Fields{
		"version": ver.Version,
		"commit":  ver.GitCommit,
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
		fmt.Fprintln(
			out,
			"\nThis request would not change any state. No action was taken.",
		)
	case render.ActionTakenOpenedPR:
		fmt.Fprintf(
			out,
			"\nOpened PR %s\n",
			res.PullRequestURL,
		)
	case render.ActionTakenPushedDirectly:
		fmt.Fprintf(
			out,
			"\nCommitted %s to branch %s\n",
			res.CommitID,
			req.TargetBranch,
		)
	case render.ActionTakenUpdatedPR:
		fmt.Fprintf(
			out,
			"\nUpdated an existing PR to %s\n",
			req.TargetBranch,
		)
	}

	return nil
}

func request() (*render.Request, error) {
	req := &render.Request{
		RepoCreds: render.RepoCredentials{
			Username: "git",
		},
		Images: libOS.GetStringSliceFromEnvVar("INPUT_IMAGES", nil),
	}
	repo, err := libOS.GetRequiredEnvVar("GITHUB_REPOSITORY")
	if err != nil {
		return nil, err
	}
	req.RepoURL = fmt.Sprintf("https://github.com/%s", repo)
	if req.RepoCreds.Password, err =
		libOS.GetRequiredEnvVar("INPUT_PERSONALACCESSTOKEN"); err != nil {
		return nil, err
	}
	if req.Ref, err = libOS.GetRequiredEnvVar("GITHUB_SHA"); err != nil {
		return nil, err
	}
	req.TargetBranch, err = libOS.GetRequiredEnvVar("INPUT_TARGETBRANCH")
	return req, err
}
