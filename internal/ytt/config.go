package ytt

// Config encapsulates optional ytt configuration options.
type Config struct {
	// Paths are paths to directories or files, relative to the root of the
	// repository, containing YTT templates or data. Each of these will be used as
	// a value for the `--file` flag in the `ytt` command. By convention, if left
	// unspecified, two paths are assumed: base/ and a path identical to the name
	// of the branch.
	Paths []string `json:"paths,omitempty"`
}
