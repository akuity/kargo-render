package git

import (
	"bufio"
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	libExec "github.com/akuity/kargo-render/internal/exec"
)

const (
	RemoteOrigin = "origin"

	tmpPrefix = "repo-"
)

// RepoCredentials represents the credentials for connecting to a private git
// repository.
type RepoCredentials struct {
	// SSHPrivateKey is a private key that can be used for both reading from and
	// writing to some remote repository.
	SSHPrivateKey string `json:"sshPrivateKey,omitempty"`
	// Username identifies a principal, which combined with the value of the
	// Password field, can be used for both reading from and writing to some
	// remote repository.
	Username string `json:"username,omitempty"`
	// Password, when combined with the principal identified by the Username
	// field, can be used for both reading from and writing to some remote
	// repository.
	Password string `json:"password,omitempty"`
}

// Repo is an interface for interacting with a git repository.
type Repo interface {
	// AddAll stages pending changes for commit.
	AddAll() error
	// AddAllAndCommit is a convenience function that stages pending changes for
	// commit to the current branch and then commits them using the provided
	// commit message.
	AddAllAndCommit(message string) error
	// Clean cleans the working directory.
	Clean() error
	// Close cleans up file system resources used by this repository. This should
	// always be called before a repository goes out of scope.
	Close() error
	// Checkout checks out the specified branch.
	Checkout(branch string) error
	// Commit commits staged changes to the current branch.
	Commit(message string, opts *CommitOptions) error
	// CreateChildBranch creates a new branch that is a child of the current
	// branch.
	CreateChildBranch(branch string) error
	// CreateOrphanedBranch creates a new branch that shares no commit history
	// with any other branch.
	CreateOrphanedBranch(branch string) error
	// HasDiffs returns a bool indicating whether the working directory currently
	// contains any differences from what's already at the head of the current
	// branch.
	HasDiffs() (bool, error)
	// GetDiffPaths returns a string slice indicating the paths, relative to the
	// root of the repository, of any new or modified files.
	GetDiffPaths() ([]string, error)
	// LastCommitID returns the ID (sha) of the most recent commit to the current
	// branch.
	LastCommitID() (string, error)
	// LocalBranchExists returns a bool indicating if the specified branch exists.
	LocalBranchExists(branch string) (bool, error)
	// CommitMessage returns the text of the most recent commit message associated
	// with the specified commit ID.
	CommitMessage(id string) (string, error)
	// CommitMessages returns a slice of commit messages starting with id1 and
	// ending with id2. The results exclude id1, but include id2.
	CommitMessages(id1, id2 string) ([]string, error)
	// Fetch fetches from the remote repository.
	Fetch() error
	// Pull fetches from the remote repository and merges the changes into the
	// current branch.
	Pull(branch string) error
	// Push pushes from the current branch to a remote branch by the same name.
	Push() error
	// RemoteBranchExists returns a bool indicating if the specified branch exists
	// in the remote repository.
	RemoteBranchExists(branch string) (bool, error)
	// Remotes returns a slice of strings representing the names of the remotes.
	Remotes() ([]string, error)
	// RemoteURL returns the URL of the the specified remote.
	RemoteURL(name string) (string, error)
	// ResetHard performs a hard reset.
	ResetHard() error
	// URL returns the remote URL of the repository.
	URL() string
	// WorkingDir returns an absolute path to the repository's working tree.
	WorkingDir() string
	// HomeDir returns an absolute path to the home directory of the system user
	// who has cloned this repo.
	HomeDir() string
}

// repo is an implementation of the Repo interface for interacting with a git
// repository.
type repo struct {
	url           string
	homeDir       string
	dir           string
	currentBranch string
	creds         RepoCredentials
}

// Clone produces a local clone of the remote git repository at the specified
// URL and returns an implementation of the Repo interface that is stateful and
// NOT suitable for use across multiple goroutines. This function will also
// perform any setup that is required for successfully authenticating to the
// remote repository.
func Clone(
	cloneURL string,
	repoCreds RepoCredentials,
) (Repo, error) {
	homeDir, err := os.MkdirTemp("", tmpPrefix)
	if err != nil {
		return nil, fmt.Errorf(
			"error creating home directory for repo %q: %w",
			cloneURL,
			err,
		)
	}
	r := &repo{
		url:     cloneURL,
		homeDir: homeDir,
		dir:     filepath.Join(homeDir, "repo"),
		creds:   repoCreds,
	}
	if err = r.setupAuth(repoCreds); err != nil {
		return nil, err
	}
	return r, r.clone()
}

