package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"

	"github.com/akuityio/bookkeeper/internal/file"
)

//go:embed schema.json
var configSchemaBytes []byte
var configSchemaJSONLoader gojsonschema.JSONLoader

func init() {
	configSchemaJSONLoader = gojsonschema.NewBytesLoader(configSchemaBytes)
}

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
	// ReleaseName specifies the release name that will be used when executing the
	// `helm template` command.
	ReleaseName string `json:"releaseName,omitempty"`
	// ChartPath is a path to a directory, relative to the root of the repository,
	// where a Helm chart can be located. This is used as an argument in the
	// `helm template` command. By convention, if left unspecified, the value
	// `base/` is assumed.
	ChartPath string `json:"chartPath,omitempty"`
	// Values are paths to Helm values files (e.g. values.yaml), relative to the
	// root of the repository. Each of these will be used as a value for the
	// `--values` flag in the `helm template` command. By convention, if left
	// unspecified, one path will be assumed: <branch name>/values.yaml.
	ValuesPaths []string `json:"valuesPaths,omitempty"`
}

// KustomizeConfig encapsulates optional Kustomize configuration options.
type KustomizeConfig struct {
	// Path is a path to a directory, relative to the root of the repository,
	// where environment-specific Kustomize configuration for this branch can be
	// located. This will be the directory from which `kustomize build` is
	// executed. By convention, if left unspecified, the path is assumed to be
	// identical to the name of the branch.
	Path string `json:"path,omitempty"`
}

// YttConfig encapsulates optional ytt configuration options.
type YttConfig struct {
	// Paths are paths to directories or files, relative to the root of the
	// repository, containing YTT templates or data. Each of these will be used as
	// a value for the `--file` flag in the `ytt` command. By convention, if left
	// unspecified, two paths are assumed: base/ and a path identical to the name
	// of the branch.
	Paths []string `json:"paths,omitempty"`
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
	var configPath string
	if exists, err := file.Exists(jsonConfigPath); err != nil {
		return cfg,
			errors.Wrap(err, "error checking for existence of JSON config file")
	} else if exists {
		configPath = jsonConfigPath
	} else if exists, err = file.Exists(yamlConfigPath); err != nil {
		return cfg,
			errors.Wrap(err, "error checking for existence of YAML config file")
	} else if exists {
		configPath = yamlConfigPath
	}
	if configPath == "" {
		return cfg, nil
	}
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, errors.Wrap(err, "error reading Bookkeeper configuration")
	}
	if configBytes, err = normalizeAndValidate(configBytes); err != nil {
		return cfg, errors.Wrap(
			err,
			"error normalizing and validating Bookkeeper configuration",
		)
	}
	err = json.Unmarshal(configBytes, cfg)
	return cfg, errors.Wrap(err, "error unmarshaling Bookkeeper configuration")
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

func normalizeAndValidate(configBytes []byte) ([]byte, error) {
	// JSON is a subset of YAML, so it's safe to unconditionally pass JSON through
	// this function
	var err error
	if configBytes, err = yaml.YAMLToJSON(configBytes); err != nil {
		return nil,
			errors.Wrap(err, "error normalizing Bookkeeper configuration")
	}
	validationResult, err := gojsonschema.Validate(
		configSchemaJSONLoader,
		gojsonschema.NewBytesLoader(configBytes),
	)
	if err != nil {
		return nil, errors.Wrap(err, "error validating Bookkeeper configuration")
	}
	if !validationResult.Valid() {
		verrStrs := make([]string, len(validationResult.Errors()))
		for i, verr := range validationResult.Errors() {
			verrStrs[i] = verr.String()
		}
		return nil, errors.Errorf(
			"error validating Bookkeeper configuration: %s",
			strings.Join(verrStrs, "; "),
		)
	}
	return configBytes, nil
}
