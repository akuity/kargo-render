package render

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/akuity/kargo-render/internal/kustomize"
	"github.com/akuity/kargo-render/internal/strings"
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
	rc requestContext,
	repoRoot string,
) (map[string][]byte, error) {
	logger := rc.logger
	manifests := map[string][]byte{}
	var err error
	for appName, appConfig := range rc.target.branchConfig.AppConfigs {
		appLogger := logger.WithField("app", appName)
		manifests[appName], err = s.renderFn(
			ctx,
			repoRoot,
			appConfig.ConfigManagement,
		)
		if err != nil {
			return nil, err
		}
		appLogger.Debug("completed manifest pre-rendering")
	}

	if !rc.request.AllowEmpty {
		// This is a sanity check. Argo CD does this also.
		for appName := range rc.target.branchConfig.AppConfigs {
			if manifests, ok := manifests[appName]; !ok || len(manifests) == 0 {
				return nil, fmt.Errorf(
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
	rc requestContext,
) ([]string, map[string][]byte, error) {
	logger := rc.logger

	tempDir, err := os.MkdirTemp("", "repo-scrap-")
	if err != nil {
		return nil, nil, fmt.Errorf(
			"error creating temporary directory %q for last mile rendering: %w",
			tempDir,
			err,
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
			return nil, nil, fmt.Errorf(
				"error creating directory %q for last mile rendering of app %q: %w",
				appDir,
				appName,
				err,
			)
		}
		// Create kustomization.yaml
		appKustomizationFile := filepath.Join(appDir, "kustomization.yaml")
		if err = os.WriteFile( // nolint: gosec
			appKustomizationFile,
			lastMileKustomizationBytes,
			0644,
		); err != nil {
			return nil, nil, fmt.Errorf(
				"error writing last-mile kustomization.yaml to %q: %w",
				appKustomizationFile,
				err,
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
			return nil, nil, fmt.Errorf(
				"error writing pre-rendered manifests to %q: %w",
				preRenderedPath,
				err,
			)
		}
		if manifests[appName], err =
			kustomize.Render(ctx, appDir, images); err != nil {
			return nil, nil, fmt.Errorf(
				"error rendering manifests from %q: %w",
				appDir,
				err,
			)
		}
		logger.WithField("app", appName).
			Debug("completed last-mile manifest rendering")
	}

	return images, manifests, nil
}
