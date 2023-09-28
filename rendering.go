package bookkeeper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/akuity/bookkeeper/internal/kustomize"
	"github.com/akuity/bookkeeper/internal/strings"
)

var lastMileKustomizationBytes = []byte(
	`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- all.yaml
`,
)

func (s *service) preRender(
	ctx context.Context,
	rc renderRequestContext,
	repoDir string,
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
				"namespace":        appConfig.ConfigManagement.Helm.Namespace,
				"chartPath":        chartPath,
				"valuesPaths":      valuesPaths,
			})
			chartPath = filepath.Join(repoDir, chartPath)
			absValuesPaths := make([]string, len(valuesPaths))
			for i, valuesPath := range valuesPaths {
				absValuesPaths[i] = filepath.Join(repoDir, valuesPath)
			}
			manifests[appName], err = s.helmRenderFn(
				ctx,
				appConfig.ConfigManagement.Helm.ReleaseName,
				appConfig.ConfigManagement.Helm.Namespace,
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
				absPaths[i] = filepath.Join(repoDir, path)
			}
			manifests[appName], err = s.yttRenderFn(ctx, absPaths)
		} else {
			path := appConfig.ConfigManagement.Kustomize.Path
			if path == "" {
				path = rc.request.TargetBranch
			}
			appLogger = appLogger.WithFields(log.Fields{
				"configManagement": "kustomize",
				"path":             path,
			})
			path = filepath.Join(repoDir, path)
			manifests[appName], err = s.kustomizeRenderFn(
				ctx,
				path,
				nil,
				appConfig.ConfigManagement.Kustomize.EnableHelm,
				appConfig.ConfigManagement.Kustomize.LoadRestrictor,
			)
		}
		if err != nil {
			return nil, err
		}
		appLogger.Debug("completed manifest pre-rendering")
	}

	if !rc.request.AllowEmpty {
		// This is a sanity check. Argo CD does this also.
		for appName := range rc.target.branchConfig.AppConfigs {
			if manifests, ok := manifests[appName]; !ok || len(manifests) == 0 {
				return nil, errors.Errorf(
					"pre-rendered manifests for app %q contain 0 bytes; this looks "+
						"like a mistake and allowEmpty is not set; refusing to proceed",
					appName,
				)
			}
		}
	}
	return manifests, nil
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

	imageMap := map[string]string{}
	for _, imageSub := range rc.target.oldBranchMetadata.ImageSubstitutions {
		addr, tag, _ := strings.SplitLast(imageSub, ":")
		imageMap[addr] = tag
	}
	if rc.intermediate.branchMetadata != nil {
		for _, imageSub := range rc.intermediate.branchMetadata.ImageSubstitutions {
			addr, tag, _ := strings.SplitLast(imageSub, ":")
			imageMap[addr] = tag
		}
	}
	if rc.target.commit.oldBranchMetadata != nil {
		for _, imageSub := range rc.target.commit.oldBranchMetadata.ImageSubstitutions { // nolint: lll
			addr, tag, _ := strings.SplitLast(imageSub, ":")
			imageMap[addr] = tag
		}
	}
	for _, imageSub := range rc.request.Images {
		addr, tag, _ := strings.SplitLast(imageSub, ":")
		imageMap[addr] = tag
	}
	images := make([]string, len(imageMap))
	i := 0
	for addr, tag := range imageMap {
		images[i] = fmt.Sprintf("%s:%s", addr, tag)
		i++
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
		// Create kustomization.yaml
		appKustomizationFile := filepath.Join(appDir, "kustomization.yaml")
		if err = os.WriteFile( // nolint: gosec
			appKustomizationFile,
			lastMileKustomizationBytes,
			0644,
		); err != nil {
			return nil, nil, errors.Wrapf(
				err,
				"error writing last-mile kustomization.yaml to %q",
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
			kustomize.Render(ctx, appDir, images, false, ""); err != nil {
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
