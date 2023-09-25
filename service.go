package bookkeeper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"

	"github.com/akuity/bookkeeper/internal/helm"
	"github.com/akuity/bookkeeper/internal/kustomize"
	"github.com/akuity/bookkeeper/internal/manifests"
	"github.com/akuity/bookkeeper/internal/ytt"
	"github.com/akuity/bookkeeper/pkg/git"
)

type ServiceOptions struct {
	LogLevel LogLevel
}

// Service is an interface for components that can handle bookkeeping requests.
// Implementations of this interface are transport-agnostic.
type Service interface {
	// RenderManifests handles a bookkeeping request.
	RenderManifests(context.Context, RenderRequest) (RenderResponse, error)
}

type service struct {
	logger *log.Logger

	// These behaviors are overridable for testing purposes
	helmRenderFn func(
		ctx context.Context,
		releaseName string,
		chartPath string,
		valuesPaths []string,
	) ([]byte, error)

	yttRenderFn func(ctx context.Context, paths []string) ([]byte, error)

	kustomizeRenderFn func(
		ctx context.Context,
		path string,
		images []string,
		enableHelm bool,
	) ([]byte, error)
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
		logger:            logger,
		helmRenderFn:      helm.Render,
		yttRenderFn:       ytt.Render,
		kustomizeRenderFn: kustomize.Render,
	}
}

