package bookkeeper

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/google/go-github/v47/github"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/akuityio/bookkeeper/internal/config"
	"github.com/akuityio/bookkeeper/internal/git"
	"github.com/akuityio/bookkeeper/internal/metadata"
)

type ServiceOptions struct {
	LogLevel LogLevel
}

// Service is an interface for components that can handle bookkeeping requests.
// Implementations of this interface are transport-agnostic.
type Service interface {
	// RenderConfig handles a bookkeeping request.
	RenderConfig(context.Context, RenderRequest) (RenderResponse, error)
}

type service struct {
	logger *log.Logger
}

// NewService returns an implementation of the Service interface for
// handling bookkeeping requests.
func NewService(opts *ServiceOptions) Service {
	if opts == nil {
		opts = &ServiceOptions{}
	}
	if opts.LogLevel == 0 {
		opts.LogLevel = LogLevelInfo
	}
	logger := log.New()
	logger.SetLevel(log.Level(opts.LogLevel))
	return &service{
		logger: logger,
	}
}

// nolint: gocyclo
func (s *service) RenderConfig(
	ctx context.Context,
	req RenderRequest,
) (RenderResponse, error) {
	req.id = uuid.NewV4().String()

	logger := s.logger.WithFields(
		log.Fields{
			"request":      req.id,
			"repo":         req.RepoURL,
			"targetBranch": req.TargetBranch,
		},
	)

	res := RenderResponse{}

	repo, err := git.Clone(
		ctx,
		req.RepoURL,
		git.RepoCredentials{
			SSHPrivateKey: req.RepoCreds.SSHPrivateKey,
			Username:      req.RepoCreds.Username,
			Password:      req.RepoCreds.Password,
		},
	)
	if err != err {
		return res, errors.Wrap(err, "error cloning remote repository")
	}
	defer repo.Close()

	sourceCommitID, err := checkoutSourceCommit(repo, req)
	if err != nil {
		return res, err
	}

	repoConfig, err := config.LoadRepoConfig(repo.WorkingDir())
	if err != nil {
		return res,
			errors.Wrap(err, "error loading Bookkeeper configuration from repo")
	}
	branchConfig := repoConfig.GetBranchConfig(req.TargetBranch)

	// Render
	preRenderedBytes, err := s.preRender(repo, branchConfig, req)
	if err != nil {
		return res, err
	}
	fullyRenderedBytes, err := s.renderLastMile(repo, req, preRenderedBytes)
	if err != nil {
		// TODO: Wrap this error
		return res, err
	}

	// Switch to the commit branch. The commit branch might be the target branch,
	// but if branchConfig.OpenPR is true, it could be a new child of the target
	// branch.
	commitBranch, err := s.switchToCommitBranch(repo, branchConfig, req)
	if err != nil {
		return res, errors.Wrap(err, "error switching to target branch")
	}
	logger = logger.WithFields(log.Fields{
		"commitBranch": commitBranch,
	})

	if err = rmYAML(repo.WorkingDir()); err != nil {
		return res, errors.Wrap(err, "error cleaning commit branch")
	}

	// Ensure the .bookkeeper directory exists
	bkDir := filepath.Join(repo.WorkingDir(), ".bookkeeper")
	if err = os.MkdirAll(bkDir, 0755); err != nil {
		return res,
			errors.Wrapf(err, "error ensuring existence of directory %q", bkDir)
	}

	// Write branch metadata
	if err = metadata.WriteTargetBranchMetadata(
		metadata.TargetBranchMetadata{
			SourceCommit:       sourceCommitID,
			ImageSubstitutions: req.Images,
		},
		repo.WorkingDir(),
	); err != nil {
		return res, errors.Wrap(err, "writing branch metadata")
	}

	// Write the new fully-rendered config to the root of the repo
	if err = writeFiles(repo.WorkingDir(), fullyRenderedBytes); err != nil {
		return res, err
	}
	logger.Debug("wrote fully-rendered configuration")

	// Before committing, check if we actually have any diffs from the head of
	// this branch. We'd have an error if we tried to commit with no diffs!
	hasDiffs, err := repo.HasDiffs()
	if err != nil {
		return res, errors.Wrap(err, "error checking for diffs")
	}
	if !hasDiffs {
		logger.Debug(
			"fully-rendered configuration does not differ from the head of the " +
				"commit branch; no further action is required",
		)
		res.ActionTaken = ActionTakenNone
		return res, nil
	}

	commitMsg, err := buildCommitMessage(repo, req, sourceCommitID)
	if err != nil {
		return res, err
	}

	// Commit the fully-rendered configuration
	if err = repo.AddAllAndCommit(commitMsg); err != nil {
		return res, errors.Wrapf(
			err,
			"error committing fully-rendered configuration",
		)
	}
	logger.Debug("committed fully-rendered configuration")

	// Push the fully-rendered configuration to the remote commit branch
	if err = repo.Push(); err != nil {
		return res, errors.Wrap(
			err,
			"error pushing fully-rendered configuration",
		)
	}
	logger.Debug("pushed fully-rendered configuration")

	// Open a PR if requested
	//
	// TODO: Support git providers other than GitHub
	//
	// TODO: Move this into its own github package
	if commitBranch != req.TargetBranch {
		var owner, repo string
		if owner, repo, err = parseGitHubURL(req.RepoURL); err != nil {
			return res, err
		}
		githubClient := github.NewClient(
			oauth2.NewClient(
				ctx,
				oauth2.StaticTokenSource(
					&oauth2.Token{AccessToken: req.RepoCreds.Password},
				),
			),
		)
		var pr *github.PullRequest
		if pr, _, err = githubClient.PullRequests.Create(
			ctx,
			owner,
			repo,
			&github.NewPullRequest{
				// PR title is just the first line of the commit message
				Title:               github.String(strings.Split(commitMsg, "\n")[0]),
				Base:                github.String(req.TargetBranch),
				Head:                github.String(commitBranch),
				MaintainerCanModify: github.Bool(false),
			},
		); err != nil {
			return res,
				errors.Wrap(err, "error opening pull request to the target branch")
		}
		res.ActionTaken = ActionTakenOpenedPR
		res.PullRequestURL = *pr.HTMLURL
		return res, nil
	}

	// Get the ID of the last commit on the target branch
	res.ActionTaken = ActionTakenPushedDirectly
	if res.CommitID, err = repo.LastCommitID(); err != nil {
		return res, errors.Wrap(
			err,
			"error getting last commit ID from the target branch",
		)
	}
	logger.Debug("obtained sha of last commit")

	return res, nil
}

