package bookkeeper

import (
	"os"
	"path/filepath"

	"github.com/akuityio/bookkeeper/internal/config"
	"github.com/akuityio/bookkeeper/internal/git"
	"github.com/akuityio/bookkeeper/internal/helm"
	"github.com/akuityio/bookkeeper/internal/kustomize"
	"github.com/akuityio/bookkeeper/internal/ytt"
	"github.com/pkg/errors"
)

var lastMileKustomizationBytes = []byte(
	`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- all.yaml
`,
)

func (s *service) preRender(
	repo git.Repo,
	branchConfig config.BranchConfig,
	req RenderRequest,
) ([]byte, error) {
	baseDir := filepath.Join(repo.WorkingDir(), "base")

	// Use branchConfig.OverlayPath as the source for environment-specific
	// configuration (for instance, a Kustomize overlay) unless it isn't specified
	// -- then default to the convention -- assuming the path to the
	// environment-specific configuration is identical to the name of the target
	// branch.
	var envDir string
	if branchConfig.OverlayPath != "" {
		envDir = filepath.Join(repo.WorkingDir(), branchConfig.OverlayPath)
	} else {
		envDir = filepath.Join(repo.WorkingDir(), req.TargetBranch)
	}

	// Use the caller's preferred config management tool for pre-rendering.
	var preRenderedBytes []byte
	var err error
	if branchConfig.ConfigManagement.Helm != nil {
		preRenderedBytes, err = helm.Render(
			branchConfig.ConfigManagement.Helm.ReleaseName,
			baseDir,
			envDir,
		)
	} else if branchConfig.ConfigManagement.Kustomize != nil {
		preRenderedBytes, err = kustomize.Render(envDir)
	} else if branchConfig.ConfigManagement.Ytt != nil {
		preRenderedBytes, err = ytt.Render(baseDir, envDir)
	} else {
		preRenderedBytes, err = kustomize.Render(envDir)
	}

	return preRenderedBytes, err
}

func (s *service) renderLastMile(
	repo git.Repo,
	req RenderRequest,
	preRenderedBytes []byte,
) ([]byte, error) {
	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, errors.Wrapf(
			err,
			"error creating temporary directory %q for last mile rendering",
			tempDir,
		)
	}
	defer os.RemoveAll(tempDir)

	// Write the pre-rendered bytes to a file
	preRenderedPath := filepath.Join(tempDir, "all.yaml")
	// nolint: gosec
	if err = os.WriteFile(preRenderedPath, preRenderedBytes, 0644); err != nil {
		return nil, errors.Wrapf(
			err,
			"error writing pre-rendered configuration to %q",
			preRenderedPath,
		)
	}

	// Create kustomization.yaml
	kustomizationFile := filepath.Join(tempDir, "kustomization.yaml")
	if err = os.WriteFile( // nolint: gosec
		kustomizationFile,
		lastMileKustomizationBytes,
		0644,
	); err != nil {
		return nil, errors.Wrapf(
			err,
			"error writing to %q",
			kustomizationFile,
		)
	}

	// Apply new images if any were specified
	for _, image := range req.Images {
		if err = kustomize.SetImage(tempDir, image); err != nil {
			return nil, errors.Wrapf(
				err,
				"error applying new image %q",
				image,
			)
		}
	}

	fullyRenderedBytes, err := kustomize.Render(tempDir)
	return fullyRenderedBytes, errors.Wrapf(
		err,
		"error rendering configuration from %q",
		tempDir,
	)
}
