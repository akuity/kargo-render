package bookkeeper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/go-github/v47/github"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/akuityio/bookkeeper/internal/config"
	"github.com/akuityio/bookkeeper/internal/git"
	"github.com/akuityio/bookkeeper/internal/helm"
	"github.com/akuityio/bookkeeper/internal/kustomize"
	"github.com/akuityio/bookkeeper/internal/metadata"
	"github.com/akuityio/bookkeeper/internal/ytt"
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

	// Pre-render
	preRenderedBytes, err := s.preRender(repo, branchConfig, req)
	if err != nil {
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

	// Ensure the .bookkeeper directory exists and is set up correctly
	bkDir := filepath.Join(repo.WorkingDir(), ".bookkeeper")
	if err = kustomize.EnsureBookkeeperDir(bkDir); err != nil {
		return res, errors.Wrapf(
			err,
			"error setting up .bookkeeper directory %q",
			bkDir,
		)
	}

	// Write branch metadata
	if err = metadata.WriteTargetBranchMetadata(
		metadata.TargetBranchMetadata{
			SourceCommit: sourceCommitID,
		},
		repo.WorkingDir(),
	); err != nil {
		return res, errors.Wrap(err, "writing branch metadata")
	}

	// Write the pre-rendered config to a temporary location
	preRenderedPath := filepath.Join(bkDir, "ephemeral.yaml")
	// nolint: gosec
	if err = os.WriteFile(preRenderedPath, preRenderedBytes, 0644); err != nil {
		return res, errors.Wrapf(
			err,
			"error writing ephemeral, pre-rendered configuration to %q",
			preRenderedPath,
		)
	}
	logger.Debug("wrote pre-rendered configuration")

	// Deal with new images if any were specified
	for _, image := range req.Images {
		if err = kustomize.SetImage(bkDir, image); err != nil {
			return res, errors.Wrapf(
				err,
				"error setting image in pre-render directory %q",
				bkDir,
			)
		}
	}

	// Now take everything the last mile with kustomize and write the
	// fully-rendered config to the commit branch...

	// Last mile rendering
	fullyRenderedBytes, err := kustomize.Render(bkDir)
	if err != nil {
		return res, errors.Wrapf(
			err,
			"error rendering configuration from %q",
			bkDir,
		)
	}

	// Write the new fully-rendered config to the root of the repo
	allPath := filepath.Join(repo.WorkingDir(), "all.yaml")
	// nolint: gosec
	if err = os.WriteFile(allPath, fullyRenderedBytes, 0644); err != nil {
		return res, errors.Wrapf(
			err,
			"error writing fully-rendered configuration to %q",
			allPath,
		)
	}
	logger.Debug("wrote fully-rendered configuration")

	// Delete the ephemeral, pre-rendered configuration
	if err = os.Remove(preRenderedPath); err != nil {
		return res, errors.Wrapf(
			err,
			"error deleting ephemeral, pre-rendered configuration from %q",
			preRenderedPath,
		)
	}

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

		if !branchConfig.OpenPR {
			return req.TargetBranch, nil
		}

		// If we get to here, we're supposed to be opening a PR instead of
		// committing directly to the target branch, so we should create and check
		// out a new child of the target branch.
		commitBranch := fmt.Sprintf("bookkeeper/%s", uuid.NewV4().String())
		if err = repo.CreateChildBranch(commitBranch); err != nil {
			return "", errors.Wrap(err, "error creating child of target branch")
		}
		logger.Debug("created child of target branch")
		return commitBranch, nil
	}

	// If we get to here, the target branch doesn't exist and we must create a
	// brand new orphaned branch.
	if err = repo.CreateOrphanedBranch(req.TargetBranch); err != nil {
		return "", errors.Wrap(err, "error creating orphaned target branch")
	}
	logger.Debug("created orphaned target branch")
	if err := repo.Reset(); err != nil {
		return "", errors.Wrap(err, "error resetting repo")
	}

	return req.TargetBranch, errors.Wrap(repo.Clean(), "error cleaning repo")
}

func (s *service) preRender(
	repo git.Repo,
	branchConfig config.BranchConfig,
	req RenderRequest,
) ([]byte, error) {
	baseDir := filepath.Join(repo.WorkingDir(), "base")
	envDir := filepath.Join(repo.WorkingDir(), req.TargetBranch)

	// Use the caller's preferred config management tool for pre-rendering.
	var preRenderedBytes []byte
	var err error
	if branchConfig.ConfigManagement.Helm != nil {
		preRenderedBytes, err = helm.Render(
			branchConfig.ConfigManagement.Helm.ReleaseName,
			baseDir,
			envDir,
		)
	} else if branchConfig.ConfigManagement.Kustomize != nil {
		preRenderedBytes, err = kustomize.Render(envDir)
	} else if branchConfig.ConfigManagement.Ytt != nil {
		preRenderedBytes, err = ytt.Render(baseDir, envDir)
	} else {
		preRenderedBytes, err = kustomize.Render(envDir)
	}

	return preRenderedBytes, err
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
	// Use the source commit's message as a starting point
	commitMsg, err := repo.CommitMessage(sourceCommitID)
	if err != nil {
		return "", errors.Wrapf(
			err,
			"error getting commit message for commit %q",
			sourceCommitID,
		)
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