// CopyRepo copies a git repository from the specified path to a temporary
// location. Repository credentials are required in order to authenticate to the
// remote repository, if any.
func CopyRepo(path string, repoCreds RepoCredentials) (Repo, error) {
	// Validate path is absolute
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("path %s is not absolute", path)
	}

	// Validate path exists and is a directory
	if fi, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("error checking if path %s exists: %w", path, err)
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("path %s is not a directory", path)
	}

	// Validate path is a git repository
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = path
	if _, err := libExec.Exec(cmd); err != nil {
		return nil, fmt.Errorf("path %s is not a git repository: %w", path, err)
	}

	homeDir, err := os.MkdirTemp("", tmpPrefix)
	if err != nil {
		return nil, fmt.Errorf(
			"error creating directory for copy of repo at %s: %w",
			path,
			err,
		)
	}

	r := &repo{
		homeDir: homeDir,
		dir:     filepath.Join(homeDir, "repo"),
	}

	// Copy from path to r.dir. Note: This obviously only works on *nix systems,
	// but we already advise that Kargo Render not be run outside of a Linux
	// container since its dependent on compatible versions of git, helm, and
	// kustomize binaries.
	// nolint: gosec
	if _, err = libExec.Exec(exec.Command("cp", "-r", path, r.dir)); err != nil {
		return nil, fmt.Errorf(
			"error copying repo from %s to %s: %w",
			path,
			r.dir,
			err,
		)
	}

	remotes, err := r.Remotes()
	if err != nil {
		return nil, err
	}
	if len(remotes) != 1 {
		return nil, fmt.Errorf(
			"expected exactly one remote in source repository; found %d",
			len(remotes),
		)
	}
	r.url, err = r.RemoteURL(remotes[0])
	if err != nil {
		return nil, err
	}

	if err = r.setupAuth(repoCreds); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *repo) AddAll() error {
	if _, err := libExec.Exec(r.buildCommand("add", ".")); err != nil {
		return fmt.Errorf("error staging changes for commit: %w", err)
	}
	return nil
}

func (r *repo) AddAllAndCommit(message string) error {
	if err := r.AddAll(); err != nil {
		return err
	}
	return r.Commit(message, nil)
}

func (r *repo) Clean() error {
	_, err := libExec.Exec(r.buildCommand("clean", "-fd"))
	if err != nil {
		return fmt.Errorf("error cleaning branch %q: %w", r.currentBranch, err)
	}
	return nil
}

func (r *repo) clone() error {
	r.currentBranch = "HEAD"
	cmd := r.buildCommand("clone", "--no-tags", r.url, r.dir)
	cmd.Dir = r.homeDir // Override the cmd.Dir that's set by r.buildCommand()
	if _, err := libExec.Exec(cmd); err != nil {
		return fmt.Errorf(
			"error cloning repo %q into %q: %w",
			r.url,
			r.dir,
			err,
		)
	}
	return nil
}

func (r *repo) Close() error {
	return os.RemoveAll(r.homeDir)
}

func (r *repo) Checkout(branch string) error {
	r.currentBranch = branch
	if _, err := libExec.Exec(r.buildCommand(
		"checkout",
		branch,
		// The next line makes it crystal clear to git that we're checking out
		// a branch. We need to do this because branch names can often resemble
		// paths within the repo.
		"--",
	)); err != nil {
		return fmt.Errorf(
			"error checking out branch %q from repo %q: %w",
			branch,
			r.url,
			err,
		)
	}
	return nil
}

type CommitOptions struct {
	AllowEmpty bool
}

func (r *repo) Commit(message string, opts *CommitOptions) error {
	if opts == nil {
		opts = &CommitOptions{}
	}
	cmdTokens := []string{"commit", "-m", message}
	if opts.AllowEmpty {
		cmdTokens = append(cmdTokens, "--allow-empty")
	}
	if _, err := libExec.Exec(r.buildCommand(cmdTokens...)); err != nil {
		return fmt.Errorf(
			"error committing changes to branch %q: %w",
			r.currentBranch,
			err,
		)
	}
	return nil
}

func (r *repo) CreateChildBranch(branch string) error {
	r.currentBranch = branch
	if _, err := libExec.Exec(r.buildCommand(
		"checkout",
		"-b",
		branch,
		// The next line makes it crystal clear to git that we're checking out
		// a branch. We need to do this because branch names can often resemble
		// paths within the repo.
		"--",
	)); err != nil {
		return fmt.Errorf(
			"error creating new branch %q for repo %q: %w",
			branch,
			r.url,
			err,
		)
	}
	return nil
}

