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
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"

	"github.com/akuityio/bookkeeper/internal/config"
	"github.com/akuityio/bookkeeper/internal/git"
	"github.com/akuityio/bookkeeper/internal/github"
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

	logger := s.logger.WithField("request", req.id)
	startEndLogger := logger.WithFields(log.Fields{
		"repo":         req.RepoURL,
		"targetBranch": req.TargetBranch,
	})

	res := RenderResponse{}

	var err error
	if req, err = validateAndCanonicalizeRequest(req); err != nil {
		return res, err
	}
	startEndLogger.Debug("validated rendering request")

	startEndLogger.Debug("starting configuration rendering")

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

	srcCommitID, err := checkoutSourceCommit(repo, req)
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
	logger.Debug("completed last-mile rendering")

	// Switch to the commit branch. The commit branch might be the target branch,
	// but if branchConfig.OpenPR is true, it could be a new child of the target
	// branch.
	commitBranch, err := s.switchToCommitBranch(repo, branchConfig, req)
	if err != nil {
		return res, errors.Wrap(err, "error switching to target branch")
	}

	if err = rmYAML(repo.WorkingDir()); err != nil {
		return res, errors.Wrap(err, "error cleaning commit branch")
	}

	// Load any existing metadata now because we'll want to use it and it won't
	// be long before we potentially overwrite it. Get it while the gettin's good.
	oldMetadata, err := metadata.LoadTargetBranchMetadata(repo.WorkingDir())
	if err != nil {
		return res, errors.Wrap(err, "error loading branch metadata")
	}

	// Write branch metadata
	if err = metadata.WriteTargetBranchMetadata(
		metadata.TargetBranchMetadata{
			SourceCommit:       srcCommitID,
			ImageSubstitutions: req.Images,
		},
		repo.WorkingDir(),
	); err != nil {
		return res, errors.Wrap(err, "error writing branch metadata")
	}
	logger.WithField("sourceCommit", srcCommitID).Debug("wrote branch metadata")

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

	var prevSrcCommitID string
	if oldMetadata != nil {
		prevSrcCommitID = oldMetadata.SourceCommit
	}
	commitMsg, err := buildCommitMessage(repo, req, prevSrcCommitID, srcCommitID)
	if err != nil {
		return res, err
	}
	logger.Debug("prepared commit message")

	// Commit the fully-rendered configuration
	if err = repo.AddAllAndCommit(commitMsg); err != nil {
		return res, errors.Wrapf(
			err,
			"error committing fully-rendered configuration",
		)
	}
	commitID, err := repo.LastCommitID()
	if err != nil {
		return res, errors.Wrap(
			err,
			"error getting last commit ID from the commit branch",
		)
	}
	logger.WithFields(log.Fields{
		"commitBranch": commitBranch,
		"commitID":     commitID,
	}).Debug("committed fully-rendered configuration")

	// Push the fully-rendered configuration to the remote commit branch
	if err = repo.Push(); err != nil {
		return res, errors.Wrap(
			err,
			"error pushing fully-rendered configuration",
		)
	}
	logger.WithField("commitBranch", commitBranch).
		Debug("pushed fully-rendered configuration")

	// Open a PR if requested
	if branchConfig.OpenPR {
		commitMsgParts := strings.SplitN(commitMsg, "\n", 2)
		// PR title is just the first line of the commit message
		prTitle := fmt.Sprintf("%s --> %s", commitMsgParts[0], req.TargetBranch)
		// PR body is just the first line of the commit message
		var prBody string
		if len(commitMsgParts) == 2 {
			prBody = strings.TrimSpace(commitMsgParts[1])
		}
		// TODO: Support git providers other than GitHub
		if res.PullRequestURL, err = github.OpenPR(
			ctx,
			req.RepoURL,
			prTitle,
			prBody,
			req.TargetBranch,
			commitBranch,
			git.RepoCredentials{
				Username: req.RepoCreds.Username,
				Password: req.RepoCreds.Password,
			},
		); err != nil {
			return res,
				errors.Wrap(err, "error opening pull request to the target branch")
		}
		logger.WithField("prURL", res.PullRequestURL).Debug("opened PR")
		res.ActionTaken = ActionTakenOpenedPR
	} else {
		res.ActionTaken = ActionTakenPushedDirectly
		res.CommitID = commitID
	}

	startEndLogger.Debug("completed configuration rendering")

	return res, nil
}

