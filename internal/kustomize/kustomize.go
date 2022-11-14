package kustomize

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/akuityio/bookkeeper/internal/config"
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
	return errors.Wrapf(
		cmd.Run(),
		"error running kustomize set image in directory %q",
		dir,
	)
}

// TODO: Document this
func PreRender(
	repoRoot string,
	targetBranch string,
	cfg *config.KustomizeConfig,
) ([]byte, error) {
	cmd := buildPreRenderCmd(repoRoot, targetBranch, cfg)
	yamlBytes, err := cmd.Output()
	return yamlBytes, errors.Wrapf(
		err,
		"error running `%s` in directory %q",
		cmd.String(),
		cmd.Dir,
	)
}

func buildPreRenderCmd(
	repoRoot string,
	targetBranch string,
	cfg *config.KustomizeConfig,
) *exec.Cmd {
	if cfg == nil {
		cfg = &config.KustomizeConfig{}
	}
	cmd := exec.Command("kustomize", "build")
	if cfg.Path != "" {
		cmd.Dir = filepath.Join(repoRoot, cfg.Path)
	} else {
		cmd.Dir = filepath.Join(repoRoot, targetBranch)
	}
	return cmd
}

// TODO: Document this
func LastMileRender(dir string) ([]byte, error) {
	cmd := exec.Command("kustomize", "build")
	cmd.Dir = dir
	yamlBytes, err := cmd.Output()
	return yamlBytes, errors.Wrapf(
		err,
		"error running `%s` in directory %q",
		cmd.String(),
		cmd.Dir,
	)
}
