package ytt

import "github.com/akuity/kargo-render/internal/file"

// Config encapsulates optional ytt configuration options.
type Config struct {
	// Paths are paths to directories or files, relative to the root of the
	// repository, containing YTT templates or data. Each of these will be used as
	// a value for the `--file` flag in the `ytt` command. By convention, if left
	// unspecified, two paths are assumed: base/ and a path identical to the name
	// of the branch.
	Paths []string `json:"paths,omitempty"`
}

// Expand expands all file/directory paths referenced by this configuration
// object, replacing placeholders of the form ${n} where n is a non-negative
// integer, with corresponding values from the provided string array. The
// modified object is returned.
func (c Config) Expand(values []string) Config {
	cfg := c
	cfg.Paths = make([]string, len(c.Paths))
	for i, pathTemplate := range c.Paths {
		cfg.Paths[i] = file.ExpandPath(pathTemplate, values)
	}
	return cfg
}
