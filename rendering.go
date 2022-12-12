package bookkeeper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/akuityio/bookkeeper/internal/file"
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

func preRender(
	ctx context.Context,
	rc renderRequestContext,
) (map[string][]byte, error) {
	logger := rc.logger
	manifests := map[string][]byte{}
	var err error
	for appName, appConfig := range rc.target.branchConfig.AppConfigs {
		appLogger := logger.WithField("app", appName)
		if appConfig.ConfigManagement.Helm != nil {
			chartPath := appConfig.ConfigManagement.Helm.ChartPath
			if chartPath == "" {
				chartPath = "base"
			}
			valuesPaths := appConfig.ConfigManagement.Helm.ValuesPaths
			if len(valuesPaths) == 0 {
				valuesPaths =
					[]string{filepath.Join(rc.request.TargetBranch, "values.yaml")}
			}
			appLogger = appLogger.WithFields(log.Fields{
				"configManagement": "helm",
				"releaseName":      appConfig.ConfigManagement.Helm.ReleaseName,
				"chartPath":        chartPath,
				"valuesPaths":      valuesPaths,
			})
			chartPath = filepath.Join(rc.repo.WorkingDir(), chartPath)
			absValuesPaths := make([]string, len(valuesPaths))
			for i, valuesPath := range valuesPaths {
				absValuesPaths[i] = filepath.Join(rc.repo.WorkingDir(), valuesPath)
			}
			manifests[appName], err = helm.Render(
				ctx,
				appConfig.ConfigManagement.Helm.ReleaseName,
				chartPath,
				absValuesPaths,
			)
		} else if appConfig.ConfigManagement.Ytt != nil {
			paths := appConfig.ConfigManagement.Ytt.Paths
			if len(paths) == 0 {
				paths = []string{"base", rc.request.TargetBranch}
			}
			appLogger = appLogger.WithFields(log.Fields{
				"configManagement": "ytt",
				"paths":            paths,
			})
			absPaths := make([]string, len(paths))
			for i, path := range paths {
				absPaths[i] = filepath.Join(rc.repo.WorkingDir(), path)
			}
			manifests[appName], err = ytt.Render(ctx, absPaths)
		} else {
			path := appConfig.ConfigManagement.Kustomize.Path
			if path == "" {
				path = rc.request.TargetBranch
			}
			appLogger = appLogger.WithFields(log.Fields{
				"configManagement": "kustomize",
				"path":             path,
			})
			path = filepath.Join(rc.repo.WorkingDir(), path)
			manifests[appName], err = kustomize.Render(ctx, path, nil)
		}
		appLogger.Debug("completed manifest pre-rendering")
	}
	return manifests, err
}

func renderLastMile(
	ctx context.Context,
	rc renderRequestContext,
) ([]string, map[string][]byte, error) {
	logger := rc.logger

	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, nil, errors.Wrapf(
			err,
			"error creating temporary directory %q for last mile rendering",
			tempDir,
		)
	}
	defer os.RemoveAll(tempDir)

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

	manifests := map[string][]byte{}
	for appName := range rc.target.branchConfig.AppConfigs {
		appDir := filepath.Join(tempDir, appName)
		if err = os.MkdirAll(appDir, 0755); err != nil {
			return nil, nil, errors.Wrapf(
				err,
				"error creating directory %q for last mile rendering of app %q",
				appDir,
				appName,
			)
		}
		appKustomizationFile := filepath.Join(appDir, "kustomization.yaml")
		if err = file.CopyFile(
			kustomizationFile,
			appKustomizationFile,
		); err != nil {
			return nil, nil, errors.Wrapf(
				err,
				"error copying kustomization.yaml from %q to %q",
				kustomizationFile,
				appKustomizationFile,
			)
		}
		// Write the pre-rendered manifests to a file
		preRenderedPath := filepath.Join(appDir, "all.yaml")
		// nolint: gosec
		if err = os.WriteFile(
			preRenderedPath,
			rc.target.prerenderedManifests[appName],
			0644,
		); err != nil {
			return nil, nil, errors.Wrapf(
				err,
				"error writing pre-rendered manifests to %q",
				preRenderedPath,
			)
		}
		if manifests[appName], err =
			kustomize.Render(ctx, appDir, nil); err != nil {
			return nil, nil, errors.Wrapf(
				err,
				"error rendering manifests from %q",
				appDir,
			)
		}
		logger.WithField("app", appName).
			Debug("completed last-mile manifest rendering")
	}

	return images, manifests, nil
}
