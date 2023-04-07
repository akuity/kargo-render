package helm

import (
	"context"

	"github.com/akuity/bookkeeper/internal/manifests"
	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/reposerver/repository"
	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Render delegates, in-process to the Argo CD repo server to render plain YAML
// manifests from a Helm chart.
func Render(
	ctx context.Context,
	releaseName string,
	chartPath string,
	valuesPaths []string,
) ([]byte, error) {
	res, err := repository.GenerateManifests(
		ctx,
		chartPath,
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
				Helm: &argoappv1.ApplicationSourceHelm{
					ReleaseName: releaseName,
					ValueFiles:  valuesPaths,
				},
			},
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