func (s *service) switchToCommitBranch(
	repo git.Repo,
	branchConfig config.BranchConfig,
	req RenderRequest,
) (string, error) {
	logger := s.logger.WithFields(
		log.Fields{
			"request":      req.id,
			"repo":         req.RepoURL,
			"targetBranch": req.TargetBranch,
		},
	)

	// Check if the target branch exists on the remote
	targetBranchExists, err := repo.RemoteBranchExists(req.TargetBranch)
	if err != nil {
		return "", errors.Wrap(err, "error checking for existence of target branch")
	}

	if targetBranchExists {
		logger.Debug("target branch exists on remote")
		if err = repo.Checkout(req.TargetBranch); err != nil {
			return "", errors.Wrap(err, "error checking out target branch")
		}
		logger.Debug("checked out target branch")
	} else {
		logger.Debug("target branch does not exist on remote")
		if err = repo.CreateOrphanedBranch(req.TargetBranch); err != nil {
			return "", errors.Wrap(err, "error creating new target branch")
		}
		logger.Debug("created target branch")
		if _, err = os.Create(
			filepath.Join(repo.WorkingDir(), ".keep"),
		); err != nil {
			return "",
				errors.Wrap(err, "error writing .keep file to new target branch")
		}
		logger.Debug("wrote .keep file")
		if err = repo.AddAllAndCommit("Initial commit"); err != nil {
			return "",
				errors.Wrap(err, "error making initial commit to new target branch")
		}
		logger.Debug("made initial commit to new target branch")
		if err = repo.Push(); err != nil {
			return "",
				errors.Wrap(err, "error pushing new target branch to remote")
		}
		logger.Debug("pushed new target branch to remote")
	}

	if !branchConfig.OpenPR {
		return req.TargetBranch, nil
	}

	// If we get to here, we're supposed to be opening a PR instead of
	// committing directly to the target branch, so we should create and check
	// out a new child of the target branch.
	commitBranch := fmt.Sprintf("bookkeeper/%s", req.id)
	if err = repo.CreateChildBranch(commitBranch); err != nil {
		return "", errors.Wrap(err, "error creating child of target branch")
	}
	logger.Debug("created child of target branch")
	return commitBranch, nil
}

