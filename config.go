package render

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xeipuuv/gojsonschema"
	"sigs.k8s.io/yaml"

	"github.com/akuity/kargo-render/internal/argocd"
	"github.com/akuity/kargo-render/internal/file"

	_ "embed"
)

//go:embed schema.json
var configSchemaBytes []byte

//go:embed argocd-schema.json
var argocdConfigSchemaBytes []byte

var configSchema *gojsonschema.Schema

func init() {
	sl := gojsonschema.NewSchemaLoader()
	if err := sl.AddSchema("argocd-schema.json", gojsonschema.NewBytesLoader(argocdConfigSchemaBytes)); err != nil {
		panic(fmt.Sprintf("error adding Argo CD schema: %s", err))
	}

	var err error
	if configSchema, err = sl.Compile(gojsonschema.NewBytesLoader(configSchemaBytes)); err != nil {
		panic(fmt.Sprintf("error compiling schema: %s", err))
	}
}

// repoConfig encapsulates all Kargo Render configuration options for a
// repository.
type repoConfig struct {
	// BranchConfigs is a list of branch-specific configurations.
	BranchConfigs []branchConfig `json:"branchConfigs,omitempty"`
}

func (r *repoConfig) GetBranchConfig(name string) (branchConfig, error) {
	for _, cfg := range r.BranchConfigs {
		if cfg.Name == name {
			return cfg, nil
		}
		if cfg.Pattern != "" {
			regex, err := regexp.Compile(cfg.Pattern)
			if err != nil {
				return branchConfig{},
					fmt.Errorf("error compiling regular expression /%s/", cfg.Pattern)
			}
			submatches := regex.FindStringSubmatch(name)
			if len(submatches) > 0 {
				return cfg.expand(submatches)
			}
		}
	}
	return branchConfig{}, nil
}

// branchConfig encapsulates branch-specific Kargo Render configuration.
type branchConfig struct {
	// Name is the name of the environment-specific branch this configuration is
	// for. This is mutually exclusive with the Pattern field.
	Name string `json:"name,omitempty"`
	// Pattern is a regular expression that can be used to specify multiple
	// environment-specific branches this configuration is for.
	Pattern string `json:"pattern,omitempty"`
	// AppConfigs is a map of application-specific configuration indexed by app
	// name.
	AppConfigs map[string]appConfig `json:"appConfigs,omitempty"`
	// PRs encapsulates details about how to manage any pull requests associated
	// with this branch.
	PRs pullRequestConfig `json:"prs,omitempty"`
	// PreservedPaths specifies paths relative to the root of the repository that
	// should be exempted from pre-render cleaning (deletion) of
	// environment-specific branch contents. This is useful for preserving any
	// environment-specific files that are manually maintained. Typically there
	// are very few such files, if any at all, with an environment-specific
	// CODEOWNERS file at the root of the repository being the most emblematic
	// exception. Paths may be to files or directories. Any path to a directory
	// will cause that directory's entire contents to be preserved.
	PreservedPaths []string `json:"preservedPaths,omitempty"`
}

func (b branchConfig) expand(values []string) (branchConfig, error) {
	cfg := b
	cfg.AppConfigs = map[string]appConfig{}
	for appName, appConfig := range b.AppConfigs {
		var err error
		if cfg.AppConfigs[appName], err = appConfig.expand(values); err != nil {
			return cfg, fmt.Errorf(
				"error expanding app config for app %q: %w",
				appName,
				err,
			)
		}
	}

	for i, path := range b.PreservedPaths {
		b.PreservedPaths[i] = file.ExpandPath(path, values)
	}
	return cfg, nil
}

// appConfig encapsulates application-specific Kargo Render configuration.
type appConfig struct {
	// ConfigManagement encapsulates configuration management options to be
	// used with this branch and app.
	ConfigManagement argocd.ConfigManagementConfig `json:"configManagement"`
	// OutputPath specifies a path relative to the root of the repository where
	// rendered manifests for this app will be stored in this branch.
	OutputPath string `json:"outputPath,omitempty"`
	// CombineManifests specifies whether rendered manifests should be combined
	// into a single file.
	CombineManifests bool `json:"combineManifests,omitempty"`
}

func (a appConfig) expand(values []string) (appConfig, error) {
	cfg := a
	var err error
	if cfg.ConfigManagement, err = a.ConfigManagement.Expand(values); err != nil {
		return cfg, fmt.Errorf("error expanding config management config: %w", err)
	}
	cfg.OutputPath = file.ExpandPath(a.OutputPath, values)
	return cfg, nil
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

// loadRepoConfig attempts to load configuration from a kargo-render.json or
// kargo-render.yaml file in the specified directory. If no such file is found,
// default configuration is returned instead.
func loadRepoConfig(repoPath string) (*repoConfig, error) {
	cfg := &repoConfig{}
	const baseConfigFilename = "kargo-render"
	jsonConfigPath := filepath.Join(
		repoPath,
		fmt.Sprintf("%s.json", baseConfigFilename),
	)
	yamlConfigPath := filepath.Join(
		repoPath,
		fmt.Sprintf("%s.yaml", baseConfigFilename),
	)
	var configPath string
	if jsonExists, err := file.Exists(jsonConfigPath); err != nil {
		return cfg,
			fmt.Errorf("error checking for existence of JSON config file: %w", err)
	} else if jsonExists {
		configPath = jsonConfigPath
	} else if yamlExists, err := file.Exists(yamlConfigPath); err != nil {
		return cfg,
			fmt.Errorf("error checking for existence of YAML config file: %w", err)
	} else if yamlExists {
		configPath = yamlConfigPath
	}
	if configPath == "" {
		return cfg, nil
	}
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, fmt.Errorf("error reading Kargo Render configuration: %w", err)
	}
	if configBytes, err = normalizeAndValidate(configBytes); err != nil {
		return cfg, fmt.Errorf(
			"error normalizing and validating Kargo Render configuration: %w",
			err,
		)
	}
	if err = json.Unmarshal(configBytes, cfg); err != nil {
		return cfg, fmt.Errorf("error unmarshaling Kargo Render configuration: %w", err)
	}
	return cfg, nil
}

func normalizeAndValidate(configBytes []byte) ([]byte, error) {
	// JSON is a subset of YAML, so it's safe to unconditionally pass JSON through
	// this function
	var err error
	if configBytes, err = yaml.YAMLToJSON(configBytes); err != nil {
		return nil,
			fmt.Errorf("error normalizing Kargo Render configuration: %w", err)
	}

	validationResult, err := configSchema.Validate(gojsonschema.NewBytesLoader(configBytes))
	if err != nil {
		return nil, fmt.Errorf("error validating Kargo Render configuration: %w", err)
	}
	if !validationResult.Valid() {
		verrStrs := make([]string, len(validationResult.Errors()))
		for i, verr := range validationResult.Errors() {
			verrStrs[i] = verr.String()
		}
		return nil, fmt.Errorf(
			"error validating Kargo Render configuration: %s",
			strings.Join(verrStrs, "; "),
		)
	}
	return configBytes, nil
}
