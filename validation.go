package render

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	repoURLRegex      = regexp.MustCompile(`^(?:(?:(?:https?://)|(?:git@))[\w:/\-\.\?=@&%]+)$`)
	targetBranchRegex = regexp.MustCompile(`^(?:[\w\.-]+\/?)*\w$`)
)

func (r *Request) canonicalizeAndValidate() error {
	var errs []error

	// First, canonicalize the input...

	r.RepoURL = strings.TrimSpace(r.RepoURL)
	r.RepoCreds.Username = strings.TrimSpace(r.RepoCreds.Username)
	r.RepoCreds.Password = strings.TrimSpace(r.RepoCreds.Password)
	r.Ref = strings.TrimSpace(r.Ref)
	r.TargetBranch = strings.TrimSpace(r.TargetBranch)
	r.TargetBranch = strings.TrimPrefix(r.TargetBranch, "refs/heads/")
	for i := range r.Images {
		r.Images[i] = strings.TrimSpace(r.Images[i])
	}
	r.CommitMessage = strings.TrimSpace(r.CommitMessage)
	r.LocalInPath = strings.TrimSpace(r.LocalInPath)
	if r.LocalInPath != "" {
		r.LocalInPath = strings.TrimSuffix(r.LocalInPath, "/")
		var err error
		if r.LocalInPath, err = filepath.Abs(r.LocalInPath); err != nil {
			errs = append(
				errs,
				fmt.Errorf("error canonicalizing path %s: %w", r.LocalInPath, err),
			)
		}
	}

	r.LocalOutPath = strings.TrimSpace(r.LocalOutPath)
	if r.LocalOutPath != "" {
		r.LocalOutPath = strings.TrimSuffix(r.LocalOutPath, "/")
		var err error
		if r.LocalOutPath, err = filepath.Abs(r.LocalOutPath); err != nil {
			errs = append(
				errs,
				fmt.Errorf("error canonicalizing path %s: %w", r.LocalOutPath, err),
			)
		}
	}

	// Check for invalid combinations of input...

	// Input comes from the remote repository or from a local path, but not both.
	if r.RepoURL == "" && r.LocalInPath == "" {
		errs = append(
			errs,
			errors.New(
				"no input source specified: at least one of RepoURL or LocalInPath is required ",
			),
		)
	}
	if r.RepoURL != "" && r.LocalInPath != "" {
		errs = append(
			errs,
			errors.New(
				"input source is ambiguous: RepoURL and LocalInPath are mutually exclusive",
			),
		)
	}
	if r.LocalInPath != "" && r.Ref != "" {
		errs = append(errs, errors.New("LocalInPath and Ref are mutually exclusive"))
	}

	var count int
	if r.CommitMessage != "" {
		count++
	}
	if r.LocalOutPath != "" {
		count++
	}
	if r.Stdout {
		count++
	}
	if count > 1 {
		errs = append(
			errs,
			errors.New(
				"output destination is ambiguous: CommitMessage, LocalOutPath, and "+
					"Stdout are mutually exclusive",
			),
		)
	}

	// Now validate individual fields...

	if r.RepoURL != "" && !repoURLRegex.MatchString(r.RepoURL) {
		errs = append(
			errs,
			fmt.Errorf(
				"RepoURL %q does not appear to be a valid git repository URL",
				r.RepoURL,
			),
		)
	}

	if r.TargetBranch == "" {
		errs = append(errs, errors.New("TargetBranch is a required field"))
	}
	if !targetBranchRegex.MatchString(r.TargetBranch) {
		errs = append(
			errs,
			fmt.Errorf("TargetBranch %q is an invalid branch name", r.TargetBranch),
		)
	}

	if len(r.Images) > 0 {
		for i := range r.Images {
			r.Images[i] = strings.TrimSpace(r.Images[i])
			if r.Images[i] == "" {
				errs = append(errs, errors.New("Images must not contain any empty strings"))
				break
			}
		}
	}

	if r.LocalInPath != "" {
		if fi, err := os.Stat(r.LocalInPath); err != nil {
			if os.IsNotExist(err) {
				errs = append(
					errs,
					fmt.Errorf("path %s does not exist", r.LocalInPath),
				)
			} else {
				errs = append(
					errs,
					fmt.Errorf("error checking if path %s exists: %w", r.LocalInPath, err),
				)
			}
		} else if !fi.IsDir() {
			errs = append(errs, fmt.Errorf("path %s is not a directory", r.LocalInPath))
		}
	}

	if r.LocalOutPath != "" {
		if _, err := os.Stat(r.LocalOutPath); err != nil && !os.IsNotExist(err) {
			errs = append(
				errs,
				fmt.Errorf("error checking if path %s exists: %w", r.LocalOutPath, err),
			)
		} else if err == nil {
			// path exists
			errs = append(
				errs,
				fmt.Errorf("path %q already exists; refusing to overwrite", r.LocalOutPath),
			)
		}
	}

	return errors.Join(errs...)
}
