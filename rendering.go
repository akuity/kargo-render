package bookkeeper

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/akuityio/bookkeeper/internal/helm"
	"github.com/akuityio/bookkeeper/internal/kustomize"
	"github.com/akuityio/bookkeeper/internal/ytt"
)

var lastMileKustomizationBytes = []byte(
	`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- all.yaml
`,
)

func preRender(rc renderRequestContext) ([]byte, error) {
	logger := rc.logger
	// Use the caller's preferred config management tool for pre-rendering.
	var bytes []byte
	var err error
	var cfgMgmtLogger *log.Entry
	if rc.target.branchConfig.ConfigManagement.Helm != nil {
		// nolint: lll
		cfgMgmtLogger = logger.WithFields(log.Fields{
			"configManagementTool": "helm",
			"releaseName":          rc.target.branchConfig.ConfigManagement.Helm.ReleaseName,
			"chartPath":            rc.target.branchConfig.ConfigManagement.Helm.ChartPath,
			"valuesPaths":          rc.target.branchConfig.ConfigManagement.Helm.ValuesPaths,
		})
		bytes, err = helm.PreRender(
			rc.repo.WorkingDir(),
			rc.request.TargetBranch,
			rc.target.branchConfig.ConfigManagement.Helm,
		)
	} else if rc.target.branchConfig.ConfigManagement.Ytt != nil {
		cfgMgmtLogger = logger.WithFields(log.Fields{
			"configManagementTool": "ytt",
			"paths":                rc.target.branchConfig.ConfigManagement.Ytt.Paths,
		})
		bytes, err = ytt.PreRender(
			rc.repo.WorkingDir(),
			rc.request.TargetBranch,
			rc.target.branchConfig.ConfigManagement.Ytt,
		)
	} else {
		cfgMgmtLogger = logger.WithFields(log.Fields{
			"configManagementTool": "kustomize",
			"path":                 rc.target.branchConfig.ConfigManagement.Kustomize.Path, // nolint: lll
		})
		bytes, err = kustomize.PreRender(
			rc.repo.WorkingDir(),
			rc.request.TargetBranch,
			rc.target.branchConfig.ConfigManagement.Kustomize,
		)
	}
	cfgMgmtLogger.Debug("completed pre-rendering")
	return bytes, err
}

func renderLastMile(rc renderRequestContext) ([]string, []byte, error) {
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
	if err = os.WriteFile(
		preRenderedPath,
		rc.target.prerenderedConfig,
		0644,
	); err != nil {
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
	for _, image := range rc.target.oldBranchMetadata.ImageSubstitutions {
		if err = kustomize.SetImage(tempDir, image); err != nil {
			return nil, nil, errors.Wrapf(
				err,
				"error applying new image %q",
				image,
			)
		}
	}
	// Apply new images from intermediate metadata if any were specified
	if rc.intermediate.branchMetadata != nil {
		for _, image := range rc.intermediate.branchMetadata.ImageSubstitutions {
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
	for _, image := range rc.request.Images {
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
