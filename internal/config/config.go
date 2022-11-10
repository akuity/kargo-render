package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"

	"github.com/akuityio/bookkeeper/internal/file"
)

// RepoConfig is an interface for locating branch-specific Bookkeeper
// configuration.
type RepoConfig interface {
	// GetBranchConfig returns branch-specific Bookkeeper configuration.
	GetBranchConfig(branch string) BranchConfig
}

type repoConfig struct {
	BranchConfigs       []BranchConfig `json:"branchConfigs,omitempty"`
	DefaultBranchConfig *BranchConfig  `json:"defaultBranchConfig,omitempty"`
}

// BranchConfig encapsulates branch-specific Bookkeeper configuration.
type BranchConfig struct {
	// Name is the name of the branch to which this configuration applies.
	Name string `json:"name,omitempty"`
	// ConfigManagement encapsulates configuration management options to be
	// used with this branch.
	ConfigManagement ConfigManagementConfig `json:"configManagement,omitempty"`
	// OverlayPath is a path, relative to the root of the repository where
	// environment-specific configuration for this branch can be located. By
	// convention, if left unspecified, the path is assumed to be identical to the
	// name of the branch.
	//
	// TODO: Consider renaming this field. "Overlay" makes sense in the context
	// of Kustomize, but it isn't nomenclature that is used by either ytt or Helm.
	// A more generic term would be nice.
	OverlayPath string `json:"overlayPath,omitempty"`
	// OpenPR specifies whether to open a PR against TargetBranch (true) instead
	// of directly committing directly to it (false).
	OpenPR bool `json:"openPR,omitempty"`
}

// ConfigManagementConfig is a wrapper around more specific configuration for
// one of three supported configuration management tools: helm, kustomize, or
// ytt. Only one of its fields may be non-nil.
type ConfigManagementConfig struct { // nolint: revive
	// Helm encapsulates optional Helm configuration options.
	Helm *HelmConfig `json:"helm,omitempty"`
	// Kustomize encapsulates optional Kustomize configuration options.
	Kustomize *KustomizeConfig `json:"kustomize,omitempty"`
	// Ytt encapsulates optional ytt configuration options.
	Ytt *YttConfig `json:"ytt,omitempty"`
}

// HelmConfig encapsulates optional Helm configuration options.
type HelmConfig struct {
	// ReleaseName specified the release name that will be used when running
	// `helm template <release name> <chart> --values <values>`
	ReleaseName string `json:"releaseName,omitempty"`
}

// KustomizeConfig encapsulates optional Kustomize configuration options.
type KustomizeConfig struct {
}

// YttConfig encapsulates optional ytt configuration options.
type YttConfig struct {
}

// LoadRepoConfig attempts to load configuration from a Bookkeeper.json or
// Bookkeeper.yaml file in the specified directory. If no such file is found,
// default configuration is returned instead.
func LoadRepoConfig(repoPath string) (RepoConfig, error) {
	cfg := &repoConfig{}
	const baseConfigFilename = "Bookfile"
	jsonConfigPath := filepath.Join(
		repoPath,
		fmt.Sprintf("%s.json", baseConfigFilename),
	)
	yamlConfigPath := filepath.Join(
		repoPath,
		fmt.Sprintf("%s.yaml", baseConfigFilename),
	)
	if exists, err := file.Exists(jsonConfigPath); err != nil {
		return cfg,
			errors.Wrap(err, "error checking for existence of JSON config file")
	} else if exists {
		var bytes []byte
		if bytes, err = os.ReadFile(jsonConfigPath); err != nil {
			return cfg, errors.Wrap(err, "error reading JSON config file")
		}
		if err = json.Unmarshal(bytes, cfg); err != nil {
			return cfg, errors.Wrap(err, "error unmarshaling JSON config file")
		}
	} else if exists, err = file.Exists(yamlConfigPath); err != nil {
		return cfg,
			errors.Wrap(err, "error checking for existence of YAML config file")
	} else if exists {
		bytes, err := os.ReadFile(yamlConfigPath)
		if err != nil {
			return cfg, errors.Wrap(err, "error reading YAML config file")
		}
		if err = yaml.Unmarshal(bytes, cfg); err != nil {
			return cfg, errors.Wrap(err, "error unmarshaling YAML config file")
		}
	}
	return cfg, nil
}

func (r *repoConfig) GetBranchConfig(branch string) BranchConfig {
	for _, branchConfig := range r.BranchConfigs {
		if branchConfig.Name == branch {
			return branchConfig
		}
	}
	if r.DefaultBranchConfig != nil {
		cfg := r.DefaultBranchConfig
		cfg.Name = branch
		return *cfg
	}
	return BranchConfig{
		Name: branch,
		ConfigManagement: ConfigManagementConfig{
			Kustomize: &KustomizeConfig{},
		},
	}
}
