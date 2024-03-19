package render

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"

	"github.com/akuity/kargo-render/internal/argocd"
	"github.com/akuity/kargo-render/internal/manifests"
	"github.com/akuity/kargo-render/pkg/git"
)

type ServiceOptions struct {
	LogLevel LogLevel
}

// Service is an interface for components that can handle rendering requests.
// Implementations of this interface are transport-agnostic.
type Service interface {
	// RenderManifests handles a rendering request.
	RenderManifests(context.Context, Request) (Response, error)
}

type service struct {
	logger   *log.Logger
	renderFn func(
		ctx context.Context,
		repoRoot string,
		cfg argocd.ConfigManagementConfig,
	) ([]byte, error)
}

// NewService returns an implementation of the Service interface for
// handling rendering requests.
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
		logger:   logger,
		renderFn: argocd.Render,
	}
}

// nolint: gocyclo
func (s *service) RenderManifests(
	ctx context.Context,
	req Request,
) (Response, error) {
	req.id = uuid.NewV4().String()

	logger := s.logger.WithField("request", req.id)
	startEndLogger := logger.WithFields(log.Fields{
		"repo":         req.RepoURL,
		"targetBranch": req.TargetBranch,
	})

	startEndLogger.Debug("handling rendering request")

	res := Response{}

	var err error
	if req, err = validateAndCanonicalizeRequest(req); err != nil {
		return res, err
	}
	startEndLogger.Debug("validated rendering request")

	rc := requestContext{
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
		return res, fmt.Errorf("error cloning remote repository: %w", err)
	}
	defer rc.repo.Close()

	// TODO: Add some logging to this block
	if rc.request.Ref == "" {
		if rc.source.commit, err = rc.repo.LastCommitID(); err != nil {
			return res, fmt.Errorf("error getting last commit ID: %w", err)
		}
	} else {
		if err = rc.repo.Checkout(rc.request.Ref); err != nil {
			return res, fmt.Errorf("error checking out %q: %w", rc.request.Ref, err)
		}
		if rc.intermediate.branchMetadata, err =
			loadBranchMetadata(rc.repo.WorkingDir()); err != nil {
			return res, fmt.Errorf("error loading branch metadata: %w", err)
		}
		if rc.intermediate.branchMetadata == nil {
			// We're not on a target branch. We're sitting on the source commit.
			if rc.source.commit, err = rc.repo.LastCommitID(); err != nil {
				return res, fmt.Errorf("error getting last commit ID: %w", err)
			}
		} else {
			// Follow the branch metadata back to the real source commit
			if err = rc.repo.Checkout(
				rc.intermediate.branchMetadata.SourceCommit,
			); err != nil {
				return res, fmt.Errorf(
					"error checking out %q: %w",
					rc.intermediate.branchMetadata.SourceCommit,
					err,
				)
			}
			rc.source.commit = rc.intermediate.branchMetadata.SourceCommit
		}
	}

	repoConfig, err := loadRepoConfig(rc.repo.WorkingDir())
	if err != nil {
		return res,
			fmt.Errorf("error loading Kargo Render configuration from repo: %w", err)
	}
	if rc.target.branchConfig, err =
		repoConfig.GetBranchConfig(rc.request.TargetBranch); err != nil {
		return res, fmt.Errorf(
			"error loading configuration for branch %q: %w",
			rc.request.TargetBranch,
			err,
		)
	}

	if len(rc.target.branchConfig.AppConfigs) == 0 {
		rc.target.branchConfig.AppConfigs = map[string]appConfig{
			"app": {
				ConfigManagement: argocd.ConfigManagementConfig{
					Path: rc.request.TargetBranch,
				},
			},
		}
	}

	if rc.target.prerenderedManifests, err =
		s.preRender(ctx, rc, rc.repo.WorkingDir()); err != nil {
		return res, fmt.Errorf("error pre-rendering manifests: %w", err)
	}

	if err = switchToTargetBranch(rc); err != nil {
		return res, fmt.Errorf("error switching to target branch: %w", err)
	}

	oldTargetBranchMetadata, err := loadBranchMetadata(rc.repo.WorkingDir())
	if err != nil {
		return res, fmt.Errorf("error loading branch metadata: %w", err)
	}
	if oldTargetBranchMetadata == nil {
		return res, fmt.Errorf(
			"target branch %q already exists, but does not appear to be managed by "+
				"Kargo Render; refusing to overwrite branch contents",
			rc.request.TargetBranch,
		)
	}
	rc.target.oldBranchMetadata = *oldTargetBranchMetadata

	if rc.target.commit.branch, err = switchToCommitBranch(rc); err != nil {
		return res, fmt.Errorf("error switching to commit branch: %w", err)
	}

	if rc.target.commit.branch != rc.request.TargetBranch {
		// The commit branch isn't the target branch and we should take into account
		// any metadata that already exists in the commit branch, in case that
		// branch already existed.
		if rc.target.commit.oldBranchMetadata, err =
			loadBranchMetadata(rc.repo.WorkingDir()); err != nil {
			return res, fmt.Errorf("error loading branch metadata: %w", err)
		}
	}

	rc.target.newBranchMetadata.SourceCommit = rc.source.commit
	if rc.target.newBranchMetadata.ImageSubstitutions,
		rc.target.renderedManifests,
		err =
		renderLastMile(ctx, rc); err != nil {
		return res, fmt.Errorf("error in last-mile manifest rendering: %w", err)
	}

	// Write branch metadata
	if err = writeBranchMetadata(
		rc.target.newBranchMetadata,
		rc.repo.WorkingDir(),
	); err != nil {
		return res, fmt.Errorf("error writing branch metadata: %w", err)
	}
	logger.WithField("sourceCommit", rc.source.commit).
		Debug("wrote branch metadata")

	// Write the new fully-rendered manifests to the root of the repo
	if err = writeAllManifests(rc); err != nil {
		return res, err
	}
	logger.Debug("wrote all manifests")

	// Before committing, check if we actually have any diffs from the head of
	// this branch that are NOT just Kargo Render metadata. We'd have an error if
	// we tried to commit with no diffs!
	diffPaths, err := rc.repo.GetDiffPaths()
	if err != nil {
		return res, fmt.Errorf("error checking for diffs: %w", err)
	}
	if len(diffPaths) == 0 ||
		(len(diffPaths) == 1 && diffPaths[0] == ".kargo-render/metadata.yaml") {
		logger.WithField("commitBranch", rc.target.commit.branch).Debug(
			"manifests do not differ from the head of the " +
				"commit branch; no further action is required",
		)
		res.ActionTaken = ActionTakenNone
		if res.CommitID, err = rc.repo.LastCommitID(); err != nil {
			return res, fmt.Errorf(
				"error getting last commit ID from the commit branch: %w",
				err,
			)
		}
		return res, nil
	}

	if rc.target.commit.message, err = buildCommitMessage(rc); err != nil {
		return res, err
	}
	logger.Debug("prepared commit message")

	// Commit the changes
	if err = rc.repo.AddAllAndCommit(rc.target.commit.message); err != nil {
		return res, fmt.Errorf("error committing manifests: %w", err)
	}
	if rc.target.commit.id, err = rc.repo.LastCommitID(); err != nil {
		return res, fmt.Errorf(
			"error getting last commit ID from the commit branch: %w",
			err,
		)
	}
	logger.WithFields(log.Fields{
		"commitBranch": rc.target.commit.branch,
		"commitID":     rc.target.commit.id,
	}).Debug("committed all changes")

	// Push the commit branch to the remote
	if err = rc.repo.Push(); err != nil {
		return res, fmt.Errorf(
			"error pushing commit branch to remote: %w",
			err,
		)
	}
	logger.WithField("commitBranch", rc.target.commit.branch).
		Debug("pushed commit branch to remote")

	// Open a PR if requested
	if rc.target.branchConfig.PRs.Enabled {
		if res.PullRequestURL, err = openPR(ctx, rc); err != nil {
			return res,
				fmt.Errorf("error opening pull request to the target branch: %w", err)
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
// written to a target branch by using the source commit's own commit message as
// a starting point. The message is then augmented with details about where
// Kargo Render rendered it from (the source commit) and any image substitutions
// Kargo Render made per the RenderRequest.
func buildCommitMessage(rc requestContext) (string, error) {
	var commitMsg string
	if rc.request.CommitMessage != "" {
		commitMsg = rc.request.CommitMessage
	} else {
		// Use the source commit's message as a starting point
		var err error
		if commitMsg, err = rc.repo.CommitMessage(rc.source.commit); err != nil {
			return "", fmt.Errorf(
				"error getting commit message for commit %q: %w",
				rc.source.commit,
				err,
			)
		}
	}

	// Add the source commit's ID
	formattedCommitMsg := fmt.Sprintf(
		"%s\n\nKargo Render created this commit by rendering manifests from %s",
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
			"%s\n\nKargo Render also incorporated the following images into this "+
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

func writeAllManifests(rc requestContext) error {
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
			return fmt.Errorf(
				"error writing manifests for app %q to %q: %w",
				appName,
				outputDir,
				err,
			)
		}
	}
	return nil
}

func writeManifests(dir string, yamlBytes []byte) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating directory %q: %w", dir, err)
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
			return fmt.Errorf(
				"error writing manifest to %q: %w",
				fileName,
				err,
			)
		}
	}
	return nil
}

func writeCombinedManifests(dir string, manifestBytes []byte) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating directory %q: %w", dir, err)
	}
	fileName := filepath.Join(dir, "all.yaml")
	if err := os.WriteFile(fileName, manifestBytes, 0644); err != nil { // nolint: gosec
		return fmt.Errorf(
			"error writing manifests to %q: %w",
			fileName,
			err,
		)
	}
	return nil
}