func (r *repo) CreateOrphanedBranch(branch string) error {
	r.currentBranch = branch
	if _, err := libExec.Exec(r.buildCommand(
		"switch",
		"--orphan",
		branch,
		"--discard-changes",
	)); err != nil {
		return fmt.Errorf(
			"error creating orphaned branch %q for repo %q: %w",
			branch,
			r.url,
			err,
		)
	}
	return r.Clean()
}

func (r *repo) HasDiffs() (bool, error) {
	resBytes, err := libExec.Exec(r.buildCommand("status", "-s"))
	if err != nil {
		return false,
			fmt.Errorf("error checking status of branch %q: %w", r.currentBranch, err)
	}
	return len(resBytes) > 0, nil
}

func (r *repo) GetDiffPaths() ([]string, error) {
	resBytes, err := libExec.Exec(r.buildCommand("status", "-s"))
	if err != nil {
		return nil,
			fmt.Errorf("error checking status of branch %q: %w", r.currentBranch, err)
	}
	paths := []string{}
	scanner := bufio.NewScanner(bytes.NewReader(resBytes))
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		paths = append(
			paths,
			strings.SplitN(strings.TrimSpace(scanner.Text()), " ", 2)[1],
		)
	}
	return paths, nil
}

func (r *repo) LastCommitID() (string, error) {
	shaBytes, err := libExec.Exec(r.buildCommand("rev-parse", "HEAD"))
	if err != nil {
		return "", fmt.Errorf("error obtaining ID of last commit: %w", err)
	}
	return strings.TrimSpace(string(shaBytes)), nil
}

func (r *repo) LocalBranchExists(branch string) (bool, error) {
	resBytes, err := libExec.Exec(r.buildCommand(
		"branch",
		"--list",
		branch,
	))
	if err != nil {
		return false,
			fmt.Errorf("error checking for existence of local branch %q: %w", branch, err)
	}
	return strings.TrimSpace(
		strings.Replace(string(resBytes), "*", "", 1),
	) == branch, nil
}

func (r *repo) CommitMessage(id string) (string, error) {
	msgBytes, err := libExec.Exec(
		r.buildCommand("log", "-n", "1", "--pretty=format:%s", id),
	)
	if err != nil {
		return "",
			fmt.Errorf("error obtaining commit message for commit %q: %w", id, err)
	}
	return string(msgBytes), nil
}

func (r *repo) CommitMessages(id1, id2 string) ([]string, error) {
	allMsgBytes, err := libExec.Exec(r.buildCommand(
		"log",
		"--pretty=oneline",
		"--decorate-refs=",
		"--decorate-refs-exclude=",
		fmt.Sprintf("%s..%s", id1, id2),
	))
	if err != nil {
		return nil, fmt.Errorf(
			"error obtaining commit messages between commits %q and %q: %w",
			id1,
			id2,
			err,
		)
	}
	msgsBytes := bytes.Split(allMsgBytes, []byte("\n"))
	msgs := []string{}
	for _, msgBytes := range msgsBytes {
		msgStr := string(msgBytes)
		// There's usually a trailing newline in the result. We could just discard
		// the last line, but this feels more resilient against the admittedly
		// remote possibility that that could change one day.
		if strings.TrimSpace(msgStr) != "" {
			msgs = append(msgs, string(msgBytes))
		}
	}
	return msgs, nil
}

func (r *repo) Fetch() error {
	if _, err := libExec.Exec(r.buildCommand("fetch", RemoteOrigin)); err != nil {
		return fmt.Errorf("error fetching from remote repo %q: %w", r.url, err)
	}
	return nil
}

func (r *repo) Pull(branch string) error {
	if _, err :=
		libExec.Exec(r.buildCommand("pull", RemoteOrigin, branch)); err != nil {
		return fmt.Errorf(
			"error pulling branch %q from remote repo %q: %w",
			branch,
			r.url,
			err,
		)
	}
	return nil
}

func (r *repo) Push() error {
	if _, err :=
		libExec.Exec(r.buildCommand("push", RemoteOrigin, r.currentBranch)); err != nil {
		return fmt.Errorf("error pushing branch %q: %w", r.currentBranch, err)
	}
	return nil
}