func (s *service) switchToCommitBranch(
	repo git.Repo,
	branchConfig config.BranchConfig,
	req RenderRequest,
) (string, error) {
	logger := s.logger.WithField("request", req.id)
	targetBranchLogger := logger.WithField("targetBranch", req.TargetBranch)

	// Check if the target branch exists on the remote
	targetBranchExists, err := repo.RemoteBranchExists(req.TargetBranch)
	if err != nil {
		return "", errors.Wrap(err, "error checking for existence of target branch")
	}

	if targetBranchExists {
		targetBranchLogger.Debug("target branch exists on remote")
		if err = repo.Checkout(req.TargetBranch); err != nil {
			return "", errors.Wrap(err, "error checking out target branch")
		}
		targetBranchLogger.Debug("checked out target branch")
	} else {
		targetBranchLogger.Debug("target branch does not exist on remote")
		if err = repo.CreateOrphanedBranch(req.TargetBranch); err != nil {
			return "", errors.Wrap(err, "error creating new target branch")
		}
		targetBranchLogger.Debug("created target branch")
		bkDir := filepath.Join(repo.WorkingDir(), ".bookkeeper")
		if err = os.MkdirAll(bkDir, 0755); err != nil {
			return "",
				errors.Wrapf(err, "error ensuring existence of directory %q", bkDir)
		}
		logger.Debug("created .bookkeeper/ directory")
		if err = metadata.WriteTargetBranchMetadata(
			metadata.TargetBranchMetadata{},
			repo.WorkingDir(),
		); err != nil {
			return "", errors.Wrap(err, "error writing blank target branch metadata")
		}
		targetBranchLogger.Debug("wrote blank target branch metadata")
		if err = repo.AddAllAndCommit("Initial commit"); err != nil {
			return "",
				errors.Wrap(err, "error making initial commit to new target branch")
		}
		targetBranchLogger.Debug("made initial commit to new target branch")
		if err = repo.Push(); err != nil {
			return "",
				errors.Wrap(err, "error pushing new target branch to remote")
		}
		targetBranchLogger.Debug("pushed new target branch to remote")
	}

	if !branchConfig.OpenPR {
		targetBranchLogger.Debug(
			"changes will be written directly to the target branch",
		)
		return req.TargetBranch, nil
	}

	targetBranchLogger.Debug("changes will be PR'ed to the target branch")

	// If we get to here, we're supposed to be opening a PR instead of
	// committing directly to the target branch, so we should create and check
	// out a new child of the target branch.
	commitBranch := fmt.Sprintf("bookkeeper/%s", req.id)
	if err = repo.CreateChildBranch(commitBranch); err != nil {
		return "", errors.Wrap(err, "error creating child of target branch")
	}
	targetBranchLogger.WithField("commitBranch", commitBranch).
		Debug("created commit branch")
	return commitBranch, nil
}

