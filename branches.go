package render

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"

	"github.com/akuity/kargo-render/internal/file"
)

// branchMetadata encapsulates details about an environment-specific branch for
// internal use by Kargo Render.
type branchMetadata struct {
	// SourceCommit ia a back-reference to the specific commit in the repository's
	// default branch (i.e. main or master) from which the manifests stored in
	// this branch were rendered.
	SourceCommit string `json:"sourceCommit,omitempty"`
	// ImageSubstitutions is a list of new images that were used in rendering this
	// branch.
	ImageSubstitutions []string `json:"imageSubstitutions,omitempty"`
}

// loadBranchMetadata attempts to load BranchMetadata from a
// .kargo-render/metadata.yaml file relative to the specified directory. If no
// such file is found a nil result is returned.
func loadBranchMetadata(repoPath string) (*branchMetadata, error) {
	path := filepath.Join(
		repoPath,
		".kargo-render",
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
	md := &branchMetadata{}
	err = yaml.Unmarshal(bytes, md)
	return md, errors.Wrap(err, "error unmarshaling branch metadata")
}

// writeBranchMetadata attempts to marshal the provided BranchMetadata and write
// it to a .kargo-render/metadata.yaml file relative to the specified directory.
func writeBranchMetadata(md branchMetadata, repoPath string) error {
	bkDir := filepath.Join(repoPath, ".kargo-render")
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

func switchToTargetBranch(rc requestContext) error {
	logger := rc.logger.WithField("targetBranch", rc.request.TargetBranch)

	// Check if the target branch exists on the remote
	targetBranchExists, err := rc.repo.RemoteBranchExists(rc.request.TargetBranch)
	if err != nil {
		return errors.Wrap(err, "error checking for existence of target branch")
	}

	if targetBranchExists {
		logger.Debug("target branch exists on remote")
		if err = rc.repo.Checkout(rc.request.TargetBranch); err != nil {
			return errors.Wrap(err, "error checking out target branch")
		}
		logger.Debug("checked out target branch")
		return nil
	}

	logger.Debug("target branch does not exist on remote")
	if err = rc.repo.CreateOrphanedBranch(rc.request.TargetBranch); err != nil {
		return errors.Wrap(err, "error creating new target branch")
	}
	logger.Debug("created target branch")
	if err =
		writeBranchMetadata(branchMetadata{}, rc.repo.WorkingDir()); err != nil {
		return errors.Wrap(err, "error writing blank target branch metadata")
	}
	logger.Debug("wrote blank target branch metadata")
	if err = rc.repo.AddAllAndCommit("Initial commit"); err != nil {
		return errors.Wrap(err, "error making initial commit to new target branch")
	}
	logger.Debug("made initial commit to new target branch")
	if err = rc.repo.Push(); err != nil {
		return errors.Wrap(err, "error pushing new target branch to remote")
	}
	logger.Debug("pushed new target branch to remote")

	return nil
}

func switchToCommitBranch(rc requestContext) (string, error) {
	logger := rc.logger.WithField("targetBranch", rc.request.TargetBranch)

	var commitBranch string
	if !rc.target.branchConfig.PRs.Enabled {
		commitBranch = rc.request.TargetBranch
		logger.Debug(
			"changes will be written directly to the target branch",
		)
	} else {
		if rc.target.branchConfig.PRs.UseUniqueBranchNames {
			commitBranch = fmt.Sprintf("prs/kargo-render/%s", rc.request.id)
		} else {
			commitBranch = fmt.Sprintf("prs/kargo-render/%s", rc.request.TargetBranch)
		}
		logger = logger.WithField("commitBranch", commitBranch)
		logger.Debug("changes will be PR'ed to the target branch")
		commitBranchExists, err := rc.repo.RemoteBranchExists(commitBranch)
		if err != nil {
			return "",
				errors.Wrap(err, "error checking for existence of commit branch")
		}
		if commitBranchExists {
			logger.Debug("commit branch exists on remote")
			if err = rc.repo.Checkout(commitBranch); err != nil {
				return "", errors.Wrap(err, "error checking out commit branch")
			}
			logger.Debug("checked out commit branch")
		} else {
			if err := rc.repo.CreateChildBranch(commitBranch); err != nil {
				return "", errors.Wrap(err, "error creating child of target branch")
			}
			logger.Debug("created commit branch")
		}
	}

	// Clean the branch so we can replace its contents wholesale
	if err := cleanCommitBranch(
		rc.repo.WorkingDir(),
		rc.target.branchConfig.PreservedPaths,
	); err != nil {
		return "", errors.Wrap(err, "error cleaning commit branch")
	}
	logger.Debug("cleaned commit branch")

	return commitBranch, nil
}

// cleanCommitBranch deletes the entire contents of the specified directory
// EXCEPT for the paths specified by preservedPaths.
func cleanCommitBranch(dir string, preservedPaths []string) error {
	_, err := cleanDir(
		dir,
		normalizePreservedPaths(
			dir,
			append(preservedPaths, ".git", ".kargo-render"),
		),
	)
	return err
}

// normalizePreservedPaths converts the relative paths in the preservedPaths
// argument to absolute paths relative to the workingDir argument. It also
// removes any trailing path separators from the paths.
func normalizePreservedPaths(
	workingDir string,
	preservedPaths []string,
) []string {
	normalizedPreservedPaths := make([]string, len(preservedPaths))
	for i, preservedPath := range preservedPaths {
		if strings.HasSuffix(preservedPath, string(os.PathSeparator)) {
			preservedPath = preservedPath[:len(preservedPath)-1]
		}
		normalizedPreservedPaths[i] = filepath.Join(workingDir, preservedPath)
	}
	return normalizedPreservedPaths
}

// cleanDir recursively deletes the entire contents of the directory specified
// by the absolute path dir EXCEPT for any paths specified by the preservedPaths
// argument. The function returns true if dir is left empty afterwards and false
// otherwise.
func cleanDir(dir string, preservedPaths []string) (bool, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, item := range items {
		path := filepath.Join(dir, item.Name())
		if isPathPreserved(path, preservedPaths) {
			continue
		}
		if item.IsDir() {
			var isEmpty bool
			if isEmpty, err = cleanDir(path, preservedPaths); err != nil {
				return false, err
			}
			if isEmpty {
				if err = os.Remove(path); err != nil {
					return false, err
				}
			}
		} else if err = os.Remove(path); err != nil {
			return false, err
		}
	}
	if items, err = os.ReadDir(dir); err != nil {
		return false, err
	}
	return len(items) == 0, nil
}

// isPathPreserved returns true if the specified path is among those specified
// by the preservedPaths argument. Both path and preservedPaths MUST be absolute
// paths. Paths to directories MUST NOT end with a trailing path separator.
func isPathPreserved(path string, preservedPaths []string) bool {
	for _, preservedPath := range preservedPaths {
		if path == preservedPath {
			return true
		}
	}
	return false
}
