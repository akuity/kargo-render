package helm

import "github.com/akuity/bookkeeper/internal/file"

// Config encapsulates optional Helm configuration options.
type Config struct {
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

// Expand expands all file/directory paths referenced by this configuration
// object, replacing placeholders of the form ${n} where n is a non-negative
// integer, with corresponding values from the provided string array. The
// modified object is returned.
func (c Config) Expand(values []string) Config {
	cfg := c
	cfg.ChartPath = file.ExpandPath(c.ChartPath, values)
	cfg.ValuesPaths = make([]string, len(c.ValuesPaths))
	for i, pathTemplate := range c.ValuesPaths {
		cfg.ValuesPaths[i] = file.ExpandPath(pathTemplate, values)
	}
	return cfg
}