func (r *repo) RemoteBranchExists(branch string) (bool, error) {
	if _, err := libExec.Exec(r.buildCommand(
		"ls-remote",
		"--heads",
		"--exit-code", // Return 2 if not found
		RemoteOrigin,
		branch,
	)); err != nil {
		if exitErr, ok := err.(*libExec.ExitError); ok && exitErr.ExitCode == 2 {
			// Branch does not exist
			return false, nil
		}
		return false, fmt.Errorf(
			"error checking for existence of branch %q in remote repo %q: %w",
			branch,
			r.url,
			err,
		)
	}
	return true, nil
}

func (r *repo) Remotes() ([]string, error) {
	resBytes, err := libExec.Exec(r.buildCommand("remote"))
	if err != nil {
		return nil, fmt.Errorf("error listing remotes for repo %q: %w", r.url, err)
	}
	return strings.Fields(string(resBytes)), nil
}

func (r *repo) RemoteURL(name string) (string, error) {
	resBytes, err := libExec.Exec(r.buildCommand("remote", "get-url", name))
	if err != nil {
		return "", fmt.Errorf(
			"error obtaining URL for remote %q of repo %q: %w",
			name,
			r.url,
			err,
		)
	}
	return strings.TrimSpace(string(resBytes)), nil
}

func (r *repo) ResetHard() error {
	if _, err :=
		libExec.Exec(r.buildCommand("reset", "--hard")); err != nil {
		return fmt.Errorf("error resetting branch working tree: %w", err)
	}
	return nil
}

func (r *repo) URL() string {
	return r.url
}

func (r *repo) HomeDir() string {
	return r.homeDir
}

func (r *repo) WorkingDir() string {
	return r.dir
}

// SetupAuth configures the git CLI for authentication using either SSH or the
// "store" (username/password-based) credential helper.
func (r *repo) setupAuth(repoCreds RepoCredentials) error {
	// Configure the git client
	cmd := r.buildCommand("config", "--global", "user.name", "Kargo Render")
	cmd.Dir = r.homeDir // Override the cmd.Dir that's set by r.buildCommand()
	if _, err := libExec.Exec(cmd); err != nil {
		return fmt.Errorf("error configuring git username: %w", err)
	}
	cmd =
		r.buildCommand("config", "--global", "user.email", "kargo-render@akuity.io")
	cmd.Dir = r.homeDir // Override the cmd.Dir that's set by r.buildCommand()
	if _, err := libExec.Exec(cmd); err != nil {
		return fmt.Errorf("error configuring git user email address: %w", err)
	}

	// If an SSH key was provided, use that.
	if repoCreds.SSHPrivateKey != "" {
		sshConfigPath := filepath.Join(r.homeDir, ".ssh", "config")
		// nolint: lll
		const sshConfig = "Host *\n  StrictHostKeyChecking no\n  UserKnownHostsFile=/dev/null"
		if err :=
			os.WriteFile(sshConfigPath, []byte(sshConfig), 0600); err != nil {
			return fmt.Errorf("error writing SSH config to %q: %w", sshConfigPath, err)
		}

		rsaKeyPath := filepath.Join(r.homeDir, ".ssh", "id_rsa")
		if err := os.WriteFile(
			rsaKeyPath,
			[]byte(repoCreds.SSHPrivateKey),
			0600,
		); err != nil {
			return fmt.Errorf("error writing SSH key to %q: %w", rsaKeyPath, err)
		}
		return nil // We're done
	}

	// If no password is specified, we're done'.
	if repoCreds.Password == "" {
		return nil
	}

	lowerURL := strings.ToLower(r.url)
	if strings.HasPrefix(lowerURL, "http://") || strings.HasPrefix(lowerURL, "https://") {
		u, err := url.Parse(r.url)
		if err != nil {
			return fmt.Errorf("error parsing URL %q: %w", r.url, err)
		}
		u.User = url.User(repoCreds.Username)
		r.url = u.String()

	}
	return nil
}

func (r *repo) buildCommand(arg ...string) *exec.Cmd {
	cmd := exec.Command("git", arg...)
	homeEnvVar := fmt.Sprintf("HOME=%s", r.homeDir)
	if cmd.Env == nil {
		cmd.Env = []string{homeEnvVar}
	} else {
		cmd.Env = append(cmd.Env, homeEnvVar)
	}
	if r.creds.Password != "" {
		cmd.Env = append(
			cmd.Env,
			"GIT_ASKPASS=/usr/local/bin/credential-helper",
			fmt.Sprintf("GIT_PASSWORD=%s", r.creds.Password),
		)
	}
	cmd.Dir = r.dir
	return cmd
}
