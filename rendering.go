package bookkeeper

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/akuityio/bookkeeper/internal/config"
	"github.com/akuityio/bookkeeper/internal/git"
	"github.com/akuityio/bookkeeper/internal/helm"
	"github.com/akuityio/bookkeeper/internal/kustomize"
	"github.com/akuityio/bookkeeper/internal/metadata"
	"github.com/akuityio/bookkeeper/internal/ytt"
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
	logger := s.logger.WithField("request", req.id)
	// Use the caller's preferred config management tool for pre-rendering.
	var bytes []byte
	var err error
	var cfgMgmtLogger *log.Entry
	if branchConfig.ConfigManagement.Helm != nil {
		cfgMgmtLogger = logger.WithFields(log.Fields{
			"configManagementTool": "helm",
			"releaseName":          branchConfig.ConfigManagement.Helm.ReleaseName,
			"chartPath":            branchConfig.ConfigManagement.Helm.ChartPath,
			"valuesPaths":          branchConfig.ConfigManagement.Helm.ValuesPaths,
		})
		bytes, err = helm.PreRender(
			repo.WorkingDir(),
			req.TargetBranch,
			branchConfig.ConfigManagement.Helm,
		)
	} else if branchConfig.ConfigManagement.Ytt != nil {
		cfgMgmtLogger = logger.WithFields(log.Fields{
			"configManagementTool": "ytt",
			"paths":                branchConfig.ConfigManagement.Ytt.Paths,
		})
		bytes, err = ytt.PreRender(
			repo.WorkingDir(),
			req.TargetBranch,
			branchConfig.ConfigManagement.Ytt,
		)
	} else {
		cfgMgmtLogger = logger.WithFields(log.Fields{
			"configManagementTool": "kustomize",
			"path":                 branchConfig.ConfigManagement.Kustomize.Path,
		})
		bytes, err = kustomize.PreRender(
			repo.WorkingDir(),
			req.TargetBranch,
			branchConfig.ConfigManagement.Kustomize,
		)
	}
	cfgMgmtLogger.Debug("completed pre-rendering")
	return bytes, err
}

func (s *service) renderLastMile(
	repo git.Repo,
	oldTargetBranchMetadata metadata.TargetBranchMetadata,
	intermediateMetadata *metadata.TargetBranchMetadata,
	req RenderRequest,
	preRenderedBytes []byte,
) ([]string, []byte, error) {
	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, nil, errors.Wrapf(
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
		return nil, nil, errors.Wrapf(
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
		return nil, nil, errors.Wrapf(
			err,
			"error writing to %q",
			kustomizationFile,
		)
	}

	// Apply new images from old target branch metadata if any were specified
	for _, image := range oldTargetBranchMetadata.ImageSubstitutions {
		if err = kustomize.SetImage(tempDir, image); err != nil {
			return nil, nil, errors.Wrapf(
				err,
				"error applying new image %q",
				image,
			)
		}
	}
	// Apply new images from intermediate metadata if any were specified
	if intermediateMetadata != nil {
		for _, image := range intermediateMetadata.ImageSubstitutions {
			if err = kustomize.SetImage(tempDir, image); err != nil {
				return nil, nil, errors.Wrapf(
					err,
					"error applying new image %q",
					image,
				)
			}
		}
	}
	// Apply new images from the request if any were specified. These will take
	// precedence over anything we already set.
	for _, image := range req.Images {
		if err = kustomize.SetImage(tempDir, image); err != nil {
			return nil, nil, errors.Wrapf(
				err,
				"error applying new image %q",
				image,
			)
		}
	}

	// Read images back from kustomization.yaml. This is a convenient way to
	// collapse images from intermediate metadata and request images into a single
	// list of images, with precedence given to the latter.
	kustomization := struct {
		Images []struct {
			Name   string `json:"name,omitempty"`
			NewTag string `json:"newTag,omitempty"`
		} `json:"images,omitempty"`
	}{}
	kustomizationBytes, err := os.ReadFile(kustomizationFile)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error reading kustomization.yaml")
	}
	if err = yaml.Unmarshal(kustomizationBytes, &kustomization); err != nil {
		return nil, nil, errors.Wrap(err, "error unmarshaling kustomization.yaml")
	}
	images := make([]string, len(kustomization.Images))
	for i, image := range kustomization.Images {
		images[i] = fmt.Sprintf("%s:%s", image.Name, image.NewTag)
	}

	fullyRenderedBytes, err := kustomize.LastMileRender(tempDir)
	return images, fullyRenderedBytes, errors.Wrapf(
		err,
		"error rendering configuration from %q",
		tempDir,
	)
}
