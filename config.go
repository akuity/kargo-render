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

// repoConfig all Bookkeeper configuration options for a repository.
type repoConfig struct {
	// BranchConfigs is a map of branch-specific configuration indexed by branch
	// name.
	BranchConfigs map[string]branchConfig `json:"branchConfigs,omitempty"`
}

// branchConfig encapsulates branch-specific Bookkeeper configuration.
type branchConfig struct {
	// AppConfigs is a map of application-specific configuration indexed by app
	// name.
	AppConfigs map[string]appConfig `json:"appConfigs,omitempty"`
	// PRs encapsulates details about how to manage any pull requests associated
	// with this branch.
	PRs pullRequestConfig `json:"prs,omitempty"`
}

// appConfig encapsulates application-specific Bookkeeper configuration.
type appConfig struct {
	// ConfigManagement encapsulates configuration management options to be
	// used with this branch and app.
	ConfigManagement configManagementConfig `json:"configManagement,omitempty"`
	// OutputPath specifies a path relative to the root of the repository where
	// rendered manifests for this app will be stored in this branch.
	OutputPath string `json:"outputPath,omitempty"`
	// CombineManifests specifies whether rendered manifests should be combined
	// into a single file.
	CombineManifests bool `json:"combineManifests,omitempty"`
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

// pullRequestConfig encapsulates details related to PR management for a branch.
type pullRequestConfig struct {
	// Enabled specifies whether PRs should be opened for changes to a given
	// environment-specific branch.
	Enabled bool `json:"enabled,omitempty"`
	// UseUniqueBranchNames specifies whether each PR should be based on a
	// new/unique branch name. When this is false (the default), PRs to a given
	// environment-specific branch will be opened from a predictably names branch.
	// The consequence of using a new/unique branch name vs a single predictable
	// named branch will be either a new PR per render request for a given
	// environment-specific branch (if true) vs a single PR that batches all
	// unmerged changes to the environment-specific branch. Which of these one
	// prefers would depend on team preferences and the particulars of whatever
	// other automation is involved. There are valid reasons for using either
	// approach.
	UseUniqueBranchNames bool `json:"useUniqueBranchNames,omitempty"`
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
