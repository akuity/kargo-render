package render

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

func validateAndCanonicalizeRequest(req Request) (Request, error) {
	req.RepoURL = strings.TrimSpace(req.RepoURL)
	if req.RepoURL == "" {
		return req, errors.New("validation failed: RepoURL is a required field")
	}
	repoURLRegex :=
		regexp.MustCompile(`^(?:(?:(?:https?://)|(?:git@))[\w:/\-\.\?=@&%]+)$`)
	if !repoURLRegex.MatchString(req.RepoURL) {
		return req, fmt.Errorf(
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

	req.Ref = strings.TrimSpace(req.Ref)

	req.TargetBranch = strings.TrimSpace(req.TargetBranch)
	if req.TargetBranch == "" {
		return req,
			errors.New("validation failed: TargetBranch is a required field")
	}
	targetBranchRegex := regexp.MustCompile(`^(?:[\w\.-]+\/?)*\w$`)
	if !targetBranchRegex.MatchString(req.TargetBranch) {
		return req, fmt.Errorf(
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
