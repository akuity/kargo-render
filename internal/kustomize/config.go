package kustomize

import "github.com/akuity/bookkeeper/internal/file"

// Config encapsulates optional Kustomize configuration options.
type Config struct {
	// Path is a path to a directory, relative to the root of the repository,
	// where environment-specific Kustomize configuration for this branch can be
	// located. This will be the directory from which `kustomize build` is
	// executed. By convention, if left unspecified, the path is assumed to be
	// identical to the name of the branch.
	Path string `json:"path,omitempty"`
	// EnableHelm specifies whether Kustomize's Helm Chart Inflator should be
	// enabled. If left unspecified, it defaults to false -- not enabled.
	EnableHelm bool `json:"enableHelm,omitempty"`
}

// Expand expands all file/directory paths referenced by this configuration
// object, replacing placeholders of the form ${n} where n is a non-negative
// integer, with corresponding values from the provided string array. The
// modified object is returned.
func (c Config) Expand(values []string) Config {
	cfg := c
	cfg.Path = file.ExpandPath(c.Path, values)
	return cfg
}
