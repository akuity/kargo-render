package bookkeeper

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
	"github.com/akuityio/bookkeeper/internal/helm"
	"github.com/akuityio/bookkeeper/internal/kustomize"
	"github.com/akuityio/bookkeeper/internal/ytt"
)

//go:embed schema.json
var configSchemaBytes []byte
var configSchemaJSONLoader gojsonschema.JSONLoader

func init() {
	configSchemaJSONLoader = gojsonschema.NewBytesLoader(configSchemaBytes)
}

type repoConfig struct {
	BranchConfigs       []branchConfig `json:"branchConfigs,omitempty"`
	DefaultBranchConfig *branchConfig  `json:"defaultBranchConfig,omitempty"`
}

// branchConfig encapsulates branch-specific Bookkeeper configuration.
type branchConfig struct {
	// Name is the name of the branch to which this configuration applies.
	Name string `json:"name,omitempty"`
	// ConfigManagement encapsulates configuration management options to be
	// used with this branch.
	ConfigManagement configManagementConfig `json:"configManagement,omitempty"`
	// OpenPR specifies whether to open a PR against TargetBranch (true) instead
	// of directly committing directly to it (false).
	OpenPR bool `json:"openPR,omitempty"`
}

// configManagementConfig is a wrapper around more specific configuration for
// one of three supported configuration management tools: helm, kustomize, or
// ytt. Only one of its fields may be non-nil.
type configManagementConfig struct { // nolint: revive
	// Helm encapsulates optional Helm configuration options.
	Helm *helm.Config `json:"helm,omitempty"`
	// Kustomize encapsulates optional Kustomize configuration options.
	Kustomize *kustomize.Config `json:"kustomize,omitempty"`
	// Ytt encapsulates optional ytt configuration options.
	Ytt *ytt.Config `json:"ytt,omitempty"`
}

// loadRepoConfig attempts to load configuration from a Bookkeeper.json or
// Bookkeeper.yaml file in the specified directory. If no such file is found,
// default configuration is returned instead.
func loadRepoConfig(repoPath string) (*repoConfig, error) {
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

func (r *repoConfig) getBranchConfig(branch string) branchConfig {
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
	return branchConfig{
		Name: branch,
		ConfigManagement: configManagementConfig{
			Kustomize: &kustomize.Config{},
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
