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
	// Use the caller's preferred config management tool for pre-rendering.
	if branchConfig.ConfigManagement.Helm != nil {
		return helm.PreRender(
			repo.WorkingDir(),
			req.TargetBranch,
			branchConfig.ConfigManagement.Helm,
		)
	}
	if branchConfig.ConfigManagement.Ytt != nil {
		return ytt.PreRender(
			repo.WorkingDir(),
			req.TargetBranch,
			branchConfig.ConfigManagement.Ytt,
		)
	}
	return kustomize.PreRender(
		repo.WorkingDir(),
		req.TargetBranch,
		branchConfig.ConfigManagement.Kustomize,
	)
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

	fullyRenderedBytes, err := kustomize.LastMileRender(tempDir)
	return fullyRenderedBytes, errors.Wrapf(
		err,
		"error rendering configuration from %q",
		tempDir,
	)
}
