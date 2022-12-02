package bookkeeper

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"

	"github.com/akuityio/bookkeeper/internal/git"
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

	rc := renderRequestContext{
		logger:  logger,
		request: req,
	}

	if rc.repo, err = git.Clone(
		ctx,
		rc.request.RepoURL,
		git.RepoCredentials{
			SSHPrivateKey: rc.request.RepoCreds.SSHPrivateKey,
			Username:      rc.request.RepoCreds.Username,
			Password:      rc.request.RepoCreds.Password,
		},
	); err != err {
		return res, errors.Wrap(err, "error cloning remote repository")
	}
	defer rc.repo.Close()

	// TODO: Add some logging to this block
	if rc.request.Commit == "" {
		if rc.source.commit, err = rc.repo.LastCommitID(); err != nil {
			return res, errors.Wrap(err, "error getting last commit ID")
		}
	} else {
		if err = rc.repo.Checkout(rc.request.Commit); err != nil {
			return res, errors.Wrapf(err, "error checking out %q", rc.request.Commit)
		}
		if rc.intermediate.branchMetadata, err =
			loadBranchMetadata(rc.repo.WorkingDir()); err != nil {
			return res, errors.Wrap(err, "error loading branch metadata")
		}
		if rc.intermediate.branchMetadata == nil {
			// We're not on a target branch. We assume we're on the default branch.
			rc.source.commit = rc.request.Commit
		} else {
			// Follow the branch metadata back to the real source commit
			if err = rc.repo.Checkout(
				rc.intermediate.branchMetadata.SourceCommit,
			); err != nil {
				return res, errors.Wrapf(
					err,
					"error checking out %q",
					rc.intermediate.branchMetadata.SourceCommit,
				)
			}
			rc.source.commit = rc.intermediate.branchMetadata.SourceCommit
		}
	}

	repoConfig, err := loadRepoConfig(rc.repo.WorkingDir())
	if err != nil {
		return res,
			errors.Wrap(err, "error loading Bookkeeper configuration from repo")
	}
	rc.target.branchConfig = repoConfig.getBranchConfig(rc.request.TargetBranch)

	if rc.target.prerenderedConfig, err = preRender(rc); err != nil {
		return res, errors.Wrap(err, "error pre-rendering configuration")
	}

	if err = switchToTargetBranch(rc); err != nil {
		return res, errors.Wrap(err, "error switching to target branch")
	}

	oldTargetBranchMetadata, err := loadBranchMetadata(rc.repo.WorkingDir())
	if err != nil {
		return res, errors.Wrap(err, "error loading branch metadata")
	}
	rc.target.oldBranchMetadata = *oldTargetBranchMetadata

	if rc.target.commit.branch, err = switchToCommitBranch(rc); err != nil {
		return res, errors.Wrap(err, "error switching to commit branch")
	}

	rc.target.newBranchMetadata.SourceCommit = rc.source.commit
	if rc.target.newBranchMetadata.ImageSubstitutions,
		rc.target.renderedConfig,
		err =
		renderLastMile(rc); err != nil {
		return res, errors.Wrap(err, "error in last-mile configuration rendering")
	}
	logger.Debug("completed last-mile rendering")

	// Write branch metadata
	if err = writeBranchMetadata(
		rc.target.newBranchMetadata,
		rc.repo.WorkingDir(),
	); err != nil {
		return res, errors.Wrap(err, "error writing branch metadata")
	}
	logger.WithField("sourceCommit", rc.source.commit).
		Debug("wrote branch metadata")

	// Write the new fully-rendered config to the root of the repo
	if err =
		writeFiles(rc.repo.WorkingDir(), rc.target.renderedConfig); err != nil {
		return res, err
	}
	logger.Debug("wrote fully-rendered configuration")

	// Before committing, check if we actually have any diffs from the head of
	// this branch. We'd have an error if we tried to commit with no diffs!
	hasDiffs, err := rc.repo.HasDiffs()
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

	if rc.target.commit.message, err = buildCommitMessage(rc); err != nil {
		return res, err
	}
	logger.Debug("prepared commit message")

	// Commit the fully-rendered configuration
	if err = rc.repo.AddAllAndCommit(rc.target.commit.message); err != nil {
		return res, errors.Wrapf(
			err,
			"error committing fully-rendered configuration",
		)
	}
	if rc.target.commit.id, err = rc.repo.LastCommitID(); err != nil {
		return res, errors.Wrap(
			err,
			"error getting last commit ID from the commit branch",
		)
	}
	logger.WithFields(log.Fields{
		"commitBranch": rc.target.commit.branch,
		"commitID":     rc.target.commit.id,
	}).Debug("committed fully-rendered configuration")

	// Push the fully-rendered configuration to the remote commit branch
	if err = rc.repo.Push(); err != nil {
		return res, errors.Wrap(
			err,
			"error pushing fully-rendered configuration",
		)
	}
	logger.WithField("commitBranch", rc.target.commit.branch).
		Debug("pushed fully-rendered configuration")

	// Open a PR if requested
	if rc.target.branchConfig.OpenPR {
		if res.PullRequestURL, err = openPR(ctx, rc); err != nil {
			return res,
				errors.Wrap(err, "error opening pull request to the target branch")
		}
		logger.WithField("prURL", res.PullRequestURL).Debug("opened PR")
		res.ActionTaken = ActionTakenOpenedPR
	} else {
		res.ActionTaken = ActionTakenPushedDirectly
		res.CommitID = rc.target.commit.id
	}

	startEndLogger.Debug("completed configuration rendering")

	return res, nil
}

// buildCommitMessage builds a commit message for rendered configuration being
// written to a target branch by using the source commit's own commit message
// as a starting point. The message is then augmented with details about where
// Bookkeeper rendered it from (the source commit) and any image substitutions
// Bookkeeper made per the RenderRequest.
func buildCommitMessage(rc renderRequestContext) (string, error) {
	var commitMsg string
	if rc.request.CommitMessage != "" {
		commitMsg = rc.request.CommitMessage
	} else {
		// Use the source commit's message as a starting point
		var err error
		if commitMsg, err = rc.repo.CommitMessage(rc.source.commit); err != nil {
			return "", errors.Wrapf(
				err,
				"error getting commit message for commit %q",
				rc.source.commit,
			)
		}
	}

	// Add the source commit's ID
	formattedCommitMsg := fmt.Sprintf(
		"%s\n\nBookkeeper created this commit by rendering configuration from %s",
		commitMsg,
		rc.source.commit,
	)

	// Find all recent commits
	var memberCommitMsgs []string
	if rc.target.oldBranchMetadata.SourceCommit != "" {
		// Add info about member commits
		formattedCommitMsg = fmt.Sprintf(
			"%s\n\nThis includes the following changes (newest to oldest):",
			formattedCommitMsg,
		)
		var err error
		if memberCommitMsgs, err = rc.repo.CommitMessages(
			rc.target.oldBranchMetadata.SourceCommit,
			rc.source.commit,
		); err != nil {
			return "", errors.Wrapf(
				err,
				"error getting commit messages between commit %q and %q",
				rc.target.oldBranchMetadata.SourceCommit,
				rc.source.commit,
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

	if len(rc.target.newBranchMetadata.ImageSubstitutions) != 0 {
		formattedCommitMsg = fmt.Sprintf(
			"%s\n\nBookkeeper also incorporated the following images into this "+
				"commit:\n",
			formattedCommitMsg,
		)
		for _, image := range rc.target.newBranchMetadata.ImageSubstitutions {
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