func parseGitHubURL(url string) (string, string, error) {
	regex := regexp.MustCompile(`^https\://github\.com/([\w-]+)/([\w-]+).*`)
	parts := regex.FindStringSubmatch(url)
	if len(parts) != 3 {
		return "", "", errors.Errorf("error parsing github repository URL %q", url)
	}
	return parts[1], parts[2], nil
}

// checkoutSourceCommit examines a RenderRequest and determines if it is
// requesting to render configuration from a specific commit. If not, it returns
// the ID of the most recent commit.
//
// THIS ASSUMES THAT THIS FUNCTION IS ONLY CALLED IMMEDIATELY AFTER CLONING THE
// REPOSITORY, MEANING THE HEAD OF THE CURRENT BRANCH IS THE HEAD OF THE
// REPOSITORY'S DEFAULT BRANCH.
//
// If the RenderRequest specifies a commit, it is checked out. If metadata in
// that commit indicates it was, itself, rendered from source configuration
// in another commit, this function "follows" that reference back to the
// original commit.
func checkoutSourceCommit(repo git.Repo, req RenderRequest) (string, error) {
	// If no commit ID was specified return the commit ID at the head of this
	// branch and we're done.
	if req.Commit == "" {
		sourceCommitID, err := repo.LastCommitID()
		return sourceCommitID,
			errors.Wrap(err, "error getting last commit ID from the default branch")
	}

	// Check out the specified commit
	if err := repo.Checkout(req.Commit); err != nil {
		return "", errors.Wrapf(err, "error checking out %q", req.Commit)
	}

	// Try to load target branch metadata
	targetBranchMetadata, err :=
		metadata.LoadTargetBranchMetadata(repo.WorkingDir())
	if err != nil {
		return "",
			errors.Wrapf(err, "error loading branch metadata from %q", req.Commit)
	}

	// If we got no branch metadata, then we assume we're already sitting on the
	// source commit.
	if targetBranchMetadata == nil {
		return req.Commit, nil
	}

	// Follow the branch metadata back to the real source commit
	err = repo.Checkout(targetBranchMetadata.SourceCommit)
	return targetBranchMetadata.SourceCommit, errors.Wrapf(
		err,
		"error checking out %q",
		targetBranchMetadata.SourceCommit,
	)
}

// buildCommitMessage builds a commit message for rendered configuration being
// written to a target branch by using the source commit's own commit message
// as a starting point. The message is then augmented with details about where
// Bookkeeper rendered it from (the source commit) and any image substitutions
// Bookkeeper made per the RenderRequest.
func buildCommitMessage(
	repo git.Repo,
	req RenderRequest,
	sourceCommitID string,
) (string, error) {
	var commitMsg string
	if req.CommitMessage != "" {
		commitMsg = req.CommitMessage
	} else {
		// Use the source commit's message as a starting point
		var err error
		if commitMsg, err = repo.CommitMessage(sourceCommitID); err != nil {
			return "", errors.Wrapf(
				err,
				"error getting commit message for commit %q",
				sourceCommitID,
			)
		}
	}

	// Add the source commit's ID
	commitMsg = fmt.Sprintf(
		"%s\n\nBookkeeper created this commit by rendering configuration from %s",
		commitMsg,
		sourceCommitID,
	)

	if len(req.Images) != 0 {
		commitMsg = fmt.Sprintf(
			"%s\n\nBookkeeper incorporated the following new images into this "+
				"commit:\n",
			commitMsg,
		)
		for _, image := range req.Images {
			commitMsg = fmt.Sprintf(
				"%s\n  * %s",
				commitMsg,
				image,
			)
		}
	}

	return commitMsg, nil
}

func writeFiles(dir string, yamlBytes []byte) error {
	resourcesBytes := bytes.Split(yamlBytes, []byte("---\n"))
	for _, resourceBytes := range resourcesBytes {
		resource := struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		}{}
		if err := yaml.Unmarshal(resourceBytes, &resource); err != nil {
			return errors.Wrap(err, "error unmarshaling resource")
		}
		fileName := filepath.Join(
			dir,
			fmt.Sprintf(
				"%s-%s.yaml",
				strings.ToLower(resource.Metadata.Name),
				strings.ToLower(resource.Kind),
			),
		)
		// nolint: gosec
		if err := os.WriteFile(fileName, resourceBytes, 0644); err != nil {
			return errors.Wrapf(
				err,
				"error writing fully-rendered configuration to %q",
				fileName,
			)
		}
	}
	return nil
}

func rmYAML(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return err
	}
	for _, file := range files {
		if err = os.Remove(file); err != nil {
			return err
		}
	}
	return nil
}
