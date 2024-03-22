package argocd

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/reposerver/repository"
	"github.com/argoproj/argo-cd/v2/util/git"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/akuity/kargo-render/internal/file"
	"github.com/akuity/kargo-render/internal/manifests"
)

// ConfigManagementConfig is a wrapper around more specific configuration for
// the configuration management tools. Only one of its fields may be non-nil.
type ConfigManagementConfig struct {
	Path      string                                `json:"path,omitempty"`
	Helm      *ApplicationSourceHelm                `json:"helm,omitempty"`
	Kustomize *ApplicationSourceKustomize           `json:"kustomize,omitempty"`
	Directory *argoappv1.ApplicationSourceDirectory `json:"directory,omitempty"`
	Plugin    *argoappv1.ApplicationSourcePlugin    `json:"plugin,omitempty"`
}

// ApplicationSourceHelm holds configuration for Helm-based applications.
type ApplicationSourceHelm struct {
	argoappv1.ApplicationSourceHelm

	Namespace   string   `json:"namespace,omitempty"`
	K8SVersion  string   `json:"k8sVersion,omitempty"`
	APIVersions []string `json:"apiVersions,omitempty"`

	RepoURL string `json:"repoURL,omitempty"`
	Chart   string `json:"chart,omitempty"`
}

// ApplicationSourceKustomize holds configuration for Kustomize-based
// applications.
type ApplicationSourceKustomize struct {
	argoappv1.ApplicationSourceKustomize
	BuildOptions string `json:"buildOptions,omitempty"`
}

func expand(item map[string]any, values []string) {
	for k, v := range item {
		switch value := v.(type) {
		case string:
			item[k] = file.ExpandPath(value, values)
		case map[string]any:
			expand(value, values)
		case []any:
			for i, v := range value {
				switch v := v.(type) {
				case string:
					value[i] = file.ExpandPath(v, values)
				case map[string]any:
					expand(v, values)
				}
			}
		}
	}
}

func (c ConfigManagementConfig) Expand(
	values []string,
) (ConfigManagementConfig, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return c, err
	}
	var cfgMap map[string]any
	if err = json.Unmarshal(data, &cfgMap); err != nil {
		return c, err
	}
	expand(cfgMap, values)
	data, err = json.Marshal(cfgMap)
	if err != nil {
		return c, err
	}
	var cfg ConfigManagementConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return c, err
	}
	return cfg, nil
}

func Render(
	ctx context.Context,
	repoRoot string,
	cfg ConfigManagementConfig,
) ([]byte, error) {
	src := argoappv1.ApplicationSource{
		Plugin: cfg.Plugin,
	}
	var apiVersions []string
	var namespace string
	var k8sVersion string
	if cfg.Helm != nil {
		src.Helm = &cfg.Helm.ApplicationSourceHelm
		apiVersions = cfg.Helm.APIVersions
		namespace = cfg.Helm.Namespace
		k8sVersion = cfg.Helm.K8SVersion
	}
	var kustomizeOptions *argoappv1.KustomizeOptions
	if cfg.Kustomize != nil {
		src.Kustomize = &cfg.Kustomize.ApplicationSourceKustomize
		kustomizeOptions = &argoappv1.KustomizeOptions{
			BuildOptions: cfg.Kustomize.BuildOptions,
		}
	}

	res, err := repository.GenerateManifests(
		ctx,
		filepath.Join(repoRoot, cfg.Path),
		repoRoot, // Repo root
		"",       // Revision -- seems ok to be empty string
		&apiclient.ManifestRequest{
			// Both of these fields need to be non-nil
			Repo:              &argoappv1.Repository{},
			ApplicationSource: &src,
			KustomizeOptions:  kustomizeOptions,
			ApiVersions:       apiVersions,
			Namespace:         namespace,
			KubeVersion:       k8sVersion,
		},
		true,
		&git.NoopCredsStore{}, // No need for this
		// Allow any quantity of generated manifests
		resource.MustParse("0"),
		nil,
	)
	if err != nil {
		return nil,
			fmt.Errorf("error generating manifests using Argo CD repo server: %w", err)
	}

	// res.Manifests contains JSON manifests. We want YAML.
	yamlManifests, err := manifests.JSONStringsToYAMLBytes(res.Manifests)
	if err != nil {
		return nil, err
	}

	// Glue the manifests together
	return manifests.CombineYAML(yamlManifests), nil
}
