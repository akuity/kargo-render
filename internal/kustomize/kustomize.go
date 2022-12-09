package kustomize

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"

	libExec "github.com/akuityio/bookkeeper/internal/exec"
	"github.com/akuityio/bookkeeper/internal/strings"
)

// SetImage runs `kustomize edit set image ...` in the specified directory.
// The specified directory must already exist and contain a kustomization.yaml
// file.
func SetImage(dir string, image string) error {
	repo, _, err := strings.SplitLast(image, ":")
	if err != nil {
		return errors.Wrapf(
			err,
			"error parsing image name %q",
			image,
		)
	}
	cmd := exec.Command( // nolint: gosec
		"kustomize",
		"edit",
		"set",
		"image",
		fmt.Sprintf(
			"%s=%s",
			repo,
			image,
		),
	)
	cmd.Dir = dir
	_, err = libExec.Exec(cmd)
	return err
}

// TODO: Document this
// TODO: Use repo server to do this
func Render(_ context.Context, path string, images []string) ([]byte, error) {
	cmd := exec.Command("kustomize", "build")
	cmd.Dir = path
	return libExec.Exec(cmd)
}
