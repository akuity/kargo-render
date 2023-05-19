package kustomize

import (
	"context"
	"fmt"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/reposerver/repository"
	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/akuity/bookkeeper/internal/manifests"
	"github.com/akuity/bookkeeper/internal/strings"
)

// Render delegates, in-process to the Argo CD repo server to render plain YAML
// manifests from a directory containing a kustomization.yaml file. This
// function also accepts a list of images (address/name + tag) that will be
// substituted for older versions of the same image. Because of this capability,
// this function is used for last-mile rendering, even when a configuration
// management tool other than Kustomize is used for pre-rendering.
func Render(
	ctx context.Context,
	path string,
	images []string,
	enableHelm bool,
) ([]byte, error) {
	kustomizeImages := make(argoappv1.KustomizeImages, len(images))
	for i, image := range images {
		addr, _, _ := strings.SplitLast(image, ":")
		kustomizeImages[i] =
			argoappv1.KustomizeImage(fmt.Sprintf("%s=%s", addr, image))
	}

	var opts *argoappv1.KustomizeOptions
	if enableHelm {
		opts = &argoappv1.KustomizeOptions{
			BuildOptions: "--enable-helm",
		}
	}
	res, err := repository.GenerateManifests(
		ctx,
		path,
		// Seems ok for these next two arguments to be empty strings. If this is
		// last mile rendering, we might be doing this in a directory outside of any
		// repo. And event for regular rendering, we have already checked the
		// revision we want.
		"", // Repo root
		"", // Revision
		&apiclient.ManifestRequest{
			// Both of these fields need to be non-nil
			Repo: &argoappv1.Repository{},
			ApplicationSource: &argoappv1.ApplicationSource{
				Kustomize: &argoappv1.ApplicationSourceKustomize{
					Images: kustomizeImages,
				},
			},
			KustomizeOptions: opts,
		},
		true,
		&git.NoopCredsStore{}, // No need for this
		// TODO: Don't completely understand this next arg, but @alexmt says this is
		// right. Something to do with caching?
		resource.MustParse("0"),
		nil,
	)
	if err != nil {
		return nil,
			errors.Wrap(err, "error generating manifests using Argo CD repo server")
	}

	// res.Manifests contains JSON manifests. We want YAML.
	yamlManifests, err := manifests.JSONStringsToYAMLBytes(res.Manifests)
	if err != nil {
		return nil, err
	}

	// Glue the manifests together
	return manifests.CombineYAML(yamlManifests), nil
}
