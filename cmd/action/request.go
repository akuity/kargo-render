package action

import (
	"fmt"

	render "github.com/akuity/kargo-render"
	libOS "github.com/akuity/kargo-render/internal/os"
)

func request() (render.Request, error) {
	req := render.Request{
		RepoCreds: render.RepoCredentials{
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
	if req.Ref, err = libOS.GetRequiredEnvVar("GITHUB_SHA"); err != nil {
		return req, err
	}
	req.TargetBranch, err = libOS.GetRequiredEnvVar("INPUT_TARGETBRANCH")
	return req, err
}
