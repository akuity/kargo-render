package render

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/akuity/kargo-render/internal/github"
	"github.com/akuity/kargo-render/pkg/git"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func openPR(ctx context.Context, rc requestContext) (string, error) {
	commitMsgParts := strings.SplitN(rc.target.commit.message, "\n", 2)
	var title string
	if rc.target.branchConfig.PRs.UseUniqueBranchNames {
		// PR title is just the first line of the commit message
		title = fmt.Sprintf("%s <-- %s", rc.request.TargetBranch, commitMsgParts[0])
	} else {
		// Something more generic because this PR can be updated with more commits
		title = fmt.Sprintf("%s <-- latest batched changes", rc.request.TargetBranch)
	}

	// TODO: Support git providers other than GitHub and Gitlab.
	//
	// Wish list:
	//
	// * GitHub Enterprise
	// * Bitbucket
	// * Azure DevOps
	// * Other?
	if isGitlabURL(rc.request.RepoURL) {
		return openGitlabMR(ctx, rc, title)
	}
	return openGithubPR(ctx, rc, title)
}

func openGithubPR(ctx context.Context, rc requestContext, title string) (string, error) {
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
	if err != nil {
		return "",
			fmt.Errorf("error opening pull request to the target branch: %w", err)
	}
	return url, nil
}

func openGitlabMR(ctx context.Context, rc requestContext, title string) (string, error) {
	gitlabToken := rc.request.RepoCreds.Password
	if gitlabToken == "" {
		return "",
			fmt.Errorf("GITLAB_TOKEN not set")
	}

	git, err := gitlab.NewClient(gitlabToken)

	opts := &gitlab.CreateMergeRequestOptions{
		Title:        &title,
		SourceBranch: &rc.target.commit.branch,
		TargetBranch: &rc.request.TargetBranch,
	}

	u, err := url.Parse(rc.request.RepoURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse repo URL: %w", err)
	}

	projectPath := u.Path

	// Remove leading slash if present
	projectPath = strings.TrimPrefix(projectPath, "/")

	// Remove trailing .git if present
	projectPath = strings.TrimSuffix(projectPath, ".git")

	mr, _, err := git.MergeRequests.CreateMergeRequest(projectPath, opts)
	if err != nil {
		return "",
			fmt.Errorf("failed to create merge request: %w", err)
	}

	fmt.Printf("Merge request created: %s\n", mr.WebURL)
	return mr.WebURL, nil
}

func isGitlabURL(repoURL string) bool {
	u, err := url.Parse(repoURL)
	if err != nil {
		return false // Handle parsing errors gracefully
	}
	hostname := u.Hostname()
	return strings.Contains(hostname, "gitlab.com") || strings.Contains(hostname, "gitlab.") // Check for gitlab.com or other gitlab domains
}

