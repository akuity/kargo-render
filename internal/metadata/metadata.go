package metadata

import (
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"

	"github.com/akuityio/bookkeeper/internal/file"
)

// TargetBranchMetadata encapsulates details about an environment-specific
// branch for internal use by Bookkeeper.
type TargetBranchMetadata struct {
	// SourceCommit ia a back-reference to the specific commit in the repository's
	// default branch (i.e. main or master) from which the configuration stored in
	// this branch was rendered.
	SourceCommit string `json:"sourceCommit,omitempty"`
	// ImageSubstitutions is a list of new images that were used in rendering this
	// branch.
	ImageSubstitutions []string `json:"imageSubstitutions,omitempty"`
}

// LoadTargetBranchMetadata attempts to load TargetBranchMetadata from a
// .bookkeeper/metadata.yaml file relative to the specified directory. If no
// such file is found a nil result is returned.
func LoadTargetBranchMetadata(repoPath string) (*TargetBranchMetadata, error) {
	path := filepath.Join(
		repoPath,
		".bookkeeper",
		"metadata.yaml",
	)
	if exists, err := file.Exists(path); err != nil {
		return nil, errors.Wrap(
			err,
			"error checking for existence of branch metadata",
		)
	} else if !exists {
		return nil, nil
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "error reading branch metadata")
	}
	md := &TargetBranchMetadata{}
	err = yaml.Unmarshal(bytes, md)
	return md, errors.Wrap(err, "error unmarshaling branch metadata")
}

// WriteTargetBranchMetadata attempts to marshal the provided
// TargetBranchMetadata and write it to a .bookkeeper/metadata.yaml file
// relative to the specified directory.
func WriteTargetBranchMetadata(md TargetBranchMetadata, repoPath string) error {
	bkDir := filepath.Join(repoPath, ".bookkeeper")
	// Ensure the existence of the directory
	if err := os.MkdirAll(bkDir, 0755); err != nil {
		return errors.Wrapf(err, "error ensuring existence of directory %q", bkDir)
	}
	path := filepath.Join(bkDir, "metadata.yaml")
	bytes, err := yaml.Marshal(md)
	if err != nil {
		return errors.Wrap(err, "error marshaling branch metadata")
	}
	return errors.Wrap(
		os.WriteFile(path, bytes, 0644), // nolint: gosec
		"error writing branch metadata",
	)
}
