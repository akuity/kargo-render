package main

import (
	"fmt"

	"github.com/akuityio/bookkeeper"
	libOS "github.com/akuityio/bookkeeper/internal/os"
)

func request() (bookkeeper.RenderRequest, error) {
	req := bookkeeper.RenderRequest{
		RepoCreds: bookkeeper.RepoCredentials{
			Username: "git",
		},
		Images: libOS.GetStringSliceFromEnvVar("INPUT_IMAGES", nil),
	}
	repo, err := libOS.GetRequiredEnvVar("GITHUB_REPOSITORY")
	if err != nil {
		return req, err
	}
	req.RepoURL = fmt.Sprintf("https://github.com/%s", repo)
	if req.RepoCreds.Password, err =
		libOS.GetRequiredEnvVar("INPUT_PERSONALACCESSTOKEN"); err != nil {
		return req, err
	}
	if req.Commit, err = libOS.GetRequiredEnvVar("INPUT_COMMITSHA"); err != nil {
		return req, err
	}
	if req.TargetBranch, err =
		libOS.GetRequiredEnvVar("INPUT_TARGETBRANCH"); err != nil {
		return req, err
	}
	req.OpenPR, err = libOS.GetBoolFromEnvVar("INPUT_OPENPR", false)
	return req, err
}
