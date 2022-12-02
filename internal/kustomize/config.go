package kustomize

// Config encapsulates optional Kustomize configuration options.
type Config struct {
	// Path is a path to a directory, relative to the root of the repository,
	// where environment-specific Kustomize configuration for this branch can be
	// located. This will be the directory from which `kustomize build` is
	// executed. By convention, if left unspecified, the path is assumed to be
	// identical to the name of the branch.
	Path string `json:"path,omitempty"`
}
