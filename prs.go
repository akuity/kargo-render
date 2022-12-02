package bookkeeper

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/akuityio/bookkeeper/internal/git"
	"github.com/akuityio/bookkeeper/internal/github"
)

func openPR(ctx context.Context, rc renderRequestContext) (string, error) {
	commitMsgParts := strings.SplitN(rc.target.commit.message, "\n", 2)
	// PR title is just the first line of the commit message
	title := fmt.Sprintf("%s <-- %s", rc.request.TargetBranch, commitMsgParts[0])
	// PR body is just the first line of the commit message
	var body string
	if len(commitMsgParts) == 2 {
		body = strings.TrimSpace(commitMsgParts[1])
	}
	// TODO: Support git providers other than GitHub
	url, err := github.OpenPR(
		ctx,
		rc.request.RepoURL,
		title,
		body,
		rc.request.TargetBranch,
		rc.target.commit.branch,
		git.RepoCredentials{
			Username: rc.request.RepoCreds.Username,
			Password: rc.request.RepoCreds.Password,
		},
	)
	return url,
		errors.Wrap(err, "error opening pull request to the target branch")
}