func validateAndCanonicalizeRequest(req RenderRequest) (RenderRequest, error) {
	req.RepoURL = strings.TrimSpace(req.RepoURL)
	if req.RepoURL == "" {
		return req, errors.New("validation failed: RepoURL is a required field")
	}
	repoURLRegex :=
		regexp.MustCompile(`^(?:(?:(?:https?://)|(?:git@))[\w:/\-\.\?=@&%]+)$`)
	if !repoURLRegex.MatchString(req.RepoURL) {
		return req, errors.Errorf(
			"validation failed: RepoURL %q does not appear to be a valid git "+
				"repository URL",
			req.RepoURL,
		)
	}

	// TODO: Should this be required? I think some git providers don't require
	// this if the password is a bearer token -- e.g. such as in the case of a
	// GitHub personal access token.
	req.RepoCreds.Username = strings.TrimSpace(req.RepoCreds.Username)
	req.RepoCreds.Password = strings.TrimSpace(req.RepoCreds.Password)
	if req.RepoCreds.Password == "" {
		return req, errors.New(
			"validation failed: RepoCreds.Password is a required field",
		)
	}

	req.Commit = strings.TrimSpace(req.Commit)
	if req.Commit != "" {
		shaRegex := regexp.MustCompile(`^[a-fA-F0-9]{8,40}$`)
		if !shaRegex.MatchString(req.Commit) {
			return req, errors.Errorf(
				"validation failed: Commit %q does not appear to be a valid commit ID",
				req.Commit,
			)
		}
	}

	req.TargetBranch = strings.TrimSpace(req.TargetBranch)
	if req.TargetBranch == "" {
		return req,
			errors.New("validation failed: TargetBranch is a required field")
	}
	targetBranchRegex := regexp.MustCompile(`^(?:[\w\.-]+\/?)*\w$`)
	if !targetBranchRegex.MatchString(req.TargetBranch) {
		return req, errors.Errorf(
			"validation failed: TargetBranch %q is an invalid branch name",
			req.TargetBranch,
		)
	}
	req.TargetBranch = strings.TrimPrefix(req.TargetBranch, "refs/heads/")

	if len(req.Images) > 0 {
		for i := range req.Images {
			req.Images[i] = strings.TrimSpace(req.Images[i])
			if req.Images[i] == "" {
				return req, errors.New(
					"validation failed: Images must not contain any empty strings",
				)
			}
		}
	}

	req.CommitMessage = strings.TrimSpace(req.CommitMessage)

	return req, nil
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

	// If we get to here, we should follow the branch metadata back to the real
	// source commit
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
	prevSrcCommitID string,
	srcCommitID string,
) (string, error) {
	var commitMsg string
	if req.CommitMessage != "" {
		commitMsg = req.CommitMessage
	} else {
		// Use the source commit's message as a starting point
		var err error
		if commitMsg, err = repo.CommitMessage(srcCommitID); err != nil {
			return "", errors.Wrapf(
				err,
				"error getting commit message for commit %q",
				srcCommitID,
			)
		}
	}

	// Add the source commit's ID
	formattedCommitMsg := fmt.Sprintf(
		"%s\n\nBookkeeper created this commit by rendering configuration from %s",
		commitMsg,
		srcCommitID,
	)

	// Find all recent commits
	var memberCommitMsgs []string
	if prevSrcCommitID != "" {
		// Add info about member commits
		formattedCommitMsg = fmt.Sprintf(
			"%s\n\nThis includes the following changes (newest to oldest):",
			formattedCommitMsg,
		)
		var err error
		if memberCommitMsgs, err =
			repo.CommitMessages(prevSrcCommitID, srcCommitID); err != nil {
			return "", errors.Wrapf(
				err,
				"error getting commit messages between commit %q and %q",
				prevSrcCommitID,
				srcCommitID,
			)
		}
		for _, memberCommitMsg := range memberCommitMsgs {
			formattedCommitMsg = fmt.Sprintf(
				"%s\n  * %s",
				formattedCommitMsg,
				memberCommitMsg,
			)
		}
	}

	if len(req.Images) != 0 {
		formattedCommitMsg = fmt.Sprintf(
			"%s\n\nBookkeeper also incorporated the following new images into this "+
				"commit:\n",
			formattedCommitMsg,
		)
		for _, image := range req.Images {
			formattedCommitMsg = fmt.Sprintf(
				"%s\n  * %s",
				formattedCommitMsg,
				image,
			)
		}
	}

	return formattedCommitMsg, nil
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
