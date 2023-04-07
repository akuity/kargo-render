package bookkeeper

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/akuity/bookkeeper/internal/git"
	"github.com/akuity/bookkeeper/internal/github"
)

func openPR(ctx context.Context, rc renderRequestContext) (string, error) {
	commitMsgParts := strings.SplitN(rc.target.commit.message, "\n", 2)
	var title string
	if rc.target.branchConfig.PRs.UseUniqueBranchNames {
		// PR title is just the first line of the commit message
		title = fmt.Sprintf("%s <-- %s", rc.request.TargetBranch, commitMsgParts[0])
	} else {
		// Something more generic because this PR can be updated with more commits
		title =
			fmt.Sprintf("%s <-- latest batched changes", rc.request.TargetBranch)
	}

	// TODO: Support git providers other than GitHub.
	//
	// Wish list:
	//
	// * GitHub Enterprise
	// * Bitbucket
	// * Azure DevOps
	// * GitLab
	// * Other?
	url, err := github.OpenPR(
		ctx,
		rc.request.RepoURL,
		title,
		"See individual commit messages for details.",
		rc.request.TargetBranch,
		rc.target.commit.branch,
		git.RepoCredentials{
			Username: rc.request.RepoCreds.Username,
			Password: rc.request.RepoCreds.Password,
		},
	)
	// TODO: Catch specific errors that have to do with an open PR already being
	// associated with the target branch
	return url,
		errors.Wrap(err, "error opening pull request to the target branch")
}