// nolint: gocyclo
func (s *service) RenderManifests(
	ctx context.Context,
	req RenderRequest,
) (RenderResponse, error) {
	req.id = uuid.NewV4().String()

	logger := s.logger.WithField("request", req.id)
	startEndLogger := logger.WithFields(log.Fields{
		"repo":         req.RepoURL,
		"targetBranch": req.TargetBranch,
	})

	startEndLogger.Debug("handling rendering request")

	res := RenderResponse{}

	var err error
	if req, err = validateAndCanonicalizeRequest(req); err != nil {
		return res, err
	}
	startEndLogger.Debug("validated rendering request")

	rc := renderRequestContext{
		logger:  logger,
		request: req,
	}

	if rc.repo, err = git.Clone(
		rc.request.RepoURL,
		git.RepoCredentials{
			SSHPrivateKey: rc.request.RepoCreds.SSHPrivateKey,
			Username:      rc.request.RepoCreds.Username,
			Password:      rc.request.RepoCreds.Password,
		},
	); err != nil {
		return res, errors.Wrap(err, "error cloning remote repository")
	}
	defer rc.repo.Close()

	// TODO: Add some logging to this block
	if rc.request.Ref == "" {
		if rc.source.commit, err = rc.repo.LastCommitID(); err != nil {
			return res, errors.Wrap(err, "error getting last commit ID")
		}
	} else {
		if err = rc.repo.Checkout(rc.request.Ref); err != nil {
			return res, errors.Wrapf(err, "error checking out %q", rc.request.Ref)
		}
		if rc.intermediate.branchMetadata, err =
			loadBranchMetadata(rc.repo.WorkingDir()); err != nil {
			return res, errors.Wrap(err, "error loading branch metadata")
		}
		if rc.intermediate.branchMetadata == nil {
			// We're not on a target branch. We're sitting on the source commit.
			if rc.source.commit, err = rc.repo.LastCommitID(); err != nil {
				return res, errors.Wrap(err, "error getting last commit ID")
			}
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
	if rc.target.branchConfig, err =
		repoConfig.GetBranchConfig(rc.request.TargetBranch); err != nil {
		return res, errors.Wrapf(
			err,
			"error loading configuration for branch %q",
			rc.request.TargetBranch,
		)
	}

	if len(rc.target.branchConfig.AppConfigs) == 0 {
		rc.target.branchConfig.AppConfigs = map[string]appConfig{
			"app": {
				ConfigManagement: configManagementConfig{
					Kustomize: &kustomize.Config{
						Path: rc.request.TargetBranch,
					},
				},
			},
		}
	}

	if rc.target.prerenderedManifests, err =
		s.preRender(ctx, rc, rc.repo.WorkingDir()); err != nil {
		return res, errors.Wrap(err, "error pre-rendering manifests")
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

	if rc.target.commit.branch != rc.request.TargetBranch {
		// The commit branch isn't the target branch and we should take into account
		// any metadata that already exists in the commit branch, in case that
		// branch already existed.
		if rc.target.commit.oldBranchMetadata, err =
			loadBranchMetadata(rc.repo.WorkingDir()); err != nil {
			return res, errors.Wrap(err, "error loading branch metadata")
		}
	}

	rc.target.newBranchMetadata.SourceCommit = rc.source.commit
	if rc.target.newBranchMetadata.ImageSubstitutions,
		rc.target.renderedManifests,
		err =
		renderLastMile(ctx, rc); err != nil {
		return res, errors.Wrap(err, "error in last-mile manifest rendering")
	}

	// Write branch metadata
	if err = writeBranchMetadata(
		rc.target.newBranchMetadata,
		rc.repo.WorkingDir(),
	); err != nil {
		return res, errors.Wrap(err, "error writing branch metadata")
	}
	logger.WithField("sourceCommit", rc.source.commit).
		Debug("wrote branch metadata")

	// Write the new fully-rendered manifests to the root of the repo
	if err = writeAllManifests(rc); err != nil {
		return res, err
	}
	logger.Debug("wrote all manifests")

	// Before committing, check if we actually have any diffs from the head of
	// this branch that are NOT just Bookkeeper metadata. We'd have an error if we
	// tried to commit with no diffs!
	diffPaths, err := rc.repo.GetDiffPaths()
	if err != nil {
		return res, errors.Wrap(err, "error checking for diffs")
	}
	if len(diffPaths) == 0 ||
		(len(diffPaths) == 1 && diffPaths[0] == ".bookkeeper/metadata.yaml") {
		logger.WithField("commitBranch", rc.target.commit.branch).Debug(
			"manifests do not differ from the head of the " +
				"commit branch; no further action is required",
		)
		res.ActionTaken = ActionTakenNone
		res.CommitID, err = rc.repo.LastCommitID()
		return res, errors.Wrap(
			err,
			"error getting last commit ID from the commit branch",
		)
	}

	if rc.target.commit.message, err = buildCommitMessage(rc); err != nil {
		return res, err
	}
	logger.Debug("prepared commit message")

	// Commit the changes
	if err = rc.repo.AddAllAndCommit(rc.target.commit.message); err != nil {
		return res, errors.Wrapf(err, "error committing manifests")
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
	}).Debug("committed all changes")

	// Push the commit branch to the remote
	if err = rc.repo.Push(); err != nil {
		return res, errors.Wrap(
			err,
			"error pushing commit branch to remote",
		)
	}
	logger.WithField("commitBranch", rc.target.commit.branch).
		Debug("pushed commit branch to remote")

	// Open a PR if requested
	if rc.target.branchConfig.PRs.Enabled {
		if res.PullRequestURL, err = openPR(ctx, rc); err != nil {
			return res,
				errors.Wrap(err, "error opening pull request to the target branch")
		}
		if res.PullRequestURL == "" {
			res.ActionTaken = ActionTakenUpdatedPR
			logger.Debug("updated existing PR")
		} else {
			res.ActionTaken = ActionTakenOpenedPR
			logger.WithField("prURL", res.PullRequestURL).Debug("opened PR")
		}
	} else {
		res.ActionTaken = ActionTakenPushedDirectly
		res.CommitID = rc.target.commit.id
	}

	startEndLogger.Debug("completed rendering request")

	return res, nil
}

// buildCommitMessage builds a commit message for rendered manifests being
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
		"%s\n\nBookkeeper created this commit by rendering manifests from %s",
		commitMsg,
		rc.source.commit,
	)

	// TODO: Tentatively removing the following because it simply results in too
	// much noise in the repo history. Leaving it commented for now in case we
	// decide to bring it back later.
	//
	// // Find all recent commits
	// if rc.target.oldBranchMetadata.SourceCommit != "" {
	// 	var memberCommitMsgs []string
	// 	// Add info about member commits
	// 	formattedCommitMsg = fmt.Sprintf(
	// 		"%s\n\nThis includes the following changes (newest to oldest):",
	// 		formattedCommitMsg,
	// 	)
	// 	var err error
	// 	if memberCommitMsgs, err = rc.repo.CommitMessages(
	// 		rc.target.oldBranchMetadata.SourceCommit,
	// 		rc.source.commit,
	// 	); err != nil {
	// 		return "", errors.Wrapf(
	// 			err,
	// 			"error getting commit messages between commit %q and %q",
	// 			rc.target.oldBranchMetadata.SourceCommit,
	// 			rc.source.commit,
	// 		)
	// 	}
	// 	for _, memberCommitMsg := range memberCommitMsgs {
	// 		formattedCommitMsg = fmt.Sprintf(
	// 			"%s\n  * %s",
	// 			formattedCommitMsg,
	// 			memberCommitMsg,
	// 		)
	// 	}
	// }

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

func writeAllManifests(rc renderRequestContext) error {
	for appName, appConfig := range rc.target.branchConfig.AppConfigs {
		appLogger := rc.logger.WithField("app", appName)
		var outputDir string
		if appConfig.OutputPath != "" {
			outputDir = filepath.Join(rc.repo.WorkingDir(), appConfig.OutputPath)
		} else {
			outputDir = filepath.Join(rc.repo.WorkingDir(), appName)
		}
		var err error
		if appConfig.CombineManifests {
			appLogger.Debug("manifests will be combined into a single file")
			err =
				writeCombinedManifests(outputDir, rc.target.renderedManifests[appName])
		} else {
			appLogger.Debug("manifests will NOT be combined into a single file")
			err = writeManifests(outputDir, rc.target.renderedManifests[appName])
		}
		appLogger.Debug("wrote manifests")
		if err != nil {
			return errors.Wrapf(
				err, "error writing manifests for app %q to %q",
				appName,
				outputDir,
			)
		}
	}
	return nil
}

func writeManifests(dir string, yamlBytes []byte) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Wrapf(err, "error creating directory %q", dir)
	}
	manifestsByResourceTypeAndName, err := manifests.SplitYAML(yamlBytes)
	if err != nil {
		return err
	}
	for resourceTypeAndName, manifest := range manifestsByResourceTypeAndName {
		fileName := filepath.Join(
			dir,
			fmt.Sprintf("%s.yaml", resourceTypeAndName),
		)
		// nolint: gosec
		if err := os.WriteFile(fileName, manifest, 0644); err != nil {
			return errors.Wrapf(
				err,
				"error writing manifest to %q",
				fileName,
			)
		}
	}
	return nil
}

func writeCombinedManifests(dir string, manifests []byte) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Wrapf(err, "error creating directory %q", dir)
	}
	fileName := filepath.Join(dir, "all.yaml")
	return errors.Wrapf(
		// nolint: gosec
		os.WriteFile(fileName, manifests, 0644),
		"error writing manifests to %q",
		fileName,
	)
}
