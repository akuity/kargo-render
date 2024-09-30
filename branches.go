package render

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	libExec "github.com/akuity/kargo-render/internal/exec"
	"github.com/akuity/kargo-render/internal/file"
	"github.com/akuity/kargo-render/pkg/git"
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
		return nil, fmt.Errorf(
			"error checking for existence of branch metadata: %w",
			err,
		)
	} else if !exists {
		return nil, nil
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading branch metadata: %w", err)
	}
	md := &branchMetadata{}
	if err = yaml.Unmarshal(bytes, md); err != nil {
		return nil, fmt.Errorf("error unmarshaling branch metadata: %w", err)
	}
	return md, nil
}

// writeBranchMetadata attempts to marshal the provided BranchMetadata and write
// it to a .kargo-render/metadata.yaml file relative to the specified directory.
func writeBranchMetadata(md branchMetadata, repoPath string) error {
	bkDir := filepath.Join(repoPath, ".kargo-render")
	// Ensure the existence of the directory
	if err := os.MkdirAll(bkDir, 0755); err != nil {
		return fmt.Errorf("error ensuring existence of directory %q: %w", bkDir, err)
	}
	path := filepath.Join(bkDir, "metadata.yaml")
	bytes, err := yaml.Marshal(md)
	if err != nil {
		return fmt.Errorf("error marshaling branch metadata: %w", err)
	}
	if err = os.WriteFile(path, bytes, 0644); err != nil { // nolint: gosec
		return fmt.Errorf(
			"error writing branch metadata: %w",
			err,
		)
	}
	return nil
}

func switchToTargetBranch(rc requestContext) error {
	logger := rc.logger.WithField("targetBranch", rc.request.TargetBranch)

	// Check if the target branch exists on the remote
	remoteTargetBranchExists, err := rc.repo.RemoteBranchExists(rc.request.TargetBranch)
	if err != nil {
		return fmt.Errorf("error checking for existence of remote target branch: %w", err)
	}

	if remoteTargetBranchExists {
		logger.Debug("target branch exists on remote")
		if err = rc.repo.Fetch(); err != nil {
			return fmt.Errorf("error fetching from remote: %w", err)
		}
		logger.Debug("fetched from remote")
		if err = rc.repo.Checkout(rc.request.TargetBranch); err != nil {
			return fmt.Errorf("error checking out target branch: %w", err)
		}
		logger.Debug("checked out target branch")
		if err = rc.repo.Pull(rc.request.TargetBranch); err != nil {
			return fmt.Errorf("error pulling from remote: %w", err)
		}
		logger.Debug("pulled from remote")
		return nil
	}

	logger.Debug("target branch does not exist on remote")

	// Check if the target branch exists locally
	localTargetBranchExists, err := rc.repo.LocalBranchExists(rc.request.TargetBranch)
	if err != nil {
		return fmt.Errorf("error checking for existence of local target branch: %w", err)
	}

	if localTargetBranchExists {
		logger.Debug("target branch exists locally")
		if err = rc.repo.Checkout(rc.request.TargetBranch); err != nil {
			return fmt.Errorf("error checking out target branch: %w", err)
		}
		logger.Debug("checked out target branch")
	} else {
		logger.Debug("target branch does not exist locally")
		if err = rc.repo.CreateOrphanedBranch(rc.request.TargetBranch); err != nil {
			return fmt.Errorf("error creating new target branch: %w", err)
		}
		logger.Debug("created target branch locally")
	}

	if rc.request.LocalOutPath != "" {
		return nil // There's no need to push the new branch to the remote
	}

	if err = rc.repo.Commit(
		"Initial commit",
		&git.CommitOptions{
			AllowEmpty: true,
		},
	); err != nil {
		return fmt.Errorf("error making initial commit to new target branch: %w", err)
	}
	logger.Debug("made initial commit to new target branch")
	if err = rc.repo.Push(); err != nil {
		return fmt.Errorf("error pushing new target branch to remote: %w", err)
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
				fmt.Errorf("error checking for existence of commit branch: %w", err)
		}
		if commitBranchExists {
			logger.Debug("commit branch exists on remote")
			if err = rc.repo.Checkout(commitBranch); err != nil {
				return "", fmt.Errorf("error checking out commit branch: %w", err)
			}
			logger.Debug("checked out commit branch")
		} else {
			if err := rc.repo.CreateChildBranch(commitBranch); err != nil {
				return "", fmt.Errorf("error creating child of target branch: %w", err)
			}
			logger.Debug("created commit branch")
		}
	}

	// Clean the branch so we can replace its contents wholesale
	if err := cleanCommitBranch(
		rc.repo.WorkingDir(),
		rc.target.branchConfig.PreservedPaths,
	); err != nil {
		return "", fmt.Errorf("error cleaning commit branch: %w", err)
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

// copyBranchContents copies the entire contents of the source directory to the
// destination directory, except for .git.
func copyBranchContents(srcDir, dstDir string) error {
	// nolint: gosec
	if _, err := libExec.Exec(
		exec.Command("cp", "-r", srcDir, dstDir),
	); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(dstDir, ".git"))
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
